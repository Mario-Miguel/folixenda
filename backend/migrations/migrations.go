// Package migrations holds the SQL schema migrations of the Go API, embedded
// in the binary and applied with goose.
//
// Files are named NNNNN_description.sql and use goose annotations
// (-- +goose Up / -- +goose Down). Applied versions are recorded in the
// goose_db_version table. The server applies pending migrations on startup;
// `go run ./cmd/migrate` runs them by hand.
package migrations

import (
	"context"
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var FS embed.FS

// NewProvider returns a goose provider for the embedded migrations.
func NewProvider(db *sql.DB) (*goose.Provider, error) {
	return goose.NewProvider(goose.DialectPostgres, db, FS)
}

// Up applies every pending migration.
func Up(ctx context.Context, db *sql.DB) ([]*goose.MigrationResult, error) {
	p, err := NewProvider(db)
	if err != nil {
		return nil, err
	}
	return p.Up(ctx)
}
