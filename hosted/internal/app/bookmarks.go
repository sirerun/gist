package app

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

const maxBookmarks = 200

var errBookmarksFull = errors.New("bookmark limit reached")

type bookmarkStore interface {
	list(context.Context, ports.Principal) ([]string, error)
	set(context.Context, ports.Principal, string, bool) error
}

func (h *webAPI) serveBookmarks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "PUT" && r.Method != "DELETE" {
		w.Header().Set("Allow", "GET, PUT, DELETE")
		webError(w, 405, "method_not_allowed", "Use GET, PUT or DELETE.")
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
		if status == 401 {
			webError(w, status, "sign_in_required", "Sign in to use your bookmarks.")
		} else {
			webError(w, status, "unavailable", "Bookmarks unavailable.")
		}
		return
	}
	if h.bookmarks == nil || h.publicIndex == nil {
		webError(w, 503, "unavailable", "Bookmarks unavailable.")
		return
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
	if r.Method != "GET" {
		token := h.csrfToken(r)
		if r.Header.Get("Origin") != h.origin || token == "" || !hmac.Equal([]byte(token), []byte(r.Header.Get("X-Gist-CSRF"))) {
			webError(w, 403, "csrf_denied", "Reload before changing bookmarks.")
			return
		}
		id := v.Get("id")
		if !webQueryKeys(v, "id") || !webRefValue(id, 512) {
			webError(w, 400, "invalid_query", "Choose an entry ID.")
			return
		}
		if r.Method == "PUT" {
			if _, ok := h.publicIndex.Get(id); !ok {
				webError(w, 404, "not_found", "Entry not found.")
				return
			}
		}
		if err = h.bookmarks.set(ctx, p, id, r.Method == "PUT"); err != nil {
			if errors.Is(err, errDirectoryDenied) {
				webError(w, 403, "permission_denied", "Your workspace access changed. Reload before trying again.")
			} else if errors.Is(err, errBookmarksFull) {
				webError(w, 409, "bookmark_limit", "You can save up to 200 bookmarks.")
			} else {
				webError(w, 503, "unavailable", "Could not update bookmarks.")
			}
			return
		}
		w.WriteHeader(204)
		return
	}
	if !webQueryKeys(v, "view", "q", "kind", "tag", "offset") || len(v.Get("q")) > 200 || len(v.Get("tag")) > 64 || (v.Get("kind") != "" && v.Get("kind") != "mcp" && v.Get("kind") != "openapi") || (v.Get("view") != "" && v.Get("view") != "entries") {
		webError(w, 400, "invalid_query", "Invalid bookmark query.")
		return
	}
	queryTerms := bookmarkTerms(v.Get("q"))
	if len(queryTerms) > 16 {
		webError(w, 400, "invalid_query", "Use at most 16 search terms.")
		return
	}
	ids, err := h.bookmarks.list(ctx, p)
	if err != nil {
		webError(w, 503, "unavailable", "Could not load bookmarks.")
		return
	}
	if ids == nil {
		ids = []string{}
	}
	if v.Get("view") != "entries" {
		h.write(w, map[string]any{"ids": ids})
		return
	}
	offset := 0
	if v.Has("offset") {
		offset, err = strconv.Atoi(v.Get("offset"))
		if err != nil || offset < 0 || offset > maxBookmarks {
			webError(w, 400, "invalid_query", "Invalid page.")
			return
		}
	}
	items := []directory.PublicRecord{}
	tagFilter := strings.ToLower(strings.TrimSpace(v.Get("tag")))
	for _, id := range ids {
		record, ok := h.publicIndex.Get(id)
		if !ok {
			record = directory.Record{ID: id, Title: id, Kind: "unavailable", Description: "This entry is no longer in the public catalog. You can remove its bookmark."}
		}
		if kind := v.Get("kind"); kind != "" && record.Kind != kind {
			continue
		}
		if !bookmarkTextMatches(record, queryTerms) {
			continue
		}
		if tag := tagFilter; tag != "" {
			found := false
			for _, t := range record.Tags {
				if t == tag {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		items = append(items, directory.PublicRecordOf(record))
	}
	total := len(items)
	if offset > total {
		offset = total
	}
	end := min(offset+24, total)
	// Keep a saved page within the same response budget as workspace reads.
	for end > offset {
		raw, _ := json.Marshal(map[string]any{"items": items[offset:end], "total": total, "next_cursor": strconv.Itoa(end)})
		if len(raw) <= h.maxBytes {
			break
		}
		end--
	}
	if end == offset && total > offset {
		webError(w, 503, "response_too_large", "This entry exceeds the library display limit.")
		return
	}
	next := ""
	if end < total {
		next = strconv.Itoa(end)
	}
	h.write(w, map[string]any{"items": items[offset:end], "total": total, "next_cursor": next})
}

// Match the directory index's Unicode word-prefix AND search semantics.
func bookmarkTerms(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsNumber(c) })
}
func bookmarkTextMatches(record directory.Record, terms []string) bool {
	words := bookmarkTerms(record.Title + " " + record.ID + " " + record.Description + " " + record.ServiceID + " " + strings.Join(record.Tags, " "))
	for _, term := range terms {
		found := false
		for _, word := range words {
			if strings.HasPrefix(word, term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type postgresBookmarks struct{ pool *pgxpool.Pool }

func (s postgresBookmarks) list(ctx context.Context, p ports.Principal) ([]string, error) {
	ids := []string{}
	err := storage.WithTenant(ctx, s.pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT entry_id FROM directory_bookmarks WHERE workspace_id=$1 AND issuer=$2 AND subject=$3 AND `+webAuthority+` ORDER BY created_at DESC,entry_id LIMIT 200`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}
func (s postgresBookmarks) set(ctx context.Context, p ports.Principal, id string, saved bool) error {
	return storage.WithTenant(ctx, s.pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		// Serialize this user's bounded collection, including idempotent saves.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, p.WorkspaceID+"/"+p.Issuer+"/"+p.Subject); err != nil {
			return err
		}
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT `+webAuthority+` FROM workspaces WHERE id=$1`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return errDirectoryDenied
		}
		if !saved {
			_, err := tx.Exec(ctx, `DELETE FROM directory_bookmarks WHERE workspace_id=$1 AND issuer=$2 AND subject=$3 AND entry_id=$5 AND `+webAuthority, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration, id)
			return err
		}
		var count int
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(bool_or(entry_id=$4),false) FROM directory_bookmarks WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, p.WorkspaceID, p.Issuer, p.Subject, id).Scan(&count, &exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		if count >= maxBookmarks {
			return errBookmarksFull
		}
		_, err := tx.Exec(ctx, `INSERT INTO directory_bookmarks(workspace_id,issuer,subject,entry_id) SELECT $1,$2,$3,$5 WHERE `+webAuthority+` ON CONFLICT DO NOTHING`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration, id)
		return err
	})
}
