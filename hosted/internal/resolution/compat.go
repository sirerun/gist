// Package resolution preserves the hosted API using the shared catalog implementation.
package resolution

import shared "github.com/sirerun/gist/catalog/resolution"

type Catalog = shared.Catalog
type Authorizer = shared.Authorizer
type ResolutionStore = shared.ResolutionStore
type Clock = shared.Clock
type Request = shared.Request
type Digest = shared.Digest
type ArtifactPin = shared.ArtifactPin
type Finding = shared.Finding
type Result = shared.Result
type RuntimeRequirements = shared.RuntimeRequirements
type Resolver = shared.Resolver
type ConnectionRequest = shared.ConnectionRequest
type ConnectionService = shared.ConnectionService

const StatusReady = shared.StatusReady
const StatusRequiresConnection = shared.StatusRequiresConnection
const StatusRequiresSelection = shared.StatusRequiresSelection
const StatusIncomplete = shared.StatusIncomplete
const StatusUnsupportedRuntime = shared.StatusUnsupportedRuntime
const StatusRequiresGateway = shared.StatusRequiresGateway

var ErrServiceUnavailable = shared.ErrServiceUnavailable
var ErrBudgetExceeded = shared.ErrBudgetExceeded
var ErrScanLimit = shared.ErrScanLimit
var ErrArtifactRevoked = shared.ErrArtifactRevoked
var ErrArtifactNotFound = shared.ErrArtifactNotFound
var ErrAccessDenied = shared.ErrAccessDenied
var NewResolver = shared.NewResolver
var NewConnectionService = shared.NewConnectionService
