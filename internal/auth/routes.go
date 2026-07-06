package auth

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the auth handler and its token manager. The composition root
// constructs it once and mounts its public and protected routes.
type Module struct {
	handler *Handler
	tm      *TokenManager
}

// New wires the auth module: store → repository → service → handler. refreshTTL
// is the lifetime of issued refresh tokens.
func New(s *store.Store, tm *TokenManager, refreshTTL time.Duration) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s), tm, refreshTTL)), tm: tm}
}

// TokenManager exposes the manager so the composition root can build the
// Authenticate middleware from the same signing key.
func (m *Module) TokenManager() *TokenManager { return m.tm }

// MountPublic registers routes that must be reachable without an access token
// (they authenticate via credentials or a refresh token). loginLimiter is
// applied only to the login route — the brute-force-sensitive one — so refresh
// and logout are not throttled by login's tight bucket.
func (m *Module) MountPublic(r chi.Router, loginLimiter func(http.Handler) http.Handler) {
	r.With(loginLimiter).Post("/auth/login", m.handler.login)
	r.Post("/auth/refresh", m.handler.refresh)
	r.Post("/auth/logout", m.handler.logout)
}

// MountProtected registers routes that require an authenticated identity. Mount
// it on a router that already applies the Authenticate middleware.
func (m *Module) MountProtected(r chi.Router) {
	r.Get("/auth/me", m.handler.me)
}
