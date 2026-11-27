package store

import (
	"errors"

	"github.com/lib/pq"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// Postgres error codes we translate into store errors.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgInvalidTextRepr     = "22P02" // e.g. a malformed uuid literal
)

// translateError maps Postgres errors onto the store's sentinel errors so the
// handlers don't need to know about the driver:
//   - unique violation      → ErrConflict
//   - foreign key violation → ErrNotFound (the referenced row doesn't exist)
//   - invalid text repr.    → ErrNotFound (e.g. "abc" can't be a uuid id)
//
// Other errors are returned unchanged.
func translateError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	switch pqErr.Code {
	case pgUniqueViolation:
		return errors.Join(ErrConflict, err)
	case pgForeignKeyViolation, pgInvalidTextRepr:
		return errors.Join(ErrNotFound, err)
	}
	return err
}
