// Package packages preserves the hosted API using the shared catalog implementation.
package packages

import shared "github.com/sirerun/gist/catalog/packages"

type Limits = shared.Limits
type Archive = shared.Archive
type Envelope = shared.Envelope
type Retriever = shared.Retriever
type Manifest = shared.Manifest
type Package = shared.Package

var ErrDenied = shared.ErrDenied
var ErrBudgetExceeded = shared.ErrBudgetExceeded
var DefaultLimits = shared.DefaultLimits
var ReadArchive = shared.ReadArchive
var Validate = shared.Validate
var ScreenText = shared.ScreenText
