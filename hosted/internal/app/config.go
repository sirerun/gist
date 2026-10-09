package app

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
)

// Config is the service configuration. Values are deliberately explicit so
// deployment configuration, rather than source defaults, owns the limits.
type Config struct {
	ListenAddress         string
	PublicOrigin          string
	ResourceAudience      string
	DatabaseURL           string
	ObjectStoreRoot       string
	BrokerURL             string
	BrokerClient          *http.Client
	RequestTimeout        time.Duration
	MaxPackageBytes       int64
	MaxExpandedBytes      int64
	MaxRequestBytes       int64
	MaxResponseBytes      int
	MaxCatalogEntries     int
	MaxConcurrentRequests int
	MaxDiscoveryResults   int
	RetryAfter            int
	// OAuthLoginURL is where the reference authorization server sends a
	// person who has no session. Optional; without it authorize answers 401.
	OAuthLoginURL string
	// OAuthConsentSecret keys the reference authorization server's consent
	// and session HMACs (GIST_OAUTH_CONSENT_SECRET). Every replica must share
	// it and it must survive restarts, so it is required configuration of at
	// least MinOAuthConsentSecretBytes bytes; test compositions pass their
	// own explicitly.
	OAuthConsentSecret []byte
	// SigningKeyConfig is operator-supplied strict key-set JSON. Every replica
	// must receive the same custody-approved source; it is never generated.
	SigningKeyConfig []byte
	// Every retained workspace needs an explicitly bound existing maintainer.
	EventMaintenanceTargets []MaintenanceTarget
	// PublicationV2 enables the additive publication API only when all trusted
	// reviewer, verifier and explicit maintenance bindings are supplied.
	PublicationV2 *PublicationV2Config
}

// PublicationV2Config is runtime-only trust configuration. Evidence remains
// in approved review_records; requests cannot supply or install it.
type PublicationV2Config struct {
	AllowSynthetic     bool
	TrustedReviewers   []TrustedPublicationReviewer
	BindingVerifier    ports.PublicationBindingVerifier
	MaintenanceTargets []MaintenanceTarget
	CleanupInterval    time.Duration
}
type TrustedPublicationReviewer struct{ Issuer, Subject string }

// MinOAuthConsentSecretBytes is the shortest accepted OAuth consent secret.
const MinOAuthConsentSecretBytes = 32

func (c Config) Validate() error {
	if c.PublicOrigin == "" {
		return errors.New("app: public origin is required")
	}
	u, err := url.Parse(c.PublicOrigin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(c.PublicOrigin, "/") {
		return fmt.Errorf("app: public origin must be one exact HTTPS origin")
	}
	if strings.ContainsAny(u.Host, "* ") {
		return errors.New("app: public origin cannot contain a wildcard or space")
	}
	if c.ResourceAudience == "" {
		c.ResourceAudience = c.PublicOrigin
	}
	if c.ResourceAudience != c.PublicOrigin {
		return errors.New("app: resource audience must equal public origin")
	}
	if len(c.OAuthConsentSecret) < MinOAuthConsentSecretBytes {
		return fmt.Errorf("app: oauth consent secret (GIST_OAUTH_CONSENT_SECRET) must be at least %d bytes", MinOAuthConsentSecretBytes)
	}
	if _, err := identity.LoadKeySet(c.SigningKeyConfig, identityClock{}); err != nil {
		return fmt.Errorf("app: GIST_SIGNING_KEY_CONFIG is required and must be valid: %w", err)
	}
	if c.DatabaseURL == "" {
		return errors.New("app: database URL is required")
	}
	if c.ObjectStoreRoot == "" {
		return errors.New("app: object store root is required")
	}
	if c.RequestTimeout <= 0 || c.MaxPackageBytes <= 0 || c.MaxExpandedBytes <= 0 || c.MaxRequestBytes <= 0 || c.MaxResponseBytes <= 0 || c.MaxCatalogEntries <= 0 || c.MaxDiscoveryResults <= 0 || c.MaxConcurrentRequests <= 0 {
		return errors.New("app: request timeout and all request limits are required")
	}
	if len(c.EventMaintenanceTargets) == 0 || len(c.EventMaintenanceTargets) > 100 {
		return errors.New("app: one to one hundred explicit event maintenance targets are required")
	}
	seen := map[string]bool{}
	for _, target := range c.EventMaintenanceTargets {
		if target.WorkspaceID == "" || target.Subject == "" || seen[target.WorkspaceID] {
			return errors.New("app: event maintenance targets require unique workspaces and subjects")
		}
		seen[target.WorkspaceID] = true
	}
	if c.PublicationV2 != nil {
		v := c.PublicationV2
		if len(v.TrustedReviewers) == 0 || len(v.TrustedReviewers) > 100 || len(v.MaintenanceTargets) == 0 || len(v.MaintenanceTargets) > 100 || v.CleanupInterval < 0 {
			return errors.New("app: v2 publication requires explicit trusted reviewers and maintenance targets")
		}
		seen = map[string]bool{}
		for _, reviewer := range v.TrustedReviewers {
			key := reviewer.Issuer + "\x00" + reviewer.Subject
			if reviewer.Issuer == "" || reviewer.Subject == "" || seen[key] {
				return errors.New("app: invalid v2 trusted reviewer configuration")
			}
			seen[key] = true
		}
		seen = map[string]bool{}
		for _, target := range v.MaintenanceTargets {
			if target.WorkspaceID == "" || target.Subject == "" || seen[target.WorkspaceID] {
				return errors.New("app: invalid v2 publication maintenance targets")
			}
			seen[target.WorkspaceID] = true
		}
	}
	return nil
}

func (c Config) RESTLimits() rest.Limits {
	return rest.Limits{MaxBodyBytes: c.MaxRequestBytes, MaxResponseBytes: c.MaxResponseBytes, MaxResults: c.MaxDiscoveryResults, RateLimit: c.MaxConcurrentRequests, RetryAfter: c.RetryAfter}
}
