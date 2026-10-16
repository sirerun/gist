// Package ports preserves the hosted API using the shared catalog implementation.
package ports

import shared "github.com/sirerun/gist/catalog/ports"

type ArtifactKind = shared.ArtifactKind
type ArtifactRef = shared.ArtifactRef
type Digest = shared.Digest
type CatalogRecord = shared.CatalogRecord
type SearchQuery = shared.SearchQuery
type SearchPage = shared.SearchPage
type CatalogStore = shared.CatalogStore
type ArtifactReader = shared.ArtifactReader
type ArtifactStore = shared.ArtifactStore
type Action = shared.Action
type Principal = shared.Principal
type Decision = shared.Decision
type Authorizer = shared.Authorizer
type Cursor = shared.Cursor
type Resolution = shared.Resolution
type Finding = shared.Finding
type ResolutionStore = shared.ResolutionStore
type ConnectionStatus = shared.ConnectionStatus
type Connection = shared.Connection
type ConnectionInitiator = shared.ConnectionInitiator
type Clock = shared.Clock
type SystemClock = shared.SystemClock
type EventType = shared.EventType
type Event = shared.Event
type EventPage = shared.EventPage
type EventStore = shared.EventStore
type LexicalSearcher = shared.LexicalSearcher
type ArtifactPin = shared.ArtifactPin
type PinnedFinding = shared.PinnedFinding
type PinnedResolution = shared.PinnedResolution
type PinnedResolutionStore = shared.PinnedResolutionStore
type IdentityRecord = shared.IdentityRecord
type IdentityStore = shared.IdentityStore
type V2Publisher = shared.V2Publisher
type PublicationResult = shared.PublicationResult
type V2Reader = shared.V2Reader
type PublicationRead = shared.PublicationRead
type PreparedPublication = shared.PreparedPublication
type PublicationReceipt = shared.PublicationReceipt
type PublicationAdmissionEvidence = shared.PublicationAdmissionEvidence
type PublicationEvidenceArtifact = shared.PublicationEvidenceArtifact
type PublicationEvidenceSource = shared.PublicationEvidenceSource
type PublicationEvidenceCollector = shared.PublicationEvidenceCollector
type PublicationEvidenceRights = shared.PublicationEvidenceRights
type PublicationEvidenceReviewer = shared.PublicationEvidenceReviewer
type PublicationEvidenceOrigin = shared.PublicationEvidenceOrigin
type PublicationProviderQualification = shared.PublicationProviderQualification
type PublicationConformance = shared.PublicationConformance
type PublicationTaxonomyGrant = shared.PublicationTaxonomyGrant
type PublicationBindingVerifier = shared.PublicationBindingVerifier

const KindSkill = shared.KindSkill
const KindCapability = shared.KindCapability
const KindTool = shared.KindTool
const KindProvider = shared.KindProvider
const KindBinding = shared.KindBinding
const KindTaxonomy = shared.KindTaxonomy
const KindConnection = shared.KindConnection
const KindResolution = shared.KindResolution
const ActionRead = shared.ActionRead
const ActionPublish = shared.ActionPublish
const ActionIdentityRevoke = shared.ActionIdentityRevoke
const ConnectionPending = shared.ConnectionPending
const ConnectionConnected = shared.ConnectionConnected
const ConnectionFailed = shared.ConnectionFailed
const ConnectionCanceled = shared.ConnectionCanceled
const EventVersionPublished = shared.EventVersionPublished
const EventVersionRevoked = shared.EventVersionRevoked

var ErrIdentityNotFound = shared.ErrIdentityNotFound
var PrincipalHash = shared.PrincipalHash
