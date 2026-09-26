package rest

import (
	"context"
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
	if h.s.Search == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	ref := &ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: ports.KindSkill, ID: id}
	if err := h.authorize(r.Context(), p, ports.ActionRead, ref); err != nil {
		return err
	}
	q := ports.SearchQuery{Principal: p, Kinds: []ports.ArtifactKind{ports.KindSkill}, Limit: h.limits.MaxResults, MaxBytes: budget(r, h.limits.MaxResponseBytes)}
	page, err := h.s.Search.Search(r.Context(), q)
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	return h.writeJSON(w, map[string]any{"items": page.Records, "next_cursor": page.Next.ID}, q.MaxBytes)
}
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
	return h.writeJSON(w, map[string]any{"items": page.Records, "next_cursor": page.Next.ID}, in.MaxBytes)
}
func (h *Handler) download(w http.ResponseWriter, r *http.Request, p ports.Principal, id, version string) error {
	ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: ports.KindSkill, ID: id, Version: version}
	if err := h.authorize(r.Context(), p, ports.ActionRead, &ref); err != nil {
		return err
	}
	if h.s.Artifacts == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
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
