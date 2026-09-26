package contract

import "fmt"

type FindingStatus string

const (
	FindingReady              FindingStatus = "ready"
	FindingRequiresConnection FindingStatus = "requires_connection"
	FindingRequiresSelection  FindingStatus = "requires_selection"
	FindingIncomplete         FindingStatus = "incomplete"
	FindingUnsupportedRuntime FindingStatus = "unsupported_runtime"
	FindingRequiresGateway    FindingStatus = "requires_gateway"
)

type ResolutionWire struct {
	Required  []FindingWire
	Aggregate FindingStatus
}
type FindingWire struct {
	CapabilityID string
	Required     bool
	Status       FindingStatus
	ConnectURL   string
	Provenance   string
}

func ValidateResolution(r ResolutionWire) error {
	if len(r.Required) == 0 {
		return fmt.Errorf("resolution must contain findings")
	}
	allReady := true
	for _, f := range r.Required {
		if f.CapabilityID == "" || f.Status == "" {
			return fmt.Errorf("finding is incomplete")
		}
		if f.Required && f.Status != FindingReady {
			allReady = false
		}
		if f.Status == FindingRequiresConnection && f.ConnectURL == "" && f.Provenance != "client_asserted" {
			return fmt.Errorf("connection finding needs connect URL or client assertion")
		}
	}
	if r.Aggregate == FindingReady && !allReady {
		return fmt.Errorf("ready aggregate has non-ready required finding")
	}
	if allReady && r.Aggregate != FindingReady {
		return fmt.Errorf("all required findings must produce ready aggregate")
	}
	return nil
}

type ErrorCode string

const (
	Unauthorized       ErrorCode = "unauthorized"
	Forbidden          ErrorCode = "forbidden"
	NotFound           ErrorCode = "not_found"
	VersionConflict    ErrorCode = "version_conflict"
	ArtifactRevoked    ErrorCode = "artifact_revoked"
	CursorExpired      ErrorCode = "cursor_expired"
	ResolutionExpired  ErrorCode = "resolution_expired"
	BudgetExceeded     ErrorCode = "budget_exceeded"
	ValidationFailed   ErrorCode = "validation_failed"
	IntegrityError     ErrorCode = "integrity_error"
	RateLimited        ErrorCode = "rate_limited"
	ServiceUnavailable ErrorCode = "service_unavailable"
)

var errorStatus = map[ErrorCode]int{Unauthorized: 401, Forbidden: 403, NotFound: 404, VersionConflict: 409, ArtifactRevoked: 409, CursorExpired: 409, ResolutionExpired: 409, BudgetExceeded: 413, ValidationFailed: 422, IntegrityError: 422, RateLimited: 429, ServiceUnavailable: 503}

func HTTPStatus(code ErrorCode) (int, bool) { s, ok := errorStatus[code]; return s, ok }
