package auth

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the auth handler and its token manager. The composition root
// constructs it once and mounts its public and protected routes.
type Module struct {
	handler *Handler
	tm      *TokenManager
}

// New wires the auth module: store → repository → service → handler.
func New(s *store.Store, tm *TokenManager) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s), tm)), tm: tm}
}

// TokenManager exposes the manager so the composition root can build the
// Authenticate middleware from the same signing key.
func (m *Module) TokenManager() *TokenManager { return m.tm }

// MountPublic registers routes that must be reachable without a token.
func (m *Module) MountPublic(r chi.Router) {
	r.Post("/auth/login", m.handler.login)
}

// MountProtected registers routes that require an authenticated identity. Mount
// it on a router that already applies the Authenticate middleware.
func (m *Module) MountProtected(r chi.Router) {
	r.Get("/auth/me", m.handler.me)
}
