package ports

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// PrincipalHash is the canonical binding value for state owned by one
// principal (event cursors, stored resolutions). It is the hex SHA-256 of the
// JSON-encoded issuer, subject, audience and workspace: JSON string encoding
// keeps field boundaries unambiguous, and the hex digest is valid PostgreSQL
// text (a raw NUL separator is rejected by text columns).
func PrincipalHash(p Principal) string {
	b, _ := json.Marshal([4]string{p.Issuer, p.Subject, p.Audience, p.WorkspaceID})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
