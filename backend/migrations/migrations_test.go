package migrations

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProviderLoadsEmbeddedMigrations(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	p, err := NewProvider(db)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	sources := p.ListSources()
	if len(sources) == 0 {
		t.Fatal("no migrations embedded")
	}
	for i, s := range sources {
		if s.Version != int64(i+1) {
			t.Errorf("migration %d has version %d: versions must be consecutive from 1", i, s.Version)
		}
	}
}

func TestMigrationsHaveUpAndDown(t *testing.T) {
	files, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		b, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, marker := range []string{"-- +goose Up", "-- +goose Down"} {
			if !strings.Contains(src, marker) {
				t.Errorf("%s: missing %q", name, marker)
			}
		}
		if strings.Count(src, "-- +goose StatementBegin") != strings.Count(src, "-- +goose StatementEnd") {
			t.Errorf("%s: unbalanced StatementBegin/StatementEnd", name)
		}
	}
}
