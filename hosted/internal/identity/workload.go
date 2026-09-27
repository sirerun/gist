package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
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
	// SubjectType is "workload" (the default) or "human". OAuth grants made
	// by a person through the reference authorization server use "human".
	SubjectType string
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
	switch req.SubjectType {
	case "":
		req.SubjectType = subjectWorkload
	case subjectWorkload, subjectHuman:
	default:
		return "", fmt.Errorf("unsupported subject type: %w", ErrUnauthorized)
	}
	policy, err := i.cfg.Policy.CheckWorkload(ctx, req.Subject, req.WorkspaceID, req.Scopes, 0)
	if err != nil {
		return "", fmt.Errorf("check workload membership: %w", err)
	}
	if !policy.Allowed || policy.Revoked || len(policy.ParentScopes) == 0 || !containsAll(policy.ParentScopes, req.Scopes) {
		return "", ErrUnauthorized
	}
	// The scope ceiling is the stored policy, never the caller-supplied request.
	req.ParentScopes = append([]string(nil), policy.ParentScopes...)
	now := i.cfg.Clock.Now().UTC().Truncate(time.Second)
	ttl := req.TTL
	if ttl <= 0 || ttl > i.cfg.MaxTTL {
		ttl = i.cfg.MaxTTL
	}
	jti, err := randomJTI()
	if err != nil {
		return "", fmt.Errorf("generate workload token id: %w", err)
	}
	key, err := i.cfg.Keys.currentKey()
	if err != nil {
		return "", fmt.Errorf("load workload signing key: %w", err)
	}
	privateKey, err := jwk.FromRaw(key.Private)
	if err != nil {
		return "", fmt.Errorf("prepare workload signing key: %w", err)
	}
	if err := privateKey.Set(jwk.KeyIDKey, key.KID); err != nil {
		return "", fmt.Errorf("set workload signing key id: %w", err)
	}
	token, err := jwt.NewBuilder().Issuer(i.cfg.Issuer).Subject(req.Subject).
		Audience([]string{i.cfg.Audience}).JwtID(jti).IssuedAt(now).NotBefore(now).
		Expiration(now.Add(ttl)).Claim("workspace_id", req.WorkspaceID).
		Claim("scopes", normalizeScopes(req.Scopes)).Claim("policy_generation", policy.PolicyGeneration).
		Claim("parent_sub", req.ParentSubject).Claim("parent_scopes", normalizeScopes(req.ParentScopes)).
		Claim("subject_type", req.SubjectType).Build()
	if err != nil {
		return "", fmt.Errorf("build workload token: %w", err)
	}
	encoded, err := jwt.Sign(token, jwt.WithKey(jwa.EdDSA, privateKey))
	if err != nil {
		return "", fmt.Errorf("sign workload token: %w", err)
	}
	return string(encoded), nil
}

func (i *WorkloadIssuer) Verify(ctx context.Context, rawToken string) (VerifiedToken, error) {
	if rawToken == "" {
		return VerifiedToken{}, fmt.Errorf("parse bearer token: %w", ErrUnauthorized)
	}
	now := i.cfg.Clock.Now().UTC()
	keyID, err := tokenKeyID(rawToken)
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("parse bearer token: %w", ErrUnauthorized)
	}
	key, err := i.cfg.Keys.verifyKey(keyID, now)
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("signature: %w", ErrUnauthorized)
	}
	publicKey, err := jwk.FromRaw(key.Public)
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("prepare workload verification key: %w", ErrUnauthorized)
	}
	if err := publicKey.Set(jwk.KeyIDKey, key.KID); err != nil {
		return VerifiedToken{}, fmt.Errorf("set workload verification key id: %w", ErrUnauthorized)
	}
	parsed, err := jwt.ParseString(rawToken, jwt.WithKey(jwa.EdDSA, publicKey), jwt.WithValidate(false))
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("verify workload token: %w", ErrUnauthorized)
	}
	validationErr := jwt.Validate(parsed, jwt.WithClock(i.cfg.Clock), jwt.WithAcceptableSkew(maxClockSkew),
		jwt.WithIssuer(i.cfg.Issuer), jwt.WithAudience(i.cfg.Audience),
		jwt.WithRequiredClaim(jwt.IssuerKey), jwt.WithRequiredClaim(jwt.SubjectKey),
		jwt.WithRequiredClaim(jwt.AudienceKey), jwt.WithRequiredClaim(jwt.IssuedAtKey),
		jwt.WithRequiredClaim(jwt.NotBeforeKey), jwt.WithRequiredClaim(jwt.ExpirationKey),
		jwt.WithRequiredClaim(jwt.JwtIDKey))
	if validationErr != nil {
		if errors.Is(validationErr, jwt.ErrTokenExpired()) || errors.Is(validationErr, jwt.ErrTokenNotYetValid()) || errors.Is(validationErr, jwt.ErrInvalidIssuedAt()) {
			return VerifiedToken{}, ErrExpired
		}
		return VerifiedToken{}, fmt.Errorf("claims validation: %w", ErrUnauthorized)
	}
	workspace, scopes, generation, parentSubject, parentScopes, err := privateClaims(parsed)
	if err != nil || parsed.Subject() == "" || parsed.JwtID() == "" || generation == 0 {
		return VerifiedToken{}, fmt.Errorf("claims values: %w", ErrUnauthorized)
	}
	if !containsAll(parentScopes, scopes) || parentSubject == "" {
		return VerifiedToken{}, fmt.Errorf("parent claims: %w", ErrUnauthorized)
	}
	if i.cfg.Revocations != nil && i.cfg.Revocations.IsRevoked(parsed.JwtID(), now) {
		return VerifiedToken{}, ErrReplay
	}
	if parsed.Expiration().Sub(parsed.IssuedAt()) > i.cfg.MaxTTL+maxClockSkew {
		return VerifiedToken{}, ErrExpired
	}
	policy, err := i.cfg.Policy.CheckWorkload(ctx, parsed.Subject(), workspace, scopes, generation)
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("check current workload policy: %w", err)
	}
	if !policy.Allowed || policy.Revoked || policy.PolicyGeneration != generation {
		return VerifiedToken{}, ErrRevoked
	}
	subjectType, err := subjectTypeClaim(parsed)
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("subject type claim: %w", ErrUnauthorized)
	}
	audience := parsed.Audience()
	return VerifiedToken{Principal: ports.Principal{Issuer: parsed.Issuer(), Subject: parsed.Subject(), Audience: audience[0], WorkspaceID: workspace, Scopes: normalizeScopes(scopes), PolicyGeneration: generation, SubjectType: subjectType}, JTI: parsed.JwtID(), ExpiresAt: parsed.Expiration().UTC(), ParentSubject: parentSubject, ParentScopes: normalizeScopes(parentScopes)}, nil
}

const (
	subjectWorkload = "workload"
	subjectHuman    = "human"
)

// subjectTypeClaim reads the optional subject_type claim. Tokens minted
// before the claim existed carry none and are workload tokens.
func subjectTypeClaim(token jwt.Token) (string, error) {
	value, ok := token.Get("subject_type")
	if !ok {
		return subjectWorkload, nil
	}
	s, _ := value.(string)
	if s != subjectWorkload && s != subjectHuman {
		return "", ErrUnauthorized
	}
	return s, nil
}

func randomJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func tokenKeyID(rawToken string) (string, error) {
	message, err := jws.ParseString(rawToken)
	if err != nil || len(message.Signatures()) != 1 {
		return "", ErrUnauthorized
	}
	header := message.Signatures()[0].ProtectedHeaders()
	if header == nil || header.Algorithm() != jwa.EdDSA || header.KeyID() == "" {
		return "", ErrUnauthorized
	}
	return header.KeyID(), nil
}

func privateClaims(token jwt.Token) (string, []string, uint64, string, []string, error) {
	workspace, ok := token.Get("workspace_id")
	workspaceID, okString := workspace.(string)
	if !ok || !okString || workspaceID == "" {
		return "", nil, 0, "", nil, ErrUnauthorized
	}
	scopes, err := claimStrings(token, "scopes")
	if err != nil {
		return "", nil, 0, "", nil, err
	}
	parentScopes, err := claimStrings(token, "parent_scopes")
	if err != nil {
		return "", nil, 0, "", nil, err
	}
	parentValue, ok := token.Get("parent_sub")
	parentSubject, okString := parentValue.(string)
	if !ok || !okString || parentSubject == "" {
		return "", nil, 0, "", nil, ErrUnauthorized
	}
	generationValue, ok := token.Get("policy_generation")
	if !ok {
		return "", nil, 0, "", nil, ErrUnauthorized
	}
	var generation uint64
	switch value := generationValue.(type) {
	case uint64:
		generation = value
	case int64:
		if value >= 0 {
			generation = uint64(value)
		}
	case float64:
		if value >= 0 {
			generation = uint64(value)
		}
	default:
		return "", nil, 0, "", nil, ErrUnauthorized
	}
	return workspaceID, scopes, generation, parentSubject, parentScopes, nil
}

func claimStrings(token jwt.Token, name string) ([]string, error) {
	value, ok := token.Get(name)
	if !ok {
		return nil, ErrUnauthorized
	}
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...), nil
	case []interface{}:
		result := make([]string, len(values))
		for idx, item := range values {
			var ok bool
			result[idx], ok = item.(string)
			if !ok || result[idx] == "" {
				return nil, ErrUnauthorized
			}
		}
		return result, nil
	default:
		return nil, ErrUnauthorized
	}
}
