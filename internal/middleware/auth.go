package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// tokenParser is the slice of auth.TokenManager that middleware needs, kept as
// an interface so the middleware package does not hard-depend on the concrete
// manager (and is trivial to fake in tests).
type tokenParser interface {
	Parse(token string) (auth.Identity, error)
}

// Authenticate returns middleware that requires a valid Bearer token, parses it
// into an auth.Identity, and stores it in the request context. Requests without
// a valid token are rejected with 401.
func Authenticate(tp tokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				httpx.Error(w, r, httpx.ErrUnauthorized)
				return
			}
			id, err := tp.Parse(token)
			if err != nil {
				httpx.Error(w, r, httpx.ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), id)))
		})
	}
}

// RequireRole returns middleware that allows the request only if the
// authenticated identity has one of the given roles. It must be mounted after
// Authenticate. Admins are always allowed.
func RequireRole(roles ...store.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := auth.IdentityFrom(r.Context())
			if !ok {
				httpx.Error(w, r, httpx.ErrUnauthorized)
				return
			}
			if id.IsAdmin() || slices.Contains(roles, id.Role) {
				next.ServeHTTP(w, r)
				return
			}
			httpx.Error(w, r, httpx.ErrForbidden)
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}
