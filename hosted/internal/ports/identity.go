package ports

import "context"

type IdentityRecord struct {
	Issuer, Subject, WorkspaceID, SubjectType string
	Scopes                                    []string
	PolicyGeneration                          uint64
	ExpiresAt                                 int64
}

type IdentityStore interface {
	Lookup(context.Context, string, string) (IdentityRecord, error)
	Revoke(context.Context, string, string) error
}
