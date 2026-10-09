// Package publicationv2 implements the pure, byte-preserving source decoder
// for the additive registry publication protocol.
package publicationv2

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"io"
	"io/fs"
	"math"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sirerun/gist/hosted/internal/contract"
	"github.com/sirerun/gist/hosted/internal/ports"
)

var ErrValidation = errors.New("publication v2 validation failed")
var ErrBudgetExceeded = errors.New("publication v2 budget exceeded")

//go:embed schemas/**/*.json
var schemaFiles embed.FS

type Limits struct {
	MaxEnvelopeBytes, MaxPackageBytes, MaxExpandedBytes, MaxFileBytes int64
	MaxFiles                                                          int
}

func DefaultLimits() Limits { return Limits{16 << 20, 10 << 20, 10 << 20, 2 << 20, 256} }

func fail(f string, a ...any) error { return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(f, a...)) }
func exactKeys(raw []byte, allowed ...string) bool {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	want := map[string]bool{}
	for _, k := range allowed {
		want[k] = true
	}
	if len(m) != len(want) {
		return false
	}
	for k := range m {
		if !want[k] {
			return false
		}
	}
	return true
}
func sum(b []byte) string          { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func digest(v string) ports.Digest { return ports.Digest{Algorithm: "sha256", Value: v} }

// checkJSON implements the source grammar independently of encoding/json's
// permissive duplicate-key and invalid UTF-8 handling.
func checkJSON(b []byte) error {
	if !utf8.Valid(b) {
		return fail("invalid UTF-8")
	}
	if err := checkSurrogates(b); err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fail("expected a JSON object")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := scanValue(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fail("trailing JSON value")
	}
	return nil
}

func checkSurrogates(b []byte) error {
	for i := 0; i < len(b); i++ {
		if b[i] != '"' {
			continue
		}
		i++
		for i < len(b) && b[i] != '"' {
			if b[i] != '\\' {
				i++
				continue
			}
			i++
			if i >= len(b) {
				return fail("truncated escape")
			}
			if b[i] != 'u' {
				i++
				continue
			}
			if i+4 >= len(b) {
				return fail("truncated unicode escape")
			}
			v, e := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
			if e != nil {
				return fail("invalid unicode escape")
			}
			i += 4
			if v >= 0xDC00 && v <= 0xDFFF {
				return fail("unpaired low surrogate")
			}
			if v >= 0xD800 && v <= 0xDBFF {
				if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
					return fail("unpaired high surrogate")
				}
				w, e := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
				if e != nil || w < 0xDC00 || w > 0xDFFF {
					return fail("unpaired high surrogate")
				}
				i += 6
			}
		}
	}
	return nil
}
func scanValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return fail("invalid JSON: %v", err)
	}
	if x, ok := t.(json.Delim); ok {
		switch x {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return fail("invalid object key")
				}
				key, ok := k.(string)
				if !ok {
					return fail("invalid object key")
				}
				if seen[key] {
					return fail("duplicate key %q", key)
				}
				seen[key] = true
				if e = scanValue(d); e != nil {
					return e
				}
			}
			if _, e := d.Token(); e != nil {
				return fail("unterminated object")
			}
		case '[':
			for d.More() {
				if e := scanValue(d); e != nil {
					return e
				}
			}
			if _, e := d.Token(); e != nil {
				return fail("unterminated array")
			}
		default:
			return fail("unexpected delimiter")
		}
	}
	if n, ok := t.(json.Number); ok {
		if strings.ContainsAny(string(n), ".eE") {
			var f float64
			if _, e := fmt.Sscan(string(n), &f); e != nil || math.IsInf(f, 0) || math.IsNaN(f) {
				return fail("non-finite number")
			}
		}
	}
	return nil
}

type envelope struct {
	Artifact       json.RawMessage `json:"artifact"`
	MaxBytes       json.Number     `json:"max_bytes"`
	IdempotencyKey string          `json:"idempotency_key"`
}
type typedArtifact struct {
	ID       string          `json:"id"`
	Version  string          `json:"version"`
	Document json.RawMessage `json:"document"`
}
type skillArtifact struct {
	Raw     json.RawMessage `json:"-"`
	Package struct {
		Encoding  string `json:"encoding"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
	} `json:"package"`
}

func Decode(kind ports.ArtifactKind, raw []byte, limits Limits) (ports.PreparedPublication, error) {
	var out ports.PreparedPublication
	if limits.MaxEnvelopeBytes <= 0 || limits.MaxPackageBytes <= 0 || limits.MaxExpandedBytes <= 0 || limits.MaxFileBytes <= 0 || limits.MaxFiles <= 0 {
		return out, fail("invalid limits")
	}
	if int64(len(raw)) > limits.MaxEnvelopeBytes {
		return out, fmt.Errorf("%w: envelope", ErrBudgetExceeded)
	}
	if err := checkJSON(raw); err != nil {
		return out, err
	}
	name, ok := map[ports.ArtifactKind]string{ports.KindSkill: "skill", ports.KindCapability: "capability", ports.KindTool: "tool", ports.KindProvider: "provider", ports.KindBinding: "binding", ports.KindTaxonomy: "taxonomy"}[kind]
	if !ok {
		return out, fail("unsupported artifact kind")
	}
	if err := validateSchema("https://gist.local/contracts/registry/v2/"+name+".publish.schema.json", raw); err != nil {
		return out, err
	}
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return out, fail("invalid envelope: %v", err)
	}
	if !exactKeys(raw, "artifact", "max_bytes", "idempotency_key") || len(e.IdempotencyKey) > 128 {
		return out, fail("invalid envelope keys or idempotency key")
	}
	if e.MaxBytes == "" || e.IdempotencyKey == "" {
		return out, fail("missing budget or idempotency key")
	}
	budget, err := e.MaxBytes.Int64()
	if err != nil || budget <= 0 {
		return out, fail("max_bytes must be a positive signed 64-bit integer")
	}
	if kind == ports.KindSkill {
		if !exactKeys(e.Artifact, "package") {
			return out, fail("invalid skill artifact keys")
		}
		var a skillArtifact
		if err = json.Unmarshal(e.Artifact, &a); err != nil || a.Package.Encoding != "base64" || a.Package.MediaType != "application/zip" {
			return out, fail("invalid skill package wrapper")
		}
		var pkg json.RawMessage
		var wrapper map[string]json.RawMessage
		_ = json.Unmarshal(e.Artifact, &wrapper)
		pkg = wrapper["package"]
		if !exactKeys(pkg, "encoding", "media_type", "data") {
			return out, fail("invalid package keys")
		}
		decodedUpper := int64(len(a.Package.Data)/4) * 3
		if strings.HasSuffix(a.Package.Data, "==") {
			decodedUpper -= 2
		} else if strings.HasSuffix(a.Package.Data, "=") {
			decodedUpper--
		}
		if decodedUpper > limits.MaxPackageBytes {
			return out, fmt.Errorf("%w: encoded archive", ErrBudgetExceeded)
		}
		blob, er := base64.StdEncoding.Strict().DecodeString(a.Package.Data)
		if er != nil || base64.StdEncoding.EncodeToString(blob) != a.Package.Data {
			return out, fail("non-canonical base64")
		}
		if int64(len(blob)) > limits.MaxPackageBytes {
			return out, fmt.Errorf("%w: decoded archive", ErrBudgetExceeded)
		}
		p, er := decodeSkill(blob, limits)
		if er != nil {
			return out, er
		}
		out = ports.PreparedPublication{Ref: ports.ArtifactRef{Kind: kind, ID: p.id, Version: p.version}, Artifact: blob, Metadata: p.manifest, ArtifactDigest: digest(sum(blob)), DocumentDigest: digest(sum(p.manifest)), ManifestDigest: digest(sum(p.manifest)), PackageDigest: digest(p.packageDigest)}
	} else {
		if !exactKeys(e.Artifact, "id", "version", "document") {
			return out, fail("invalid typed artifact keys")
		}
		var a typedArtifact
		if err = json.Unmarshal(e.Artifact, &a); err != nil || len(a.Document) == 0 {
			return out, fail("invalid typed artifact")
		}
		id, version, er := validateDocument(kind, a.ID, a.Version, a.Document)
		if er != nil {
			return out, er
		}
		if er = typedSemantics(kind, a.Document); er != nil {
			return out, er
		}
		out = ports.PreparedPublication{Ref: ports.ArtifactRef{Kind: kind, ID: id, Version: version}, Artifact: append([]byte(nil), a.Document...), Metadata: append([]byte(nil), a.Document...), ArtifactDigest: digest(sum(a.Document)), DocumentDigest: digest(sum(a.Document))}
	}
	out.IdempotencyKey = e.IdempotencyKey
	out.MaxBytes = budget
	return out, nil
}

type skillManifest struct {
	ID, Version, Entrypoint string
	Inventory               []contract.InventoryEntry         `json:"inventory"`
	PackageDigest           struct{ Algorithm, Value string } `json:"package_digest"`
	Publisher               *struct {
		ID string `json:"id"`
	} `json:"publisher"`
}
type skillResult struct {
	id, version   string
	manifest      []byte
	packageDigest string
}

func decodeSkill(blob []byte, l Limits) (skillResult, error) {
	zr, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		return skillResult{}, fail("invalid ZIP: %v", err)
	}
	if len(zr.File) > l.MaxFiles {
		return skillResult{}, fmt.Errorf("%w: archive members", ErrBudgetExceeded)
	}
	files := map[string][]byte{}
	var total int64
	for _, f := range zr.File {
		n := f.Name
		clean := path.Clean(n)
		mode := f.Mode()
		if n == "" || !safeInventoryPath(n) || strings.Contains(n, "\\") || strings.HasPrefix(n, "/") || clean != n || clean == "." || strings.HasPrefix(clean, "../") || mode&0o170000 != 0 && !mode.IsRegular() {
			return skillResult{}, fail("unsafe or non-regular member %q", n)
		}
		if !mode.IsRegular() {
			return skillResult{}, fail("non-regular member %q", n)
		}
		if _, ok := files[n]; ok {
			return skillResult{}, fail("duplicate member %q", n)
		}
		if f.UncompressedSize64 > uint64(l.MaxFileBytes) || f.UncompressedSize64 > math.MaxInt64 {
			return skillResult{}, fmt.Errorf("%w: member size", ErrBudgetExceeded)
		}
		if int64(f.UncompressedSize64) > l.MaxExpandedBytes-total {
			return skillResult{}, fmt.Errorf("%w: expanded archive", ErrBudgetExceeded)
		}
		total += int64(f.UncompressedSize64)
		r, e := f.Open()
		if e != nil {
			return skillResult{}, fail("open member")
		}
		b, e := io.ReadAll(io.LimitReader(r, l.MaxFileBytes+boolInt64(l.MaxFileBytes < math.MaxInt64)))
		closeErr := r.Close()
		if e != nil || closeErr != nil || int64(len(b)) > l.MaxFileBytes {
			return skillResult{}, fail("read member")
		}
		files[n] = b
	}
	mb, ok := files["manifest.json"]
	if !ok {
		return skillResult{}, fail("missing manifest")
	}
	if err := checkJSON(mb); err != nil {
		return skillResult{}, err
	}
	var m skillManifest
	if err = json.Unmarshal(mb, &m); err != nil {
		return skillResult{}, fail("invalid manifest")
	}
	if err = validateSchema("https://gist.local/contracts/registry/v1/manifest.schema.json", mb); err != nil {
		return skillResult{}, err
	}
	var semantics struct {
		Status      string `json:"publication_status"`
		Publication struct {
			Status     string `json:"status"`
			Provenance string `json:"provenance"`
		} `json:"publication"`
		Trust string `json:"trust"`
		Caps  []struct {
			ID      string `json:"id"`
			Version string `json:"contract_version"`
		} `json:"required_capabilities"`
	}
	if err = json.Unmarshal(mb, &semantics); err != nil || (semantics.Publication.Status != "approved" && semantics.Publication.Status != "published") || (semantics.Publication.Provenance != "publisher_asserted" && semantics.Publication.Provenance != "derived_unverified") || semantics.Trust != "operator_asserted" {
		return skillResult{}, fail("skill publication/trust provenance is not admissible")
	}
	for _, c := range semantics.Caps {
		if !validID(c.ID) || !semverRE.MatchString(c.Version) {
			return skillResult{}, fail("invalid required capability reference")
		}
	}
	if m.Entrypoint != "SKILL.md" {
		return skillResult{}, fail("entrypoint must be SKILL.md")
	}
	if _, ok = files["SKILL.md"]; !ok {
		return skillResult{}, fail("missing entrypoint")
	}
	if _, err = contract.ValidateAgentSkillsCore(files["SKILL.md"]); err != nil {
		return skillResult{}, fail("invalid entrypoint: %v", err)
	}
	declared := map[string]bool{}
	for _, x := range m.Inventory {
		if x.Path == "manifest.json" || !safeInventoryPath(x.Path) || declared[x.Path] {
			return skillResult{}, fail("invalid inventory")
		}
		declared[x.Path] = true
		b, ok := files[x.Path]
		if !ok || int64(len(b)) != x.Size || sum(b) != x.SHA256 {
			return skillResult{}, fail("inventory mismatch %q", x.Path)
		}
	}
	if len(declared) != len(files)-1 {
		return skillResult{}, fail("archive is not closed by inventory")
	}
	closure, _, e := contract.PackageDigest(m.Inventory)
	if e != nil || m.PackageDigest.Algorithm != "sha256" || closure != m.PackageDigest.Value {
		return skillResult{}, fail("package closure mismatch")
	}
	if m.ID == "" || m.Version == "" || !validID(m.ID) || !validVersion(m.Version) {
		return skillResult{}, fail("invalid skill identity")
	}
	return skillResult{m.ID, m.Version, mb, closure}, nil
}

var idRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,255}$`)

func boolInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
func safeInventoryPath(n string) bool {
	if n == "" || !utf8.ValidString(n) || strings.ContainsAny(n, "\\\x00\r\n\t") || strings.HasPrefix(n, "/") || strings.HasSuffix(n, "/") || path.Clean(n) != n {
		return false
	}
	for _, p := range strings.Split(n, "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
		for _, r := range p {
			if r < 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}
func validID(s string) bool {
	if !idRE.MatchString(s) {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
	}
	return true
}

var semverRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func validVersion(s string) bool {
	if s == "" || strings.ContainsAny(s, "/@ \t\r\n") {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
func validateDocument(k ports.ArtifactKind, id, version string, b []byte) (string, string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return "", "", fail("document must be object")
	}
	var did, dver string
	_ = json.Unmarshal(m["id"], &did)
	_ = json.Unmarshal(m["version"], &dver)
	if !validID(id) || id != did {
		return "", "", fail("descriptor/document id mismatch")
	}
	if k == ports.KindProvider {
		if !validVersion(version) {
			return "", "", fail("invalid provider version")
		}
	} else if k == ports.KindTaxonomy {
		var ed string
		_ = json.Unmarshal(m["edition"], &ed)
		if version != ed || !validVersion(version) {
			return "", "", fail("taxonomy edition mismatch")
		}
	} else if version != dver || !validVersion(version) || !semverRE.MatchString(version) {
		return "", "", fail("descriptor/document version mismatch")
	}
	switch k {
	case ports.KindCapability, ports.KindTool, ports.KindProvider, ports.KindBinding, ports.KindTaxonomy:
	default:
		return "", "", fail("unsupported artifact kind")
	}
	if err := validateSchema("https://gist.local/contracts/registry/v1/"+map[ports.ArtifactKind]string{ports.KindCapability: "capability.schema.json", ports.KindTool: "execution-schema.schema.json", ports.KindProvider: "provider.schema.json", ports.KindBinding: "binding.schema.json", ports.KindTaxonomy: "taxonomy.schema.json"}[k], b); err != nil {
		return "", "", err
	}
	if k == ports.KindCapability || k == ports.KindTool {
		if err := validateDynamicSchemas(b); err != nil {
			return "", "", err
		}
	}
	if k == ports.KindTaxonomy {
		if err := taxonomySemantics(b); err != nil {
			return "", "", err
		}
	}
	return id, version, nil
}

func validateDynamicSchemas(b []byte) error {
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return fail("invalid schema owner")
	}
	for _, key := range []string{"input_schema", "output_schema"} {
		var doc any
		if json.Unmarshal(m[key], &doc) != nil {
			return fail("invalid %s", key)
		}
		c := retainedCompiler()
		c.AssertFormat()
		c.DefaultDraft(jsonschema.Draft2020)
		if err := c.AddResource("urn:gist:dynamic:"+key, doc); err != nil {
			return fail("compile %s: %v", key, err)
		}
		sch, err := c.Compile("urn:gist:dynamic:" + key)
		if err != nil {
			return fail("compile %s: %v", key, err)
		}
		_ = sch
	}
	return nil
}
func typedSemantics(k ports.ArtifactKind, b []byte) error {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)
	switch k {
	case ports.KindCapability:
		var x struct {
			Scopes  []string `json:"required_scopes"`
			Effects struct {
				Terms []struct {
					ID string `json:"id"`
				} `json:"terms"`
				Aggregate struct {
					Class string `json:"class"`
				} `json:"aggregate"`
			} `json:"effects"`
			Cost struct {
				Currency string      `json:"currency"`
				Unit     string      `json:"unit"`
				Upper    json.Number `json:"upper_bound"`
			} `json:"cost"`
			Timeout int64 `json:"timeout_ms"`
			Retry   struct {
				Supported bool `json:"supported"`
				Max       int  `json:"max_attempts"`
			} `json:"retry"`
			Idempotency struct {
				Supported bool `json:"supported"`
			} `json:"idempotency"`
		}
		_ = json.Unmarshal(b, &x)
		if len(x.Effects.Terms) == 0 || x.Timeout <= 0 || x.Timeout > 3600000 || strings.TrimSpace(x.Cost.Currency) == "" || strings.TrimSpace(x.Cost.Unit) == "" {
			return fail("invalid capability effects, cost or timeout")
		}
		termIDs := map[string]bool{}
		for _, term := range x.Effects.Terms {
			if strings.TrimSpace(term.ID) == "" || termIDs[term.ID] {
				return fail("invalid or duplicate effect")
			}
			termIDs[term.ID] = true
		}
		seen := map[string]bool{}
		for _, scope := range x.Scopes {
			if strings.TrimSpace(scope) == "" || seen[scope] {
				return fail("invalid or duplicate scope")
			}
			seen[scope] = true
		}
		var cm map[string]json.RawMessage
		_ = json.Unmarshal(m["cost"], &cm)
		var f json.Number
		if e := json.Unmarshal(cm["upper_bound"], &f); e != nil {
			return fail("cost upper_bound must be numeric")
		}
		fv, e := f.Float64()
		if e != nil || math.IsNaN(fv) || math.IsInf(fv, 0) || fv < 0 {
			return fail("invalid cost upper_bound")
		}
		if x.Retry.Max < 0 || x.Retry.Max > 10 || x.Retry.Supported != (x.Retry.Max > 0) || x.Retry.Supported && x.Effects.Aggregate.Class != "read_only" && !x.Idempotency.Supported {
			return fail("retry support/max_attempts mismatch")
		}
	case ports.KindTool:
		var x struct {
			Action      struct{ ID, Version string } `json:"provider_action"`
			Location    string                       `json:"execution_location"`
			Lifecycle   string                       `json:"lifecycle"`
			Credentials struct {
				Scopes []string `json:"scopes"`
			} `json:"credentials"`
			Destinations []string `json:"destinations"`
			Provenance   string   `json:"provenance"`
			Capture      string   `json:"capture_time"`
		}
		_ = json.Unmarshal(b, &x)
		if !validID(x.Action.ID) || !validVersion(x.Action.Version) || x.Location != "client" && x.Location != "runtime" || x.Lifecycle != "published" && x.Lifecycle != "deprecated" && x.Lifecycle != "draft" || strings.TrimSpace(x.Provenance) == "" {
			return fail("tool action, lifecycle, execution or provenance invalid")
		}
		t, e := time.Parse(time.RFC3339, x.Capture)
		if e != nil || t.After(time.Now().Add(time.Minute)) {
			return fail("invalid/future tool capture time")
		}
		for _, u := range x.Destinations {
			if strings.TrimSpace(u) == "" {
				return fail("empty destination")
			}
			parsed, err := url.ParseRequestURI(u)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				return fail("invalid destination URI")
			}
		}
		scopeSet := map[string]bool{}
		for _, scope := range x.Credentials.Scopes {
			if strings.TrimSpace(scope) == "" || scopeSet[scope] {
				return fail("invalid credential scope")
			}
			scopeSet[scope] = true
		}
	case ports.KindProvider:
		var x struct {
			State    string `json:"support_state"`
			Verified string `json:"last_verified_at"`
			Capture  struct {
				Source string `json:"source"`
				Digest struct {
					Algorithm string `json:"algorithm"`
					Value     string `json:"value"`
				} `json:"digest"`
			} `json:"capture"`
		}
		_ = json.Unmarshal(b, &x)
		if x.State != "planned" && x.State != "catalog_only" && x.State != "resolvable" && x.State != "executable" && x.State != "degraded" && x.State != "retired" {
			return fail("invalid provider state")
		}
		t, e := time.Parse(time.RFC3339, x.Verified)
		if e != nil || t.After(time.Now().Add(time.Minute)) {
			return fail("invalid/future provider verification time")
		}
		if x.State == "resolvable" || x.State == "executable" {
			if x.Capture.Source == "" || x.Capture.Digest.Algorithm != "sha256" || !validHex(x.Capture.Digest.Value) {
				return fail("qualified provider requires retained capture reference")
			}
		}
	case ports.KindBinding:
		var x struct {
			Adapter     string `json:"adapter_version"`
			Conformance string `json:"conformance"`
			Exact       bool   `json:"exact_versions"`
		}
		_ = json.Unmarshal(b, &x)
		// JSON tags are explicit because the frozen document uses *_ref names.
		var refs map[string]string
		_ = json.Unmarshal(b, &refs)
		for _, key := range []string{"capability_ref", "tool_ref", "provider_ref"} {
			if !canonicalRef(refs[key]) {
				return fail("binding reference %q is not canonical", key)
			}
		}
		if !x.Exact || x.Adapter == "" || x.Conformance != "passed" {
			return fail("binding adapter/conformance/pins invalid")
		}
	case ports.KindSkill:
	}
	return nil
}
func validHex(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil
}
func canonicalRef(s string) bool {
	a, b, ok := strings.Cut(s, "@")
	if !ok || !validID(a) || !semverRE.MatchString(b) || strings.Contains(b, "@") {
		return false
	}
	return true
}

func taxonomySemantics(b []byte) error {
	var x struct {
		Nodes []struct {
			ID     string  `json:"id"`
			Level  int     `json:"level"`
			Label  string  `json:"label"`
			APQF   string  `json:"apqc_ref"`
			Parent *string `json:"parent_id"`
		} `json:"nodes"`
	}
	if json.Unmarshal(b, &x) != nil || len(x.Nodes) == 0 {
		return fail("taxonomy nodes required")
	}
	nodes := map[string]int{}
	parents := map[string]string{}
	for _, n := range x.Nodes {
		if !validID(n.ID) || n.Level < 1 || strings.TrimSpace(n.Label) == "" || strings.TrimSpace(n.APQF) == "" {
			return fail("invalid taxonomy node")
		}
		if _, ok := nodes[n.ID]; ok {
			return fail("duplicate taxonomy node")
		}
		nodes[n.ID] = n.Level
		if n.Parent != nil {
			parents[n.ID] = *n.Parent
		} else if n.Level != 1 {
			return fail("taxonomy root must be level one")
		}
	}
	for id, parent := range parents {
		if _, ok := nodes[parent]; !ok || nodes[id] != nodes[parent]+1 {
			return fail("taxonomy parent/level mismatch")
		}
	}
	for id := range nodes {
		seen := map[string]bool{}
		cur := id
		for cur != "" {
			if seen[cur] {
				return fail("taxonomy cycle")
			}
			seen[cur] = true
			cur = parents[cur]
		}
	}
	return nil
}

// Retained schemas and built-in dialects are the only permitted resources.
// jsonschema's default loader supports file: URLs, so explicitly deny missing
// resources for both owner schemas and client-supplied dynamic schemas.
type unretainedSchemaLoader struct{}

func (unretainedSchemaLoader) Load(string) (any, error) {
	return nil, errors.New("unretained schema resource")
}
func retainedCompiler() *jsonschema.Compiler {
	c := jsonschema.NewCompiler()
	c.UseLoader(unretainedSchemaLoader{})
	c.DefaultDraft(jsonschema.Draft2020)
	return c
}

func validateSchema(uri string, instance []byte) error {
	c := retainedCompiler()
	c.AssertFormat()
	err := fs.WalkDir(schemaFiles, "schemas", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := schemaFiles.ReadFile(p)
		if e != nil {
			return e
		}
		var v any
		if e = json.Unmarshal(b, &v); e != nil {
			return e
		} // IDs in retained schemas define canonical lookup URLs.
		var doc map[string]any
		_ = json.Unmarshal(b, &doc)
		u, _ := doc["$id"].(string)
		if u == "" {
			u = "https://gist.local/contracts/registry/" + strings.TrimPrefix(strings.TrimPrefix(p, "schemas/"), "v1/")
		}
		// The frozen inventory path expression uses PCRE negative lookaheads.
		// Go regexp cannot represent those; the package validator applies the
		// equivalent path closure predicate after structural schema validation.
		if strings.HasSuffix(u, "/inventory.schema.json") {
			scrubInventoryRegex(v)
		}
		return c.AddResource(u, v)
	})
	if err != nil {
		return fail("load retained schemas: %v", err)
	}
	s, err := c.Compile(uri)
	if err != nil {
		return fail("compile retained schema: %v", err)
	}
	var v any
	if err = json.Unmarshal(instance, &v); err != nil {
		return fail("invalid document")
	}
	if err = s.Validate(v); err != nil {
		return fail("schema violation: %v", err)
	}
	return nil
}
func scrubInventoryRegex(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	items, _ := m["items"].(map[string]any)
	props, _ := items["properties"].(map[string]any)
	p, _ := props["path"].(map[string]any)
	if p != nil {
		p["pattern"] = "^[^/\\\\]+(?:/[^/\\\\]+)*$"
	}
}

func ValidateEvidenceJSON(raw []byte) (ports.PublicationAdmissionEvidence, error) {
	var e ports.PublicationAdmissionEvidence
	if err := checkJSON(raw); err != nil {
		return e, err
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return e, fail("invalid evidence JSON")
	}
	if err := validateSchema("https://gist.local/contracts/registry/v2/admission-evidence.schema.json", raw); err != nil {
		return e, err
	}
	if e.EvidenceVersion != "1" || e.WorkspaceID == "" || e.Artifact.ID == "" || e.Artifact.Version == "" || e.Reviewer.Issuer == "" || e.Reviewer.Subject == "" {
		return e, fail("incomplete evidence")
	}
	for _, s := range []string{e.Artifact.ArtifactDigest, e.Source.CaptureDigest, e.Rights.GrantDigest} {
		if !validDigest(s) {
			return e, fail("invalid evidence digest")
		}
	}
	return e, nil
}
func validDigest(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
		return false
	}
	_, e := hex.DecodeString(strings.TrimPrefix(s, "sha256:"))
	return e == nil
}
func ValidateEvidence(p ports.PreparedPublication, principal ports.Principal, e ports.PublicationAdmissionEvidence, now time.Time, allowSynthetic bool) error {
	if e.WorkspaceID != principal.WorkspaceID || e.Artifact.Kind != p.Ref.Kind || e.Artifact.ID != p.Ref.ID || e.Artifact.Version != p.Ref.Version || e.Artifact.ArtifactDigest != "sha256:"+p.ArtifactDigest.Value {
		return fail("evidence artifact binding mismatch")
	}
	if e.Synthetic && !allowSynthetic {
		return fail("synthetic evidence is disabled")
	}
	for _, s := range []string{e.Source.CapturedAt, e.Reviewer.ReviewedAt} {
		t, er := time.Parse(time.RFC3339, s)
		if er != nil || t.After(now) {
			return fail("invalid or future evidence timestamp")
		}
	}
	src, er := base64.StdEncoding.Strict().DecodeString(e.Source.RetainedBytesBase64)
	if er != nil || base64.StdEncoding.EncodeToString(src) != e.Source.RetainedBytesBase64 || sum(src) != strings.TrimPrefix(e.Source.CaptureDigest, "sha256:") {
		return fail("retained source digest mismatch")
	}
	grant, er := base64.StdEncoding.Strict().DecodeString(e.Rights.RetainedGrantBase64)
	if er != nil || sum(grant) != strings.TrimPrefix(e.Rights.GrantDigest, "sha256:") {
		return fail("retained grant digest mismatch")
	}
	if base64.StdEncoding.EncodeToString(grant) != e.Rights.RetainedGrantBase64 || e.Rights.Scope != "workspace_only" || !e.Rights.Redistribution || !containsUse(e.Rights.AllowedUse, "private_distribution") {
		return fail("rights do not qualify")
	}
	captured, _ := time.Parse(time.RFC3339, e.Source.CapturedAt)
	reviewed, _ := time.Parse(time.RFC3339, e.Reviewer.ReviewedAt)
	if reviewed.Before(captured) {
		return fail("review predates source capture")
	}
	if p.Ref.Kind == ports.KindTool {
		var x struct {
			Provenance string `json:"provenance"`
			Capture    string `json:"capture_time"`
			Digest     struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"digest"`
		}
		_ = json.Unmarshal(p.Metadata, &x)
		if x.Provenance != e.Source.URI || x.Digest.Algorithm != "sha256" || x.Digest.Value != strings.TrimPrefix(e.Source.CaptureDigest, "sha256:") || x.Capture != e.Source.CapturedAt {
			return fail("tool source capture/provenance does not match retained evidence")
		}
	}
	if p.Ref.Kind == ports.KindProvider {
		var x struct {
			Verified string `json:"last_verified_at"`
			Capture  struct {
				Source string `json:"source"`
				Digest struct {
					Algorithm string `json:"algorithm"`
					Value     string `json:"value"`
				} `json:"digest"`
			} `json:"capture"`
		}
		_ = json.Unmarshal(p.Metadata, &x)
		if x.Capture.Source != "" && (x.Capture.Source != e.Source.URI || x.Capture.Digest.Algorithm != "sha256" || x.Capture.Digest.Value != strings.TrimPrefix(e.Source.CaptureDigest, "sha256:") || x.Verified != e.Source.CapturedAt) {
			return fail("provider capture does not match retained evidence")
		}
	}
	if p.Ref.Kind == ports.KindBinding {
		var x struct {
			Adapter string `json:"adapter_version"`
			Fixture struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"fixture_digest"`
			Cap      string `json:"capability_ref"`
			Tool     string `json:"tool_ref"`
			Provider string `json:"provider_ref"`
		}
		_ = json.Unmarshal(p.Metadata, &x)
		c := e.Conformance
		if c == nil || x.Adapter != c.AdapterVersion || x.Fixture.Algorithm != "sha256" || x.Fixture.Value != strings.TrimPrefix(c.FixtureDigest, "sha256:") || !validDigest(c.ResultDigest) || c.ExecutorID == "" || c.ExecutorRevision == "" {
			return fail("binding evidence does not pin adapter, fixture and executor")
		}
		refs := map[string]bool{x.Cap: true, x.Tool: true, x.Provider: true}
		for _, r := range c.ExactRefs {
			if !refs[r] {
				return fail("binding evidence exact refs mismatch")
			}
		}
		if len(refs) != 3 || len(c.ExactRefs) != 3 {
			return fail("binding exact reference set mismatch")
		}
	}
	if p.Ref.Kind == ports.KindSkill {
		if sum(src) != p.ArtifactDigest.Value {
			return fail("retained skill source differs from archive")
		}
		var manifest struct {
			Publisher *struct {
				ID string `json:"id"`
			} `json:"publisher"`
		}
		_ = json.Unmarshal(p.Metadata, &manifest)
		if manifest.Publisher != nil && (e.OriginPublisher == nil || e.OriginPublisher.ID != manifest.Publisher.ID) {
			return fail("manifest publisher lacks matching trusted origin evidence")
		}
	}
	if p.Ref.Kind == ports.KindProvider {
		if e.ApprovedSupportState == "" || e.ApprovedSupportState != stringValueFrom(p.Metadata, "support_state") {
			return fail("provider state lacks trusted match")
		}
		if e.ApprovedSupportState == "resolvable" || e.ApprovedSupportState == "executable" {
			if e.ProviderQualification == nil {
				return fail("provider qualification is required")
			}
		}
		if e.ProviderQualification != nil {
			t, er := time.Parse(time.RFC3339, e.ProviderQualification.ValidUntil)
			if er != nil || !now.Before(t) {
				return fail("provider qualification expired")
			}
		}
	}
	if p.Ref.Kind == ports.KindBinding {
		c := e.Conformance
		if c == nil || !c.Passed || c.ExecutedCaseCount < 5 || len(c.ExactRefs) < 3 {
			return fail("binding conformance prerequisites missing")
		}
		fb, er := base64.StdEncoding.Strict().DecodeString(c.RetainedFixtureBase64)
		if er != nil || base64.StdEncoding.EncodeToString(fb) != c.RetainedFixtureBase64 || sum(fb) != strings.TrimPrefix(c.FixtureDigest, "sha256:") {
			return fail("retained conformance fixture mismatch")
		}
	}
	if p.Ref.Kind == ports.KindTaxonomy {
		g := e.TaxonomyGrant
		if g == nil || g.TaxonomyID != p.Ref.ID || g.Edition != p.Ref.Version || !allFields(g.AllowedFields, []string{"id", "edition", "attribution", "nodes"}) {
			return fail("taxonomy grant mismatch")
		}
	}
	return nil
}
func allFields(got, want []string) bool {
	set := map[string]bool{}
	for _, x := range got {
		set[x] = true
	}
	for _, x := range want {
		if !set[x] {
			return false
		}
	}
	return true
}
func containsUse(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
func stringValueFrom(b []byte, k string) string {
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	s, _ := m[k].(string)
	return s
}

func Receipt(p ports.PreparedPublication) ([]byte, error) {
	if !validID(p.Ref.ID) || !validVersion(p.Ref.Version) || p.Ref.Kind == "" || !validHex(p.ArtifactDigest.Value) || p.MaxBytes <= 0 {
		return nil, fail("invalid prepared publication")
	}
	r := ports.PublicationReceipt{Kind: p.Ref.Kind, ID: p.Ref.ID, Version: p.Ref.Version, ArtifactDigest: "sha256:" + p.ArtifactDigest.Value}
	if p.Ref.Kind == ports.KindSkill {
		if !validHex(p.ManifestDigest.Value) || !validHex(p.PackageDigest.Value) {
			return nil, fail("invalid skill digest")
		}
		r.ManifestDigest = "sha256:" + p.ManifestDigest.Value
		r.PackageDigest = "sha256:" + p.PackageDigest.Value
	}
	b, e := json.Marshal(r)
	if e != nil {
		return nil, fail("receipt marshal")
	}
	if int64(len(b)) > p.MaxBytes {
		return nil, ErrBudgetExceeded
	}
	return b, nil
}
