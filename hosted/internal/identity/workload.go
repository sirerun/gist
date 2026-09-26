package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type Clock interface{ Now() time.Time }

type Config struct {
	Issuer      string
	Audience    string
	MaxTTL      time.Duration
	Clock       Clock
	Keys        *KeySet
	Policy      PolicyStore
	Revocations *RevocationLease
}

type WorkloadIssuer struct{ cfg Config }

type WorkloadRequest struct {
	Subject       string
	WorkspaceID   string
	Scopes        []string
	ParentSubject string
	ParentScopes  []string
	TTL           time.Duration
}

type claims struct {
	Issuer        string   `json:"iss"`
	Subject       string   `json:"sub"`
	Audience      string   `json:"aud"`
	Workspace     string   `json:"workspace_id"`
	Scopes        []string `json:"scopes"`
	Generation    uint64   `json:"policy_generation"`
	IssuedAt      int64    `json:"iat"`
	ExpiresAt     int64    `json:"exp"`
	JTI           string   `json:"jti"`
	ParentSubject string   `json:"parent_sub,omitempty"`
	ParentScopes  []string `json:"parent_scopes,omitempty"`
}

type VerifiedToken struct {
	Principal     ports.Principal
	JTI           string
	ExpiresAt     time.Time
	ParentSubject string
	ParentScopes  []string
}

func NewWorkloadIssuer(cfg Config) (*WorkloadIssuer, error) {
	if cfg.Issuer == "" || cfg.Audience == "" || cfg.Clock == nil || cfg.Keys == nil || cfg.Policy == nil {
		return nil, fmt.Errorf("issuer, audience, clock, keys, and policy are required")
	}
	if cfg.MaxTTL <= 0 || cfg.MaxTTL > defaultMaxTokenAge {
		cfg.MaxTTL = defaultMaxTokenAge
	}
	return &WorkloadIssuer{cfg: cfg}, nil
}

func (i *WorkloadIssuer) Mint(ctx context.Context, req WorkloadRequest) (string, error) {
	if req.Subject == "" || req.WorkspaceID == "" || len(req.Scopes) == 0 {
		return "", fmt.Errorf("subject, workspace, and scopes are required: %w", ErrUnauthorized)
	}
	if req.ParentSubject == "" {
		req.ParentSubject = req.Subject
	}
	if len(req.ParentScopes) == 0 {
		req.ParentScopes = append([]string(nil), req.Scopes...)
	}
	if !containsAll(req.ParentScopes, req.Scopes) {
		return "", fmt.Errorf("workload scopes exceed parent authority: %w", ErrUnauthorized)
	}
	policy, err := i.cfg.Policy.CheckWorkload(ctx, req.Subject, req.WorkspaceID, req.Scopes, 0)
	if err != nil {
		return "", fmt.Errorf("check workload membership: %w", err)
	}
	if !policy.Allowed || policy.Revoked || !containsAll(req.ParentScopes, req.Scopes) {
		return "", ErrUnauthorized
	}
	now := i.cfg.Clock.Now().UTC()
	ttl := req.TTL
	if ttl <= 0 || ttl > i.cfg.MaxTTL {
		ttl = i.cfg.MaxTTL
	}
	key, err := i.cfg.Keys.currentKey()
	if err != nil {
		return "", fmt.Errorf("load workload signing key: %w", err)
	}
	c := claims{Issuer: i.cfg.Issuer, Subject: req.Subject, Audience: i.cfg.Audience, Workspace: req.WorkspaceID, Scopes: normalizeScopes(req.Scopes), Generation: policy.PolicyGeneration, IssuedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(), JTI: randomJTI(now), ParentSubject: req.ParentSubject, ParentScopes: normalizeScopes(req.ParentScopes)}
	return signJWT(key, c)
}

func (i *WorkloadIssuer) Verify(ctx context.Context, token string) (VerifiedToken, error) {
	header, body, signature, err := splitJWT(token)
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("parse bearer token: %w", ErrUnauthorized)
	}
	if header.Algorithm != "EdDSA" || header.KID == "" {
		return VerifiedToken{}, fmt.Errorf("header: %w", ErrUnauthorized)
	}
	now := i.cfg.Clock.Now().UTC()
	key, err := i.cfg.Keys.verifyKey(header.KID, now)
	if err != nil || !ed25519.Verify(key.Public, []byte(body), signature) {
		return VerifiedToken{}, fmt.Errorf("signature: %w", ErrUnauthorized)
	}
	var c claims
	payload, err := decodeString(body[strings.Index(body, ".")+1:])
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("claims encoding: %w", ErrUnauthorized)
	}
	if err := json.Unmarshal([]byte(payload), &c); err != nil {
		return VerifiedToken{}, fmt.Errorf("claims json: %w", ErrUnauthorized)
	}
	if c.Issuer != i.cfg.Issuer || c.Audience != i.cfg.Audience || c.Subject == "" || c.Workspace == "" || c.JTI == "" || c.Generation == 0 {
		return VerifiedToken{}, fmt.Errorf("claims values: %#v: %w", c, ErrUnauthorized)
	}
	if !containsAll(c.ParentScopes, c.Scopes) || c.ParentSubject == "" {
		return VerifiedToken{}, fmt.Errorf("parent: %#v: %w", c, ErrUnauthorized)
	}
	if i.cfg.Revocations != nil && i.cfg.Revocations.IsRevoked(c.JTI, now) {
		return VerifiedToken{}, ErrReplay
	}
	if now.Add(maxClockSkew).Unix() < c.IssuedAt || now.Add(-maxClockSkew).Unix() > c.ExpiresAt || c.ExpiresAt-c.IssuedAt > int64(i.cfg.MaxTTL/time.Second) {
		return VerifiedToken{}, ErrExpired
	}
	policy, err := i.cfg.Policy.CheckWorkload(ctx, c.Subject, c.Workspace, c.Scopes, c.Generation)
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("check current workload policy: %w", err)
	}
	if !policy.Allowed || policy.Revoked || policy.PolicyGeneration != c.Generation {
		return VerifiedToken{}, ErrRevoked
	}
	return VerifiedToken{Principal: ports.Principal{Issuer: c.Issuer, Subject: c.Subject, Audience: c.Audience, WorkspaceID: c.Workspace, Scopes: normalizeScopes(c.Scopes), PolicyGeneration: c.Generation, SubjectType: "workload"}, JTI: c.JTI, ExpiresAt: time.Unix(c.ExpiresAt, 0).UTC(), ParentSubject: c.ParentSubject, ParentScopes: normalizeScopes(c.ParentScopes)}, nil
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KID       string `json:"kid"`
}

func signJWT(key SigningKey, c claims) (string, error) {
	h, _ := json.Marshal(jwtHeader{Algorithm: "EdDSA", Type: "JWT", KID: key.KID})
	b, _ := json.Marshal(c)
	head := b64(h) + "." + b64(b)
	return head + "." + b64(ed25519.Sign(key.Private, []byte(head))), nil
}
func splitJWT(token string) (jwtHeader, string, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, "", nil, ErrUnauthorized
	}
	hb, e := decode(parts[0])
	if e != nil {
		return jwtHeader{}, "", nil, e
	}
	_, e = decodeString(parts[1])
	if e != nil {
		return jwtHeader{}, "", nil, e
	}
	sig, e := decode(parts[2])
	if e != nil {
		return jwtHeader{}, "", nil, e
	}
	var h jwtHeader
	if json.Unmarshal(hb, &h) != nil {
		return h, "", nil, ErrUnauthorized
	}
	return h, parts[0] + "." + parts[1], sig, nil
}
func decode(v string) ([]byte, error)       { return base64.RawURLEncoding.DecodeString(v) }
func decodeString(v string) (string, error) { b, e := decode(v); return string(b), e }
func randomJTI(now time.Time) string {
	b := sha256.Sum256([]byte(fmt.Sprintf("%d", now.UnixNano())))
	return fmt.Sprintf("%x", b[:])
}
