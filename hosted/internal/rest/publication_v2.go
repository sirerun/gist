package rest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func v2Kind(raw string) (ports.ArtifactKind, bool) {
	switch raw {
	case "skill":
		return ports.KindSkill, true
	case "capability":
		return ports.KindCapability, true
	case "tool":
		return ports.KindTool, true
	case "provider":
		return ports.KindProvider, true
	case "binding":
		return ports.KindBinding, true
	case "taxonomy":
		return ports.KindTaxonomy, true
	default:
		return "", false
	}
}

func (h *Handler) publishV2(w http.ResponseWriter, r *http.Request, p ports.Principal, rawKind string) error {
	kind, ok := v2Kind(rawKind)
	if !ok || h.s.V2Publisher == nil {
		return appError("not_found", "Not found", 404, false)
	}
	if r.URL.RawQuery != "" {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	if err := h.authorize(r.Context(), p, ports.ActionPublish, nil); err != nil {
		return err
	}
	if strings.TrimSpace(r.Header.Get("Content-Type")) != "application/json" {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	limit := h.limits.MaxBodyBytes
	if limit > 16<<20 {
		limit = 16 << 20
	}
	raw, err := readBody(r, limit)
	if err != nil {
		return err
	}
	result, err := h.s.V2Publisher.PublishV2(r.Context(), p, kind, raw)
	if err != nil {
		return v2PublicError(err)
	}
	if len(result.Body) == 0 || len(result.Body) > h.limits.MaxResponseBytes {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if !validV2Digest(result.ArtifactDigest) || kind == ports.KindSkill && (!validV2Digest(result.ManifestDigest) || !validV2Digest(result.PackageDigest)) {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	setV2Digest(w, "X-Gist-Artifact-Digest", result.ArtifactDigest)
	if kind == ports.KindSkill {
		setV2Digest(w, "X-Gist-Manifest-Digest", result.ManifestDigest)
		setV2Digest(w, "X-Gist-Package-Digest", result.PackageDigest)
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	w.WriteHeader(status)
	_, err = w.Write(result.Body)
	return err
}

func (h *Handler) readV2(w http.ResponseWriter, r *http.Request, p ports.Principal, rawKind string, packageBody bool) error {
	kind, ok := v2Kind(rawKind)
	if !ok || h.s.V2Reader == nil {
		return appError("not_found", "Not found", 404, false)
	}
	if packageBody && kind != ports.KindSkill {
		return appError("not_found", "Not found", 404, false)
	}
	q, queryErr := url.ParseQuery(r.URL.RawQuery)
	if queryErr != nil {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	for _, pair := range strings.Split(r.URL.RawQuery, "&") {
		if pair == "" || !strings.Contains(pair, "=") {
			return appError("validation_failed", "Invalid request", 422, false)
		}
	}
	allowed := map[string]bool{"id": true, "version": true, "max_bytes": true}
	if packageBody {
		allowed = map[string]bool{"id": true, "version": true, "max_bytes": true}
	}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return appError("validation_failed", "Invalid request", 422, false)
		}
	}
	id, version := q.Get("id"), q.Get("version")
	if !validPublicationID(id) || !validPublicationVersion(version, kind) || q.Get("max_bytes") == "" {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	limit := int64(h.limits.MaxResponseBytes)
	if limit > 100<<20 {
		limit = 100 << 20
	}
	if value, exists := q["max_bytes"]; exists {
		n, err := strconv.ParseInt(value[0], 10, 64)
		if err != nil || n <= 0 || strconv.FormatInt(n, 10) != value[0] {
			return appError("validation_failed", "Invalid request", 422, false)
		}
		if n < limit {
			limit = n
		}
	}
	ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: kind, ID: id, Version: version}
	if err := h.authorize(r.Context(), p, ports.ActionRead, &ref); err != nil {
		return err
	}
	got, err := h.s.V2Reader.ReadV2(r.Context(), p, ref, limit, packageBody)
	if err != nil {
		return v2PublicError(err)
	}
	if int64(len(got.Body)) > limit {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	bodySum := sha256.Sum256(got.Body)
	expectedContentType := "application/json"
	if packageBody {
		expectedContentType = "application/zip"
	}
	if got.ContentType != expectedContentType || got.BodyDigest.Algorithm != "sha256" || got.BodyDigest.Value != hex.EncodeToString(bodySum[:]) || !validV2Digest(got.ArtifactDigest) || kind == ports.KindSkill && (!validV2Digest(got.ManifestDigest) || !validV2Digest(got.PackageDigest)) {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", got.ContentType)
	setV2Digest(w, "X-Gist-Body-Digest", got.BodyDigest)
	setV2Digest(w, "X-Gist-Artifact-Digest", got.ArtifactDigest)
	if kind == ports.KindSkill {
		setV2Digest(w, "X-Gist-Manifest-Digest", got.ManifestDigest)
		setV2Digest(w, "X-Gist-Package-Digest", got.PackageDigest)
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(got.Body)
	return err
}

func validPublicationID(value string) bool {
	if len(value) == 0 || len(value) > 256 {
		return false
	}
	for i, r := range value {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '/' || r == '-'
		if !valid || i == 0 && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

var publicationSemver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-+][0-9A-Za-z.-]+)?$`)

func validPublicationVersion(value string, kind ports.ArtifactKind) bool {
	if len(value) == 0 || len(value) > 256 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) || r == '@' || r == '/' {
			return false
		}
	}
	return kind == ports.KindTaxonomy || publicationSemver.MatchString(value)
}

func setV2Digest(w http.ResponseWriter, name string, digest ports.Digest) {
	if digest.Algorithm == "sha256" && len(digest.Value) == 64 {
		if _, err := hex.DecodeString(digest.Value); err == nil {
			w.Header().Set(name, "sha256:"+strings.ToLower(digest.Value))
		}
	}
}

func validV2Digest(d ports.Digest) bool {
	if d.Algorithm != "sha256" || len(d.Value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(d.Value)
	return err == nil && d.Value == strings.ToLower(d.Value)
}

func v2PublicError(err error) error {
	if errors.Is(err, ErrBudgetExceeded) {
		return appError("budget_exceeded", "Required content exceeds the requested byte budget", 413, false)
	}
	if errors.Is(err, ErrValidationFailed) {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	if errors.Is(err, ErrPublicationDenied) {
		return appError("forbidden", "Forbidden", 403, false)
	}
	if errors.Is(err, ErrPublicationConflict) {
		return appError("version_conflict", "Version conflict", 409, false)
	}
	if errors.Is(err, ErrPublicationNotFound) {
		return appError("not_found", "Not found", 404, false)
	}
	// Storage errors are deliberately collapsed here. Internal parser, SQL,
	// evidence and object details are not part of the public error contract.
	return appError("service_unavailable", "Service unavailable", 503, true)
}
