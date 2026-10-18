package directory

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const MaxSnapshotBytes = 64 << 20
const MaxEntries = 100000

// ValidateRecord validates a descriptor suitable for the public directory.
// URLs are metadata only: this function never resolves or fetches them.
func ValidateRecord(r Record) error { return validateRecord(r, false) }

// ValidatePrivateRecord also permits private/localhost URLs and local packages.
// A private descriptor is interpreted by the client; Gist never fetches it.
func ValidatePrivateRecord(r Record) error { return validateRecord(r, true) }
func bounded(s string, max int) bool {
	if len(s) > max || !strings.EqualFold(strings.TrimSpace(s), s) {
		return false
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
func safeURL(s string, private bool) bool {
	if !bounded(s, 2048) {
		return false
	}
	u, e := url.Parse(s)
	if e != nil || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	h := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if u.Scheme != "https" && !(private && u.Scheme == "http") {
		return false
	}
	if !private {
		if h == "localhost" || !strings.Contains(h, ".") || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
			return false
		}
		if ip := net.ParseIP(h); ip != nil && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
			return false
		}
	}
	return true
}
func validateRecord(r Record, private bool) error {
	fail := func(s string) error { return fmt.Errorf("directory record %q: %s", r.ID, s) }
	if r.ID == "" || !bounded(r.ID, 200) || r.ServiceID == "" || !bounded(r.ServiceID, 200) || r.Title == "" || !bounded(r.Title, 200) || !bounded(r.Description, 3000) || !bounded(r.Version, 200) {
		return fail("invalid identity or text")
	}
	if r.Kind != "mcp" && r.Kind != "openapi" {
		return fail("unknown kind")
	}
	if !private && (r.Source.ID == "" || r.Source.EntryID == "" || !safeURL(r.Source.URL, false)) {
		return fail("missing public provenance")
	}
	if !bounded(r.Source.ID, 200) || !bounded(r.Source.EntryID, 300) {
		return fail("invalid source identity")
	}
	for _, u := range []string{r.Homepage, r.Repository, r.Auth.SetupURL, r.OfficialEvidenceURL, r.ServerListingURL} {
		if u != "" && !safeURL(u, private) {
			return fail("unsafe metadata URL")
		}
	}
	if !private && r.Official && r.OfficialEvidenceURL == "" {
		return fail("official requires vendor evidence")
	}
	if !private && !r.Official && (r.Kind != "mcp" || r.ServerListingURL == "" || r.OfficialEvidenceURL != "") {
		return fail("independent remote MCP requires registry provenance and no vendor claim")
	}
	if len(r.Tags) > 20 || len(r.Remotes) > 16 || len(r.Packages) > 16 || len(r.Skills) > 12 || len(r.Verification) > 12 || len(r.Auth.CredentialNames) > 24 {
		return fail("too many descriptor fields")
	}
	for _, s := range r.Tags {
		if s == "" || !bounded(s, 64) {
			return fail("invalid tag")
		}
	}
	switch r.Auth.Type {
	case "", "none", "unknown", "oauth2", "api_key", "api-key", "bearer", "basic", "environment", "credential-required", "oauth2-or-token":
	default:
		return fail("invalid authentication type")
	}
	for _, s := range r.Auth.CredentialNames {
		if s == "" || len(s) > 128 {
			return fail("invalid credential name")
		}
		for _, c := range s {
			if !(c == '_' || c == '-' || c == '.' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				return fail("credential values are forbidden")
			}
		}
	}
	for _, v := range r.Remotes {
		if !safeURL(v.URL, private) || (v.Transport != "streamable-http" && v.Transport != "sse") {
			return fail("invalid remote")
		}
		if v.Check != nil {
			status := v.Check.HTTPStatus
			validStatus := status >= 200 && status < 400 || status == 400 || status == 401 || status == 402 || status == 403 || status == 405
			if v.Check.CheckedAt.IsZero() || !validStatus {
				return fail("invalid endpoint check")
			}
			resultValid := false
			switch v.Check.Result {
			case "mcp-initialize":
				resultValid = status == 200 && v.Transport == "streamable-http"
			case "endpoint-responded":
				resultValid = status >= 200 && status < 400 || status == 400 || status == 405
			case "authentication-required":
				resultValid = status == 401 || status == 403
			case "payment-required":
				resultValid = status == 402
			}
			if !resultValid {
				return fail("endpoint check result does not match HTTP status and transport")
			}
		}
	}
	for _, v := range r.Packages {
		if v.Identifier == "" || !bounded(v.Identifier, 512) || !bounded(v.Version, 200) || !bounded(v.Runtime, 64) {
			return fail("invalid package")
		}
		switch v.Runtime {
		case "", "node", "npx", "python", "python3", "uv", "uvx", "bun", "bunx", "docker", "podman", "native":
		default:
			return fail("unknown package runtime")
		}
		switch v.Registry {
		case "npm", "pypi", "oci":
			if strings.ContainsAny(v.Identifier, " \t\n\r;&|$`<>\"'()*?!#") || strings.Contains(v.Identifier, "://") {
				return fail("invalid package identifier")
			}
		case "local":
			if !private {
				return fail("local package must be private")
			}
			if strings.ContainsAny(v.Identifier, ";&|$`<>\"'()*?!") {
				return fail("local package must be a path or executable name, not a command")
			}
		default:
			return fail("unknown package registry")
		}
	}
	if r.Kind == "openapi" {
		if r.OpenAPI == nil || !safeURL(r.OpenAPI.SpecURL, private) || !safeURL(r.OpenAPI.MintGuideURL, false) {
			return fail("invalid OpenAPI descriptor")
		}
	} else if r.OpenAPI != nil || len(r.Remotes)+len(r.Packages) == 0 {
		return fail("MCP requires a remote or package")
	}
	for _, s := range r.Skills {
		if s.ID == "" || !bounded(s.ID, 200) || s.Title == "" || !bounded(s.Title, 200) || (s.URL != "" && !safeURL(s.URL, private)) || (s.Status != "candidate" && s.Status != "qualified") || (s.SourceDigest != "" && !pinned(s.SourceDigest)) {
			return fail("invalid skill reference")
		}
	}
	for _, v := range r.Verification {
		if !bounded(v.Client, 200) || !bounded(v.ClientVersion, 200) || !bounded(v.Workflow, 200) || !bounded(v.Version, 200) || !bounded(v.Result, 32) || !bounded(v.DescriptorDigest, 71) || !bounded(v.ToolDigest, 71) || !bounded(v.SchemaDigest, 71) || !bounded(v.SkillDigest, 71) || (v.EvidenceURL != "" && !safeURL(v.EvidenceURL, false)) {
			return fail("invalid qualification")
		}
	}
	return nil
}

// IsFeatured computes eligibility from operator selection and complete dated
// client workflow evidence. Upstream listing and recommendation are insufficient.
func pinned(s string) bool {
	if !strings.HasPrefix(s, "sha256:") {
		return false
	}
	b, e := hex.DecodeString(strings.TrimPrefix(s, "sha256:"))
	return e == nil && len(b) == 32
}

// DescriptorDigest pins the complete public descriptor to a qualification.
// Qualification evidence and operator/derived flags are excluded to avoid a
// circular digest; connection, source, auth, skill and presentation fields remain.
func DescriptorDigest(r Record) string {
	r = clone(r)
	r.Verification = nil
	r.Recommended = false
	r.Official = false
	r.FeaturedSelection = false
	r.Featured = false
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func IsFeatured(r Record, now time.Time) bool {
	if !r.FeaturedSelection || r.Version == "" || ValidateRecord(r) != nil {
		return false
	}
	for _, v := range r.Verification {
		if v.Result == "passed" && strings.EqualFold(v.Client, "zati") && v.ClientVersion != "" && bounded(v.ClientVersion, 200) && v.Workflow != "" && v.DescriptorDigest == DescriptorDigest(r) && pinned(v.ToolDigest) && pinned(v.SchemaDigest) && pinned(v.SkillDigest) && v.Version == r.Version && safeURL(v.EvidenceURL, false) && !v.CheckedAt.IsZero() && !v.CheckedAt.After(now) && v.ExpiresAt.After(now) && v.ExpiresAt.After(v.CheckedAt) {
			for _, skill := range r.Skills {
				if skill.Status == "qualified" && safeURL(skill.URL, false) && skill.SourceDigest == v.SkillDigest {
					return true
				}
			}
		}
	}
	return false
}

type Index struct {
	snapshot Snapshot
	records  map[string]Record
	ids      []string
	words    []string
	postings map[string][]string
	tags     map[string]map[string]bool
	kinds    map[string]map[string]bool
}

func tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsNumber(c) })
}
func New(s Snapshot) (*Index, error) {
	if s.Format != SnapshotFormat || s.Revision == "" || !bounded(s.Revision, 200) || s.GeneratedAt.IsZero() || len(s.Entries) > MaxEntries || len(s.Sources) > 100 {
		return nil, errors.New("invalid directory snapshot header")
	}
	sources := map[string]bool{}
	for _, v := range s.Sources {
		b, e := hex.DecodeString(v.SHA256)
		if v.ID == "" || !bounded(v.ID, 200) || sources[v.ID] || !safeURL(v.URL, false) || len(b) != 32 || e != nil || v.Revision == "" || !bounded(v.Revision, 300) || v.RetrievedAt.IsZero() || v.Attribution == "" || !bounded(v.Attribution, 1000) || !bounded(v.License, 100) {
			return nil, errors.New("invalid directory source")
		}
		sources[v.ID] = true
	}
	idx := &Index{snapshot: s, records: map[string]Record{}, postings: map[string][]string{}, tags: map[string]map[string]bool{}, kinds: map[string]map[string]bool{}}
	idx.snapshot.Entries = nil
	idx.snapshot.Sources = append([]Source(nil), s.Sources...)
	for _, original := range s.Entries {
		if e := ValidateRecord(original); e != nil {
			return nil, e
		}
		if !sources[original.Source.ID] {
			return nil, fmt.Errorf("unknown source for %s", original.ID)
		}
		if _, ok := idx.records[original.ID]; ok {
			return nil, fmt.Errorf("duplicate record %s", original.ID)
		}
		r := clone(original)
		r.Featured = false
		idx.records[r.ID] = r
		idx.ids = append(idx.ids, r.ID)
		seen := map[string]bool{}
		for _, w := range tokens(r.Title + " " + r.ID + " " + r.Description + " " + r.ServiceID + " " + strings.Join(r.Tags, " ")) {
			if !seen[w] {
				idx.postings[w] = append(idx.postings[w], r.ID)
				seen[w] = true
			}
		}
		if idx.kinds[r.Kind] == nil {
			idx.kinds[r.Kind] = map[string]bool{}
		}
		idx.kinds[r.Kind][r.ID] = true
		for _, tag := range r.Tags {
			tag = strings.ToLower(tag)
			if idx.tags[tag] == nil {
				idx.tags[tag] = map[string]bool{}
			}
			idx.tags[tag][r.ID] = true
		}
	}
	sort.Slice(idx.ids, func(i, j int) bool {
		a, b := idx.records[idx.ids[i]], idx.records[idx.ids[j]]
		if a.Recommended != b.Recommended {
			return a.Recommended
		}
		if strings.ToLower(a.Title) != strings.ToLower(b.Title) {
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		}
		return a.ID < b.ID
	})
	for w := range idx.postings {
		idx.words = append(idx.words, w)
	}
	sort.Strings(idx.words)
	return idx, nil
}
func Parse(data []byte) (*Index, error) {
	if len(data) > MaxSnapshotBytes {
		return nil, errors.New("directory snapshot too large")
	}
	var s Snapshot
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if e := d.Decode(&s); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e == nil {
		return nil, errors.New("trailing directory JSON")
	} else if e != io.EOF {
		return nil, e
	}
	return New(s)
}
func clone(r Record) Record {
	r.Tags = append([]string(nil), r.Tags...)
	r.Remotes = append([]Remote(nil), r.Remotes...)
	for n := range r.Remotes {
		if r.Remotes[n].Check != nil {
			check := *r.Remotes[n].Check
			r.Remotes[n].Check = &check
		}
	}
	r.Packages = append([]Package(nil), r.Packages...)
	r.Auth.CredentialNames = append([]string(nil), r.Auth.CredentialNames...)
	r.Skills = append([]Skill(nil), r.Skills...)
	r.Verification = append([]Qualification(nil), r.Verification...)
	if r.OpenAPI != nil {
		v := *r.OpenAPI
		r.OpenAPI = &v
	}
	return r
}
func (i *Index) Get(id string) (Record, bool) {
	r, ok := i.records[id]
	if !ok {
		return Record{}, false
	}
	r = clone(r)
	r.Featured = IsFeatured(r, time.Now().UTC())
	return r, true
}
func (i *Index) Revision() string  { return i.snapshot.Revision }
func (i *Index) Count() int        { return len(i.ids) }
func (i *Index) Sources() []Source { return append([]Source(nil), i.snapshot.Sources...) }
func (i *Index) Search(q Query) (Result, error) {
	q.Text = strings.TrimSpace(q.Text)
	q.Tag = strings.ToLower(strings.TrimSpace(q.Tag))
	if len(q.Text) > 200 || len(q.Tag) > 64 || len(q.Cursor) > 1024 || q.Limit < 0 || q.Limit > 100 || (q.Kind != "" && q.Kind != "mcp" && q.Kind != "openapi") {
		return Result{}, errors.New("invalid directory query")
	}
	if q.Limit == 0 {
		q.Limit = 24
	}
	terms := tokens(q.Text)
	if len(terms) > 16 {
		return Result{}, errors.New("too many search terms")
	}
	sum := sha256.Sum256([]byte(i.Revision() + "\n" + strings.ToLower(q.Text) + "\n" + q.Kind + "\n" + q.Tag))
	key := hex.EncodeToString(sum[:16])
	offset := 0
	if q.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil {
			return Result{}, errors.New("invalid cursor")
		}
		p := strings.Split(string(raw), ":")
		if len(p) != 2 || p[0] != key {
			return Result{}, errors.New("cursor does not match query or snapshot")
		}
		offset, e = strconv.Atoi(p[1])
		if e != nil || offset < 0 || offset > len(i.ids) {
			return Result{}, errors.New("invalid cursor offset")
		}
	}
	var matches map[string]bool
	for _, term := range terms {
		m := map[string]bool{}
		start := sort.SearchStrings(i.words, term)
		for n := start; n < len(i.words) && strings.HasPrefix(i.words[n], term); n++ {
			for _, id := range i.postings[i.words[n]] {
				m[id] = true
			}
		}
		if matches == nil {
			matches = m
		} else {
			for id := range matches {
				if !m[id] {
					delete(matches, id)
				}
			}
		}
	}
	ids := make([]string, 0)
	for _, id := range i.ids {
		if matches != nil && !matches[id] || q.Kind != "" && !i.kinds[q.Kind][id] || q.Tag != "" && !i.tags[q.Tag][id] {
			continue
		}
		ids = append(ids, id)
	}
	if offset > len(ids) {
		return Result{}, errors.New("cursor offset exceeds results")
	}
	end := offset + q.Limit
	if end > len(ids) {
		end = len(ids)
	}
	result := Result{Items: make([]Record, 0, end-offset), Total: len(ids), SnapshotRevision: i.Revision()}
	for _, id := range ids[offset:end] {
		r, _ := i.Get(id)
		result.Items = append(result.Items, r)
	}
	if end < len(ids) {
		result.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(key + ":" + strconv.Itoa(end)))
	}
	return result, nil
}
