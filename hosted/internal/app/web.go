package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/oauth"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

const webPageSize = 24

type webPolicy interface {
	CheckWorkloadForIssuer(context.Context, string, string, string, []string, uint64) (identity.Policy, error)
	Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error)
}
type webCatalog interface {
	list(context.Context, ports.Principal, string, string, webPosition) ([]webItem, error)
	get(context.Context, ports.Principal, string, string, string) (webItem, error)
}
type webItem struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id"`
	Version     string         `json:"version"`
	State       string         `json:"state"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Digest      webDigest      `json:"digest"`
	Metadata    map[string]any `json:"metadata"`
}
type webDigest struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}
type webPosition struct {
	Kind    string `json:"k"`
	ID      string `json:"i"`
	Version string `json:"v"`
}
type webCursor struct {
	Position webPosition `json:"p"`
	Binding  string      `json:"b"`
	Expires  int64       `json:"e"`
}
type webAPI struct {
	sessions                                   oauth.SessionAuthenticator
	policy                                     webPolicy
	catalog                                    webCatalog
	bookmarks                                  bookmarkStore
	publicIndex                                *directory.Index
	directory                                  privateDirectoryStore
	origin, audience, workspace, workspaceName string
	cursorKey                                  []byte
	csrfKey                                    []byte
	timeout                                    time.Duration
	maxBytes                                   int
	concurrent                                 chan struct{}
}

func (h *webAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Vary", "Cookie")
	// Browser requests are same-origin only, including credentialed reads. API
	// clients may omit Origin; cross-origin browser fetches cannot read these APIs.
	if origin := r.Header.Get("Origin"); origin != "" && origin != h.origin {
		webError(w, http.StatusForbidden, "origin_denied", "Open Gist in its original tab.")
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		webError(w, http.StatusForbidden, "origin_denied", "Open Gist in its original tab.")
		return
	}
	if r.URL.Path == "/app/api/logout" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			webError(w, 405, "method_not_allowed", "Use POST.")
			return
		}
		if r.Header.Get("Origin") != h.origin {
			webError(w, 403, "origin_denied", "Open Gist in its original tab.")
			return
		}
		if r.URL.RawQuery != "" {
			webError(w, 400, "invalid_query", "The sign-out URL does not accept parameters.")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: oauth.SessionCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path == "/app/api/bookmarks" {
		h.serveBookmarks(w, r)
		return
	}
	if r.URL.Path == "/app/api/directory" {
		h.servePrivateDirectory(w, r)
		return
	}
	if r.URL.Path != "/app/api/session" && r.URL.Path != "/app/api/catalog" && r.URL.Path != "/app/api/artifact" {
		webError(w, 404, "not_found", "Page not found.")
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		webError(w, 405, "method_not_allowed", "Use GET.")
		return
	}
	if h.concurrent != nil {
		select {
		case h.concurrent <- struct{}{}:
			defer func() { <-h.concurrent }()
		default:
			w.Header().Set("Retry-After", "1")
			webError(w, 429, "rate_limited", "The library is busy. Try again shortly.")
			return
		}
	}
	if len(r.URL.RawQuery) > 12<<10 {
		webError(w, 400, "invalid_query", "The library URL is too long.")
		return
	}
	timeout := h.timeout
	if timeout <= 0 || timeout > 8*time.Second {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	r = r.WithContext(ctx)
	p, status := h.principal(r)
	if status != http.StatusOK {
		if status == 503 {
			webError(w, status, "service_unavailable", "The library is temporarily unavailable. Try again shortly.")
		} else {
			webError(w, status, "sign_in_required", "Sign in with an invited GitHub account to open the library.")
		}
		return
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		webError(w, 400, "invalid_query", "Check the library URL and try again.")
		return
	}
	switch r.URL.Path {
	case "/app/api/session":
		if len(values) != 0 {
			webError(w, 400, "invalid_query", "The session URL does not accept parameters.")
			return
		}
		h.write(w, map[string]any{"authenticated": true, "workspace_id": h.workspace, "workspace_name": h.workspaceName, "subject": p.Subject, "csrf_token": h.csrfToken(r)})
	case "/app/api/catalog":
		if !webQueryKeys(values, "q", "kind", "cursor") {
			webError(w, 400, "invalid_query", "Use one search, kind, and cursor parameter.")
			return
		}
		q, kind := strings.TrimSpace(values.Get("q")), values.Get("kind")
		if len(q) > 200 || !utf8.ValidString(q) || strings.ContainsRune(q, 0) || kind != "" && !webKind(kind) {
			webError(w, 400, "invalid_query", "Use a search of up to 200 bytes and a supported item kind.")
			return
		}
		binding := h.binding(p, q, kind)
		after, err := h.openCursor(values.Get("cursor"), binding)
		if err != nil {
			webError(w, 400, "invalid_cursor", "This page link expired. Start a new search.")
			return
		}
		items, err := h.catalog.list(ctx, p, q, kind, after)
		if err != nil {
			webError(w, 503, "service_unavailable", "The library is temporarily unavailable. Try again shortly.")
			return
		}
		next := ""
		if len(items) > webPageSize {
			items = items[:webPageSize]
			last := items[len(items)-1]
			next = h.sealCursor(webPosition{last.Kind, last.ID, last.Version}, binding)
		}
		if items == nil {
			items = []webItem{}
		}
		h.write(w, map[string]any{"items": items, "next_cursor": next})
	case "/app/api/artifact":
		if !webQueryKeys(values, "kind", "id", "version") || !webKind(values.Get("kind")) || !webRefValue(values.Get("id"), 2048) || !webRefValue(values.Get("version"), 256) {
			webError(w, 400, "invalid_query", "Choose an exact item kind, ID, and version.")
			return
		}
		item, err := h.catalog.get(ctx, p, values.Get("kind"), values.Get("id"), values.Get("version"))
		if errors.Is(err, pgx.ErrNoRows) {
			webError(w, 404, "not_found", "This version is unavailable in your library.")
			return
		}
		if err != nil {
			webError(w, 503, "service_unavailable", "The library is temporarily unavailable. Try again shortly.")
			return
		}
		h.write(w, item)
	}
}

func (h *webAPI) principal(r *http.Request) (ports.Principal, int) {
	session, err := h.sessions.Authenticate(r)
	if err != nil || session.Subject == "" {
		return ports.Principal{}, 401
	}
	member := false
	for _, ws := range session.Workspaces {
		if ws == h.workspace {
			member = true
		}
	}
	if !member {
		return ports.Principal{}, 401
	}
	state, err := h.policy.CheckWorkloadForIssuer(r.Context(), h.origin, session.Subject, h.workspace, []string{"catalog:read"}, 0)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Principal{}, 401
	}
	if err != nil {
		return ports.Principal{}, 503
	}
	if !state.Allowed {
		return ports.Principal{}, 401
	}
	p := ports.Principal{Issuer: h.origin, Audience: h.audience, Subject: session.Subject, SubjectType: "human", WorkspaceID: h.workspace, Scopes: []string{"catalog:read"}, PolicyGeneration: state.PolicyGeneration}
	decision, err := h.policy.Decide(r.Context(), p, ports.ActionRead, nil)
	if err != nil {
		return ports.Principal{}, 503
	}
	if !decision.Allowed {
		return ports.Principal{}, 401
	}
	return p, 200
}
func webQueryKeys(v url.Values, keys ...string) bool {
	for key, values := range v {
		allowed := false
		for _, k := range keys {
			if key == k {
				allowed = true
			}
		}
		if !allowed || len(values) != 1 {
			return false
		}
	}
	return true
}
func webKind(k string) bool {
	switch k {
	case "skill", "tool", "capability", "provider", "binding", "taxonomy":
		return true
	}
	return false
}
func webRefValue(s string, max int) bool {
	return s != "" && len(s) <= max && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func (h *webAPI) binding(p ports.Principal, q, kind string) string {
	data, _ := json.Marshal([]any{p.Issuer, p.Subject, p.WorkspaceID, p.PolicyGeneration, q, kind})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (h *webAPI) sealCursor(p webPosition, binding string) string {
	data, _ := json.Marshal(webCursor{p, binding, time.Now().Add(15 * time.Minute).Unix()})
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, h.cursorKey)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (h *webAPI) openCursor(raw, binding string) (webPosition, error) {
	if raw == "" {
		return webPosition{}, nil
	}
	if len(raw) > 8192 {
		return webPosition{}, errors.New("oversized cursor")
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return webPosition{}, errors.New("invalid cursor")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return webPosition{}, err
	}
	mac := hmac.New(sha256.New, h.cursorKey)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return webPosition{}, errors.New("invalid cursor signature")
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return webPosition{}, err
	}
	var c webCursor
	if err = json.Unmarshal(rawPayload, &c); err != nil {
		return webPosition{}, err
	}
	if c.Binding != binding || c.Expires <= time.Now().Unix() || !webKind(c.Position.Kind) || !webRefValue(c.Position.ID, 2048) || !webRefValue(c.Position.Version, 256) {
		return webPosition{}, errors.New("invalid cursor scope")
	}
	return c.Position, nil
}
func (h *webAPI) write(w http.ResponseWriter, value any) {
	data, err := json.Marshal(value)
	if err != nil || len(data) > h.maxBytes {
		webError(w, 503, "service_unavailable", "This response exceeds the library limit. Refine your search.")
		return
	}
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func webError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

type postgresWebCatalog struct{ pool *pgxpool.Pool }

// The EXISTS predicate rechecks durable authority in the catalog statement too,
// so a revocation between session validation and the read cannot expose rows.
const webAuthority = `EXISTS (SELECT 1 FROM workspace_memberships wm JOIN workspaces w ON w.id=wm.workspace_id WHERE wm.workspace_id=$1 AND wm.issuer=$2 AND wm.subject=$3 AND wm.active AND wm.policy_generation=$4 AND w.policy_generation=$4 AND 'catalog:read'=ANY(wm.scopes))`
const webColumns = `kind,artifact_id,version,state,digest_algorithm,digest_value,LEFT(COALESCE(NULLIF(metadata->>'title',''),NULLIF(metadata->>'name',''),NULLIF(metadata->>'logical_name',''),artifact_id),512),LEFT(COALESCE(metadata->>'description',metadata->>'summary',''),4096)`

func (s postgresWebCatalog) list(ctx context.Context, p ports.Principal, q, kind string, after webPosition) ([]webItem, error) {
	var items []webItem
	err := storage.WithTenant(ctx, s.pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+webColumns+` FROM catalog_versions WHERE workspace_id=$1 AND `+webAuthority+` AND state IN ('published','deprecated') AND kind IN ('skill','tool','capability','provider','binding','taxonomy') AND ($5='' OR kind=$5) AND ($6='' OR strpos(lower(artifact_id || ' ' || COALESCE(metadata->>'title','') || ' ' || COALESCE(metadata->>'name','') || ' ' || COALESCE(metadata->>'logical_name','') || ' ' || COALESCE(metadata->>'description','') || ' ' || COALESCE(metadata->>'summary','')),lower($6))>0) AND (kind,artifact_id,version)>($7,$8,$9) ORDER BY kind,artifact_id,version LIMIT 25`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration, kind, q, after.Kind, after.ID, after.Version)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanWebItem(rows, false)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}
func (s postgresWebCatalog) get(ctx context.Context, p ports.Principal, kind, id, version string) (webItem, error) {
	var item webItem
	err := storage.WithTenant(ctx, s.pool, p.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		item, err = scanWebItem(tx.QueryRow(ctx, `SELECT `+webDetailColumns+` FROM catalog_versions WHERE workspace_id=$1 AND `+webAuthority+` AND state IN ('published','deprecated') AND kind=$5 AND artifact_id=$6 AND version=$7`, p.WorkspaceID, p.Issuer, p.Subject, p.PolicyGeneration, kind, id, version), true)
		return err
	})
	return item, err
}

// The detail projection decodes only a bounded typed document, never arbitrary
// metadata keys or credential, connection, object-storage, or schema bodies.
const webDetailColumns = webColumns + `,CASE WHEN octet_length(metadata::text)<=65536 THEN metadata::text ELSE NULL END`

func scanWebItem(row interface{ Scan(...any) error }, detail bool) (webItem, error) {
	var item webItem
	var metadata []byte
	columns := []any{&item.Kind, &item.ID, &item.Version, &item.State, &item.Digest.Algorithm, &item.Digest.Value, &item.Title, &item.Description}
	if detail {
		columns = append(columns, &metadata)
	}
	err := row.Scan(columns...)
	// Deliberately project display metadata; storage references, original manifests,
	// connection configuration and arbitrary private metadata never enter the UI.
	item.Metadata = map[string]any{"name": item.Title, "description": item.Description}
	if detail && err == nil {
		projectWebDeclarations(item.Metadata, metadata)
	}
	return item, err
}

// webDeclarations mirrors displayable declarations in the published manifest,
// capability, execution schema and provider contracts. These are publisher
// assertions, not evidence that a client can execute or has connected an account.
type webDeclarations struct {
	Publisher struct {
		ID       string `json:"id"`
		Revision string `json:"source_revision"`
		License  string `json:"license"`
	} `json:"publisher"`
	Publication struct {
		Status     string `json:"status"`
		Provenance string `json:"provenance"`
	} `json:"publication"`
	Trust                string           `json:"trust"`
	Tags                 []string         `json:"tags"`
	RequiredCapabilities []webRequirement `json:"required_capabilities"`
	Runtime              struct {
		LocalExecution *bool    `json:"local_execution"`
		Filesystem     []string `json:"filesystem"`
	} `json:"runtime_requirements"`
	Effects           []string `json:"effects"`
	RequiredScopes    []string `json:"required_scopes"`
	ExecutionLocation string   `json:"execution_location"`
	SupportState      string   `json:"support_state"`
	LastVerifiedAt    string   `json:"last_verified_at"`
}
type webRequirement struct {
	ID       string `json:"id"`
	Version  string `json:"contract_version"`
	Optional bool   `json:"optional"`
}

func projectWebDeclarations(out map[string]any, raw []byte) {
	var declared webDeclarations
	if len(raw) == 0 || len(raw) > 64<<10 || json.Unmarshal(raw, &declared) != nil {
		out["declarations_unavailable"] = true
		return
	}
	abbreviated := false
	text := func(value string, limit int) string {
		runes := []rune(value)
		if len(runes) > limit {
			abbreviated = true
			return string(runes[:limit]) + "…"
		}
		return value
	}
	stringsList := func(values []string) []string {
		if len(values) > 32 {
			abbreviated = true
			values = values[:32]
		}
		out := make([]string, 0, len(values))
		for _, value := range values {
			if value != "" {
				out = append(out, text(value, 256))
			}
		}
		return out
	}
	if declared.Publisher.ID != "" {
		publisher := map[string]string{"id": text(declared.Publisher.ID, 256)}
		if declared.Publisher.Revision != "" {
			publisher["source_revision"] = text(declared.Publisher.Revision, 256)
		}
		if declared.Publisher.License != "" {
			publisher["license"] = text(declared.Publisher.License, 256)
		}
		out["publisher"] = publisher
	}
	if declared.Publication.Status != "" || declared.Publication.Provenance != "" {
		out["publication"] = map[string]string{"status": text(declared.Publication.Status, 64), "provenance": text(declared.Publication.Provenance, 512)}
	}
	if declared.Trust != "" {
		out["trust"] = text(declared.Trust, 64)
	}
	if len(declared.Tags) > 0 {
		out["tags"] = stringsList(declared.Tags)
	}
	if len(declared.RequiredCapabilities) > 0 {
		reqs := declared.RequiredCapabilities
		if len(reqs) > 64 {
			abbreviated = true
			reqs = reqs[:64]
		}
		safe := make([]webRequirement, 0, len(reqs))
		for _, req := range reqs {
			if req.ID != "" && req.Version != "" {
				safe = append(safe, webRequirement{ID: text(req.ID, 256), Version: text(req.Version, 128), Optional: req.Optional})
			}
		}
		out["required_capabilities"] = safe
	}
	if declared.Runtime.LocalExecution != nil || len(declared.Runtime.Filesystem) > 0 {
		runtime := map[string]any{"filesystem_required": len(declared.Runtime.Filesystem) > 0}
		if declared.Runtime.LocalExecution != nil {
			runtime["local_execution"] = *declared.Runtime.LocalExecution
		}
		out["runtime_requirements"] = runtime
	}
	if len(declared.Effects) > 0 {
		out["effects"] = stringsList(declared.Effects)
	}
	if len(declared.RequiredScopes) > 0 {
		out["required_scopes"] = stringsList(declared.RequiredScopes)
	}
	if declared.ExecutionLocation != "" {
		out["execution_location"] = text(declared.ExecutionLocation, 64)
	}
	if declared.SupportState != "" {
		out["support_state"] = text(declared.SupportState, 64)
	}
	if declared.LastVerifiedAt != "" {
		out["last_verified_at"] = text(declared.LastVerifiedAt, 64)
	}
	if abbreviated {
		out["declarations_abbreviated"] = true
	}
}
