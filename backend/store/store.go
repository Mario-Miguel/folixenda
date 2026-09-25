package store

import (
	"time"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"golang.org/x/crypto/bcrypt"
)

type scanner interface {
	Scan(dest ...any) error
}

// extraScanner appends extra destinations after the ones passed to Scan,
// letting scanEvent be reused for rows that carry additional columns.
type extraScanner struct {
	s     scanner
	extra []any
}

func (x extraScanner) Scan(dest ...any) error {
	return x.s.Scan(append(dest, x.extra...)...)
}

func hashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func eventTimestamps(e *models.Event) (startTS time.Time, endTS time.Time, err error) {
	startTS, err = time.Parse("2006-01-02 15:04", e.Date+" "+e.StartTime)
	if err != nil {
		return
	}
	endTS, err = time.Parse("2006-01-02 15:04", e.Date+" "+e.EndTime)
	return
}
