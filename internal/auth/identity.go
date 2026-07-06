// Package auth provides authentication: password hashing, JWT issuing/parsing,
// the authenticated Identity carried through request context, and the login
// service. Middleware and feature modules depend on this package to read the
// current user; auth itself depends only on store and config.
package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Identity is the authenticated principal derived from a validated JWT. BranchID
// is nil for chain-wide roles (admins), which may operate across all branches.
type Identity struct {
	UserID   uuid.UUID
	Role     store.UserRole
	BranchID *uuid.UUID
}

// IsAdmin reports whether the identity has unrestricted, chain-wide access.
func (i Identity) IsAdmin() bool { return i.Role == store.UserRoleAdmin }

// CanAccessBranch reports whether the identity may act on the given branch.
// Admins may access any branch; everyone else is confined to their own.
func (i Identity) CanAccessBranch(branchID uuid.UUID) bool {
	if i.IsAdmin() {
		return true
	}
	return i.BranchID != nil && *i.BranchID == branchID
}

// ResolveBranch determines which branch an operation targets, enforcing tenant
// isolation. Admins must name a branch explicitly (they span the whole chain);
// branch-scoped staff are confined to their own branch, and any mismatch with a
// requested branch is forbidden. Shared by purchasing, inventory, and sales.
func (i Identity) ResolveBranch(requested *uuid.UUID) (uuid.UUID, error) {
	if i.IsAdmin() {
		if requested == nil {
			return uuid.Nil, fmt.Errorf("branch_id is required for admins: %w", httpx.ErrInvalidInput)
		}
		return *requested, nil
	}
	if i.BranchID == nil {
		return uuid.Nil, httpx.ErrForbidden
	}
	if requested != nil && *requested != *i.BranchID {
		return uuid.Nil, httpx.ErrForbidden
	}
	return *i.BranchID, nil
}

type ctxKey int

const identityKey ctxKey = iota

// WithIdentity returns a copy of ctx carrying the identity.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFrom returns the identity stored in ctx and whether one was present.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}
