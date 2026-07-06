package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is a shared, thread-safe validator instance. Request DTOs use
// `validate:"..."` struct tags; Decode runs them automatically.
var validate = validator.New(validator.WithRequiredStructEnabled())

// Decode reads a JSON body into dst and runs struct validation. It rejects
// unknown fields and oversized bodies. On validation failure it returns a
// *ValidationErrors carrying per-field messages, which handlers surface via
// ValidationError.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	const maxBytes = 1 << 20 // 1 MiB
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return &APIError{Status: http.StatusBadRequest, Message: decodeMessage(err), Err: err}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &APIError{Status: http.StatusBadRequest, Message: "body must contain a single JSON object"}
	}

	if err := validate.Struct(dst); err != nil {
		if _, ok := errors.AsType[*validator.InvalidValidationError](err); ok {
			return err
		}
		var verrs validator.ValidationErrors
		if !errors.As(err, &verrs) {
			return err
		}
		fields := map[string]string{}
		for _, fe := range verrs {
			fields[strings.ToLower(fe.Field())] = validationMessage(fe)
		}
		return &ValidationErrors{Fields: fields}
	}
	return nil
}

// ValidationErrors carries per-field validation failures.
type ValidationErrors struct {
	Fields map[string]string
}

func (e *ValidationErrors) Error() string { return "validation failed" }

// QueryInt returns the integer query parameter named key, or fallback when it is
// absent or unpar. Negative results are clamped to fallback.
func QueryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func decodeMessage(err error) string {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		return fmt.Sprintf("malformed JSON at position %d", syntaxErr.Offset)
	case errors.As(err, &typeErr):
		return fmt.Sprintf("invalid value for field %q", typeErr.Field)
	case errors.Is(err, io.EOF):
		return "request body must not be empty"
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		return strings.TrimPrefix(err.Error(), "json: ")
	default:
		return "invalid request body"
	}
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param()
	case "max":
		return "must be at most " + fe.Param()
	case "gt":
		return "must be greater than " + fe.Param()
	case "gte":
		return "must be greater than or equal to " + fe.Param()
	case "oneof":
		return "must be one of: " + fe.Param()
	default:
		return "is invalid"
	}
}
