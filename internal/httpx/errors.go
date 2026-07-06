// Package httpx contains transport-level helpers shared by every module:
// JSON encoding/decoding, a stable error envelope, domain-error → HTTP status
// mapping, and pagination parsing. Handlers depend on this package so that
// error and response shapes stay uniform across the whole API.
package httpx

import "errors"

// Sentinel domain errors. Services return these (optionally wrapped with
// context via fmt.Errorf("...: %w", ErrX)); handlers pass them to Error, which
// maps them to the correct HTTP status. This keeps status-code decisions out of
// business logic.
var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("resource already exists")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInsufficientStock = errors.New("insufficient stock")
)

// APIError is an error carrying an explicit HTTP status and a client-safe
// message. Use it when a handler needs a specific status that the sentinel
// mapping does not cover.
type APIError struct {
	Status  int
	Message string
	Err     error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *APIError) Unwrap() error { return e.Err }

// NewError builds an APIError with the given status and client-safe message.
func NewError(status int, message string) *APIError {
	return &APIError{Status: status, Message: message}
}
