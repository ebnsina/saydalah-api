package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes we translate into domain errors.
// See https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

func pgErrCode(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code
	}
	return ""
}

// IsUniqueViolation reports whether err is a unique-constraint violation
// (e.g. a duplicate email or barcode).
func IsUniqueViolation(err error) bool { return pgErrCode(err) == codeUniqueViolation }

// IsForeignKeyViolation reports whether err is a foreign-key violation
// (e.g. referencing a branch or product that does not exist).
func IsForeignKeyViolation(err error) bool { return pgErrCode(err) == codeForeignKeyViolation }

// IsCheckViolation reports whether err is a CHECK-constraint violation
// (e.g. a negative quantity).
func IsCheckViolation(err error) bool { return pgErrCode(err) == codeCheckViolation }
