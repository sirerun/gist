package ports

import (
	"context"
	"errors"
)

// ErrIdentityNotFound is returned by IdentityStore.Revoke when no identity
// with that issuer and subject exists in the caller's workspace. Missing and
// foreign identities are deliberately indistinguishable.
var ErrIdentityNotFound = errors.New("identity not found")

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
