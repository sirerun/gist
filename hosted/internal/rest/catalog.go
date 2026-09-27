package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func (h *Handler) record(ctx context.Context, p ports.Principal, kind ports.ArtifactKind, id, version string) (ports.CatalogRecord, error) {
	ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: kind, ID: id, Version: version}
	if err := h.authorize(ctx, p, ports.ActionRead, &ref); err != nil {
		return ports.CatalogRecord{}, err
	}
	if h.s.Catalog == nil {
		return ports.CatalogRecord{}, appError("service_unavailable", "Service unavailable", 503, true)
	}
	r, err := h.s.Catalog.Get(ctx, ref)
	if err != nil {
		return ports.CatalogRecord{}, appError("not_found", "Not found", 404, false)
	}
	// Revocation detail is disclosed only after authorization (ADR 005).
	if r.State == "revoked" {
		return ports.CatalogRecord{}, ErrArtifactRevoked
	}
	return r, nil
}
func (h *Handler) writeJSON(w http.ResponseWriter, v any, max int) error {
	b, err := json.Marshal(v)
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if max > 0 && len(b) > max {
		return appError("budget_exceeded", "Response exceeds the requested byte budget", 413, false)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(b)
	return err
}
func (h *Handler) getArtifact(w http.ResponseWriter, r *http.Request, p ports.Principal, kind ports.ArtifactKind, id, version string) error {
	rec, err := h.record(r.Context(), p, kind, id, version)
	if err != nil {
		return err
	}
	return h.writeJSON(w, json.RawMessage(rec.Metadata), budget(r, h.limits.MaxResponseBytes))
}
func (h *Handler) listVersions(w http.ResponseWriter, r *http.Request, p ports.Principal, id string) error {
	if h.s.Versions == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	ref := &ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: ports.KindSkill, ID: id}
	if err := h.authorize(r.Context(), p, ports.ActionRead, ref); err != nil {
		return err
	}
	limit, after, err := h.versionPage(r)
	if err != nil {
		return err
	}
	// Fetch this id's versions directly. Filtering a catalog-wide search page
	// dropped every version beyond the page's MaxResults cap. One extra row
	// tells whether another page follows.
	records, err := h.s.Versions.ListVersions(r.Context(), *ref, after, limit+1)
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	items := make([]ports.CatalogRecord, 0, len(records))
	for _, rec := range records {
		if rec.Ref.ID == id && rec.Ref.Kind == ports.KindSkill && rec.Ref.WorkspaceID == p.WorkspaceID {
			items = append(items, rec)
		}
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = base64.RawURLEncoding.EncodeToString([]byte(items[limit-1].Ref.Version))
	}
	return h.writeJSON(w, map[string]any{"items": items, "next_cursor": next}, budget(r, h.limits.MaxResponseBytes))
}

// versionPage reads the keyset paging parameters of GET
// /v1/skills/{id}/versions: limit (1..MaxResults, default MaxResults) and the
// opaque cursor a previous page returned as next_cursor.
func (h *Handler) versionPage(r *http.Request) (int, string, error) {
	q := r.URL.Query()
	limit := h.limits.MaxResults
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return 0, "", appError("validation_failed", "Invalid request", 422, false)
		}
		if n < limit {
			limit = n
		}
	}
	after := ""
	if raw := q.Get("cursor"); raw != "" {
		b, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil || len(b) == 0 || len(b) > maxVersionLength {
			return 0, "", appError("validation_failed", "Invalid request", 422, false)
		}
		after = string(b)
	}
	return limit, after, nil
}

// maxVersionLength matches the contract's Version path parameter bound.
const maxVersionLength = 128

func (h *Handler) discover(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	var in struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
		MaxBytes   int    `json:"max_bytes"`
	}
	if err := decodeBody(r, h.limits.MaxBodyBytes, &in); err != nil {
		return err
	}
	if in.Query == "" || in.MaxBytes < 1 {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	if h.s.Search == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	limit := in.MaxResults
	if limit <= 0 || limit > h.limits.MaxResults {
		limit = h.limits.MaxResults
	}
	page, err := h.s.Search.Search(r.Context(), ports.SearchQuery{Principal: p, Text: in.Query, Limit: limit, MaxBytes: in.MaxBytes})
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	// Budgets reduce the candidate count; they never fail a discovery that fits
	// with fewer candidates (RFC-002 §8). Records are ranked, so dropping from
	// the tail keeps the best matches. The discovery service budgets its own
	// encoding, which is smaller than this envelope, so trim here as well.
	records := page.Records
	for len(records) > 0 {
		b, err := json.Marshal(map[string]any{"items": records, "next_cursor": page.Next.ID})
		if err != nil {
			return appError("service_unavailable", "Service unavailable", 503, true)
		}
		if len(b) <= in.MaxBytes {
			break
		}
		records = records[:len(records)-1]
	}
	return h.writeJSON(w, map[string]any{"items": records, "next_cursor": page.Next.ID}, in.MaxBytes)
}
func (h *Handler) download(w http.ResponseWriter, r *http.Request, p ports.Principal, id, version string) error {
	ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: ports.KindSkill, ID: id, Version: version}
	if err := h.authorize(r.Context(), p, ports.ActionRead, &ref); err != nil {
		return err
	}
	if h.s.Artifacts == nil || h.s.Catalog == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if rec, e := h.s.Catalog.Get(r.Context(), ref); e == nil && rec.State == "revoked" {
		return ErrArtifactRevoked
	}
	a, err := h.s.Artifacts.Open(r.Context(), ref)
	if err != nil {
		return appError("not_found", "Not found", 404, false)
	}
	defer a.Close()
	b, err := io.ReadAll(io.LimitReader(a, h.limits.MaxBodyBytes+1))
	if err != nil {
		return appError("integrity_error", "Artifact integrity check failed", 422, false)
	}
	if int64(len(b)) > h.limits.MaxBodyBytes {
		return appError("budget_exceeded", "Required content exceeds the requested byte budget", 413, false)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", itoa(len(b)))
	if rec, e := h.s.Catalog.Get(r.Context(), ref); e == nil && rec.Digest.Value != "" {
		w.Header().Set("Digest", rec.Digest.Algorithm+"="+rec.Digest.Value)
		w.Header().Set("ETag", `"`+rec.Digest.Value+`"`)
	}
	if r.Header.Get("If-None-Match") != "" { /* policy was already checked; never disclose a denied 304 */
		if r.Header.Get("If-None-Match") == w.Header().Get("ETag") {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(b)
	return err
}
func itoa(n int) string { return strconv.Itoa(n) }
