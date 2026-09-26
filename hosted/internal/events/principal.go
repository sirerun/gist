package events

import "github.com/sirerun/gist/hosted/internal/ports"

// PrincipalHash returns the binding value a cursor carries for principal.
// Callers that resume a cursor from a client-supplied ID must set
// Cursor.PrincipalHash to this value so the store can reject cursors opened
// by a different principal.
func PrincipalHash(p ports.Principal) string { return principalHash(p) }
