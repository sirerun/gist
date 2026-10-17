// Package migrations retains the exact reviewed forward SQL assets in the
// migration binary. The server never applies these on startup.
package migrations

import "embed"

// Files contains only ordered SQL migration assets.
//
//go:embed *.sql
var Files embed.FS
