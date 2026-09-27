package oauth

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
)

type jwk struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	X   string `json:"x"`
	KID string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

// handleJWKS publishes the public half of every live identity signing key.
// The keys come from the identity key set; there is no second key system.
func (s *Server) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	keys := []jwk{}
	for _, k := range s.cfg.Keys.VerificationKeys() {
		if k.Algorithm != "EdDSA" || len(k.Public) == 0 {
			continue
		}
		keys = append(keys, jwk{KTY: "OKP", CRV: "Ed25519", X: base64.RawURLEncoding.EncodeToString(k.Public), KID: k.KID, Use: "sig", Alg: "EdDSA"})
	}
	w.Header().Set("Content-Type", "application/jwk-set+json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.WriteHeader(http.StatusOK)
	_ = jsonEncoder(w).Encode(map[string]any{"keys": keys})
}

func jsonEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}
