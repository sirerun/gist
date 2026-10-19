package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/semantic"
	"github.com/sirerun/gist/hosted/internal/storage"
)

func publicDirectoryPath(path string) bool {
	return path == "/api/directory/authorization" || path == "/api/directory" || path == "/api/directory/item" || path == "/api/directory/export" || path == "/api/directory/apps" || path == "/api/directory/app"
}

type directoryAPI struct {
	search     *semantic.Service
	index      *directory.Index
	apps       *directory.AppIndex
	origin     string
	maxBytes   int
	concurrent chan struct{}
}

func (h *directoryAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Featured eligibility expires dynamically; never cache a stale badge.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Origin")
	if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != h.origin {
		webError(w, 403, "origin_denied", "Use the Gist catalog origin.")
		return
	}
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		webError(w, 405, "method_not_allowed", "Use GET.")
		return
	}
	if h.concurrent != nil {
		select {
		case h.concurrent <- struct{}{}:
			defer func() { <-h.concurrent }()
		default:
			webError(w, 429, "rate_limited", "Try again shortly.")
			return
		}
	}
	if len(r.URL.RawQuery) > 4096 {
		webError(w, 400, "invalid_query", "Query is too long.")
		return
	}
	v, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		webError(w, 400, "invalid_query", "Invalid query.")
		return
	}
	var result any
	switch r.URL.Path {
	case "/api/directory/apps":
		if !webQueryKeys(v, "q", "category", "offset", "limit", "connectable") {
			webError(w, 400, "invalid_query", "Unsupported query parameters.")
			return
		}
		connectable := false
		if v.Has("connectable") {
			if v.Get("connectable") != "true" && v.Get("connectable") != "false" {
				webError(w, 400, "invalid_query", "Connectable must be true or false.")
				return
			}
			connectable = v.Get("connectable") == "true"
		}
		limit, offset := 24, 0
		if v.Has("limit") {
			limit, err = strconv.Atoi(v.Get("limit"))
			if err != nil || limit < 1 || limit > 100 {
				webError(w, 400, "invalid_query", "Limit must be 1 to 100.")
				return
			}
		}
		if v.Has("offset") {
			offset, err = strconv.Atoi(v.Get("offset"))
			if err != nil || offset < 0 || offset > directory.MaxEntries {
				webError(w, 400, "invalid_query", "Invalid offset.")
				return
			}
		}
		if h.apps == nil {
			webError(w, 503, "service_unavailable", "App directory unavailable.")
			return
		}
		query := directory.AppQuery{Text: v.Get("q"), Category: v.Get("category"), ConnectableOnly: connectable, Offset: offset, Limit: limit}
		if h.search != nil {
			result, err = h.search.SearchApps(r.Context(), query)
		} else {
			result, err = h.apps.Search(query)
		}
		if err != nil {
			webError(w, 400, "invalid_query", "Invalid app query.")
			return
		}
	case "/api/directory/app":
		if h.apps == nil {
			webError(w, 503, "service_unavailable", "App directory unavailable.")
			return
		}
		if !webQueryKeys(v, "slug") || !webRefValue(v.Get("slug"), 256) {
			webError(w, 400, "invalid_query", "Choose an app slug.")
			return
		}
		item, ok := h.apps.Get(v.Get("slug"))
		if !ok {
			webError(w, 404, "not_found", "App not found.")
			return
		}
		result = item
	case "/api/directory":
		if !webQueryKeys(v, "q", "kind", "tag", "cursor", "limit") {
			webError(w, 400, "invalid_query", "Unsupported query parameters.")
			return
		}
		limit := 24
		if v.Has("limit") {
			limit, err = strconv.Atoi(v.Get("limit"))
			if err != nil || limit < 1 || limit > 50 {
				webError(w, 400, "invalid_query", "Limit must be 1 to 50.")
				return
			}
		}
		var search directory.Result
		query := directory.Query{Text: v.Get("q"), Kind: v.Get("kind"), Tag: v.Get("tag"), Cursor: v.Get("cursor"), Limit: limit}
		if h.search != nil {
			search, err = h.search.Search(r.Context(), query)
		} else {
			search, err = h.index.Search(query)
		}
		if err != nil {
			webError(w, 400, "invalid_query", "Invalid directory query or expired cursor.")
			return
		}
		result = directory.PublicResultOf(search)
	case "/api/directory/authorization":
		if !webQueryKeys(v, "id", "app", "version") || (v.Get("id") == "") == (v.Get("app") == "") {
			webError(w, 400, "invalid_query", "Choose exactly one connector ID or app slug.")
			return
		}
		version := 1
		if v.Has("version") {
			version, err = strconv.Atoi(v.Get("version"))
			if err != nil || (version != 1 && version != 2) {
				webError(w, 400, "invalid_query", "Version must be 1 or 2.")
				return
			}
		}
		if slug := v.Get("app"); slug != "" {
			if !webRefValue(slug, 256) {
				webError(w, 400, "invalid_query", "Choose an app slug.")
				return
			}
			if h.apps == nil {
				webError(w, 503, "service_unavailable", "App directory unavailable.")
				return
			}
			a, ok := h.apps.Get(slug)
			if !ok {
				webError(w, 404, "not_found", "App not found.")
				return
			}
			if version == 2 {
				result, err = directory.AuthorizationForAppV2(a, h.index)
			} else {
				result, err = directory.AuthorizationForApp(a, h.index)
			}
		} else {
			if !webRefValue(v.Get("id"), 512) {
				webError(w, 400, "invalid_query", "Choose an entry ID.")
				return
			}
			record, ok := h.index.Get(v.Get("id"))
			if !ok {
				webError(w, 404, "not_found", "Directory entry not found.")
				return
			}
			result, err = directory.AuthorizationFor(record)
		}
		if err != nil {
			webError(w, 503, "service_unavailable", "Authorization guidance unavailable.")
			return
		}
	case "/api/directory/item", "/api/directory/export":
		if !webQueryKeys(v, "id") || !webRefValue(v.Get("id"), 512) {
			webError(w, 400, "invalid_query", "Choose an entry ID.")
			return
		}
		record, ok := h.index.Get(v.Get("id"))
		if !ok {
			webError(w, 404, "not_found", "Directory entry not found.")
			return
		}
		result = directory.PublicRecordOf(record)
		w.Header().Set("Link", "</api/directory/authorization?id="+url.QueryEscape(record.ID)+">; rel=\"describedby\"; type=\"application/json\"")
		if r.URL.Path == "/api/directory/export" {
			result = directory.PublicExportOf(directory.Export{Format: directory.ExportFormat, Record: record})
		}
	default:
		webError(w, 404, "not_found", "Directory route not found.")
		return
	}
	switch value := result.(type) {
	case directory.PublicResult:
		if value.SearchMode != "" {
			w.Header().Set("X-Gist-Search-Mode", value.SearchMode)
		}
	case directory.AppResult:
		if value.SearchMode != "" {
			w.Header().Set("X-Gist-Search-Mode", value.SearchMode)
		}
	}
	raw, err := json.Marshal(result)
	max := h.maxBytes
	if max <= 0 {
		max = 512 << 10
	}
	if err != nil || len(raw) > max {
		webError(w, 503, "response_too_large", "Use a smaller page.")
		return
	}
	_, _ = w.Write(raw)
}

type privateDirectoryEntry struct {
	ID        string           `json:"id"`
	Record    directory.Record `json:"record"`
	CreatedAt time.Time        `json:"created_at"`
}
type privateDirectoryStore interface {
	list(context.Context, ports.Principal, string) ([]privateDirectoryEntry, error)
	create(context.Context, ports.Principal, directory.Record) (privateDirectoryEntry, error)
	remove(context.Context, ports.Principal, string) error
}

var errDirectoryDenied = errors.New("directory authority denied")
var errDirectoryFull = errors.New("directory capacity reached")

func (h *webAPI) csrfToken(r *http.Request) string {
	session, err := h.sessions.Authenticate(r)
	if err != nil || session.ID == "" || len(h.csrfKey) < 32 {
		return ""
	}
	m := hmac.New(sha256.New, h.csrfKey)
	raw, _ := json.Marshal([]string{"gist private directory csrf v1", session.ID, session.Subject, h.workspace})
	_, _ = m.Write(raw)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func (h *webAPI) servePrivateDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" && r.Method != "DELETE" {
		w.Header().Set("Allow", "GET, POST, DELETE")
		webError(w, 405, "method_not_allowed", "Use GET, POST or DELETE.")
		return
	}
	if h.concurrent != nil {
		select {
		case h.concurrent <- struct{}{}:
			defer func() { <-h.concurrent }()
		default:
			webError(w, 429, "rate_limited", "Try again shortly.")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	p, status := h.principal(r)
	if status != 200 {
		webError(w, status, "sign_in_required", "Sign in with a current workspace membership.")
		return
	}
	if h.directory == nil {
		webError(w, 503, "service_unavailable", "Private directory unavailable.")
		return
	}
	if len(r.URL.RawQuery) > 1024 {
		webError(w, 400, "invalid_query", "Invalid private directory query.")
		return
	}
	v, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		webError(w, 400, "invalid_query", "Invalid private directory query.")
		return
	}
	if r.Method == "GET" {
		if !webQueryKeys(v, "cursor") {
			webError(w, 400, "invalid_query", "Use only a private page cursor.")
			return
		}
		binding := h.binding(p, "private-directory", "")
		after, err := h.openCursor(v.Get("cursor"), binding)
		if err != nil || (after.ID != "" && !directoryUUID(after.ID)) {
			webError(w, 400, "invalid_cursor", "Start a new private directory page.")
			return
		}
		items, err := h.directory.list(ctx, p, after.ID)
		if err != nil {
			webError(w, 503, "service_unavailable", "Private directory unavailable.")
			return
		}
		if items == nil {
			items = []privateDirectoryEntry{}
		}
		next := ""
		if len(items) > 24 {
			items = items[:24]
			next = h.sealCursor(webPosition{Kind: "tool", ID: items[len(items)-1].ID, Version: "1"}, binding)
		}
		max := h.maxBytes
		if max <= 0 {
			max = 256 << 10
		}
		hadItems := len(items) > 0
		for len(items) > 0 {
			raw, _ := json.Marshal(map[string]any{"items": items, "next_cursor": next})
			if len(raw) <= max {
				break
			}
			items = items[:len(items)-1]
			if len(items) > 0 {
				next = h.sealCursor(webPosition{Kind: "tool", ID: items[len(items)-1].ID, Version: "1"}, binding)
			}
		}
		if hadItems && len(items) == 0 {
			webError(w, 503, "response_too_large", "This saved descriptor exceeds the display limit.")
			return
		}
		h.write(w, map[string]any{"items": items, "next_cursor": next})
		return
	}
	token := h.csrfToken(r)
	if r.Header.Get("Origin") != h.origin || token == "" || !hmac.Equal([]byte(token), []byte(r.Header.Get("X-Gist-CSRF"))) {
		webError(w, 403, "csrf_denied", "Reload the library before changing private entries.")
		return
	}
	state, err := h.policy.CheckWorkloadForIssuer(ctx, h.origin, p.Subject, h.workspace, []string{"catalog:read", "catalog:publish"}, p.PolicyGeneration)
	if err != nil {
		webError(w, 503, "service_unavailable", "Private directory unavailable.")
		return
	}
	if !state.Allowed || state.PolicyGeneration != p.PolicyGeneration {
		webError(w, 403, "permission_denied", "A current maintainer membership is required.")
		return
	}
	p.Scopes = []string{"catalog:read", "catalog:publish"}
	if r.Method == "DELETE" {
		if !webQueryKeys(v, "id") || !directoryUUID(v.Get("id")) {
			webError(w, 400, "invalid_query", "Choose the saved entry ID.")
			return
		}
		err = h.directory.remove(ctx, p, v.Get("id"))
		if err != nil {
			h.directoryError(w, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	if len(v) != 0 {
		webError(w, 400, "invalid_query", "Add entries without query parameters.")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		webError(w, 415, "invalid_content_type", "Use application/json.")
		return
	}
	var input struct {
		Record directory.Record `json:"record"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF {
		webError(w, 400, "invalid_record", "Provide a bounded directory descriptor without secrets.")
		return
	}
	record := input.Record
	record.Source = directory.RecordSource{ID: "custom", EntryID: record.ID}
	record.Verification = nil
	for i := range record.Skills {
		record.Skills[i].Status = "candidate"
		record.Skills[i].SourceDigest = ""
	}
	record.Recommended = false
	record.Official = false
	record.OfficialEvidenceURL = ""
	record.FeaturedSelection = false
	record.Featured = false
	if directory.ValidatePrivateRecord(record) != nil {
		webError(w, 400, "invalid_record", "Check the descriptor fields. Credentials and executable arguments are not accepted.")
		return
	}
	recordBytes, _ := json.Marshal(record)
	if len(recordBytes)+256 > h.maxBytes {
		webError(w, 400, "record_too_large", "Descriptor exceeds this library display limit.")
		return
	}
	item, err := h.directory.create(ctx, p, record)
	if err != nil {
		h.directoryError(w, err)
		return
	}
	raw, _ := json.Marshal(item)
	if len(raw) > h.maxBytes {
		webError(w, 503, "response_too_large", "Saved descriptor exceeds display limit.")
		return
	}
	w.WriteHeader(201)
	_, _ = w.Write(raw)
}
func (h *webAPI) directoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errDirectoryDenied):
		webError(w, 403, "permission_denied", "Current maintainer membership required.")
	case errors.Is(err, pgx.ErrNoRows):
		webError(w, 404, "not_found", "Saved entry not found.")
	case errors.Is(err, errDirectoryFull):
		webError(w, 409, "directory_full", "Private directory limit reached.")
	default:
		webError(w, 503, "service_unavailable", "Private directory unavailable.")
	}
}
func directoryUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

type postgresPrivateDirectory struct{ pool *pgxpool.Pool }

const directoryWriteAuthority = webAuthority + ` AND EXISTS(SELECT 1 FROM workspace_memberships wm WHERE wm.workspace_id=$1 AND wm.issuer=$2 AND wm.subject=$3 AND wm.active AND wm.policy_generation=$4 AND wm.role='maintainer' AND 'catalog:publish'=ANY(wm.scopes))`

func (s postgresPrivateDirectory) list(ctx context.Context, p ports.Principal, after string) ([]privateDirectoryEntry, error) {
	items := []privateDirectoryEntry{}
	err := storage.WithTenant(ctx, s.pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text,record,created_at FROM private_directory_entries WHERE workspace_id=$1 AND `+webAuthority+` AND ($5='' OR id::text>$5) ORDER BY id LIMIT 25`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration, after)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item privateDirectoryEntry
			var raw []byte
			if err := rows.Scan(&item.ID, &raw, &item.CreatedAt); err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &item.Record); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}
func (s postgresPrivateDirectory) create(ctx context.Context, p ports.Principal, record directory.Record) (privateDirectoryEntry, error) {
	var out privateDirectoryEntry
	raw, err := json.Marshal(record)
	if err != nil {
		return out, err
	}
	if len(raw) > 64<<10 {
		return out, errDirectoryFull
	}
	err = storage.WithTenant(ctx, s.pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		// Serialize the bounded count without granting UPDATE on workspace metadata.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "gist-private-directory:"+p.WorkspaceID); err != nil {
			return err
		}
		var authorized bool
		if err := tx.QueryRow(ctx, `SELECT `+directoryWriteAuthority+` FROM workspaces WHERE id=$1`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration).Scan(&authorized); err != nil {
			return err
		}
		if !authorized {
			return errDirectoryDenied
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM private_directory_entries WHERE workspace_id=$1`, p.WorkspaceID).Scan(&count); err != nil {
			return err
		}
		if count >= 1000 {
			return errDirectoryFull
		}
		return tx.QueryRow(ctx, `INSERT INTO private_directory_entries(workspace_id,record,created_by) SELECT $1,$5::jsonb,$3 WHERE `+directoryWriteAuthority+` RETURNING id::text,created_at`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration, raw).Scan(&out.ID, &out.CreatedAt)
	})
	out.Record = record
	return out, err
}
func (s postgresPrivateDirectory) remove(ctx context.Context, p ports.Principal, id string) error {
	return storage.WithTenant(ctx, s.pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM private_directory_entries WHERE workspace_id=$1 AND `+directoryWriteAuthority+` AND id=$5::uuid`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		return nil
	})
}
