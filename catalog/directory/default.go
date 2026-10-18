package directory

import _ "embed"

//go:embed snapshot.json
var defaultSnapshot []byte

// Default constructs an independent immutable index from the pinned public
// snapshot. No network requests or execution happen during initialization.
func Default() (*Index, error) { return Parse(defaultSnapshot) }

// DefaultSnapshot returns a copy of the committed, attributed snapshot.
func DefaultSnapshot() []byte { return append([]byte(nil), defaultSnapshot...) }
