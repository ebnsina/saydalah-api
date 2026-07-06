package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

// envelope is the stable JSON error shape returned to clients:
//
//	{"error": {"message": "...", "details": {...}}}
type envelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// JSON writes v as an indented JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("httpx: failed to encode response", "error", err)
	}
}

// JSONCached writes a 200 with Cache-Control and a content ETag, and returns
// 304 Not Modified when the client's If-None-Match matches (saving bandwidth on
// unchanged reads). maxAge is the client freshness window in seconds; use it for
// rarely-changing reads (e.g. tax settings, product categories, catalog pages).
func JSONCached(w http.ResponseWriter, r *http.Request, maxAge int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.Error("httpx: failed to marshal cached response", "error", err)
		Error(w, r, err)
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age="+strconv.Itoa(maxAge))
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		slog.Error("httpx: failed to write cached response", "error", err)
	}
}

// NoContent writes a 204 with no body.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Error inspects err, maps it to an HTTP status, and writes the error envelope.
// Server-side (5xx) errors are logged with their full detail; the client only
// ever sees a safe message. This is the single choke point for error responses.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	if verrs, ok := errors.AsType[*ValidationErrors](err); ok {
		ValidationError(w, verrs.Fields)
		return
	}

	status, msg := classify(err)

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed",
			"method", r.Method, "path", r.URL.Path, "error", err)
	}
	JSON(w, status, envelope{Error: errorBody{Message: msg}})
}

// ValidationError writes a 422 with per-field validation messages.
func ValidationError(w http.ResponseWriter, fields map[string]string) {
	JSON(w, http.StatusUnprocessableEntity, envelope{
		Error: errorBody{Message: "validation failed", Details: fields},
	})
}

// classify maps sentinel and typed errors to (status, client-safe message).
func classify(err error) (int, string) {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.Status, apiErr.Message
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, "resource not found"
	case errors.Is(err, ErrConflict):
		return http.StatusConflict, "resource already exists"
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest, "invalid input"
	case errors.Is(err, ErrInsufficientStock):
		return http.StatusConflict, "insufficient stock"
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden, "forbidden"
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}
