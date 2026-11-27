// Command migrate applies or inspects the database migrations in ./migrations.
//
//	go run ./cmd/migrate            # apply pending migrations (same as "up")
//	go run ./cmd/migrate status     # list migrations and whether they're applied
//	go run ./cmd/migrate down       # roll back the last migration
//
// It reads the database settings from .env, like the server.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Mario-Miguel/folixenda/backend/database"
	"github.com/Mario-Miguel/folixenda/backend/migrations"
)

func main() {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	database.Init(".env")
	p, err := migrations.NewProvider(database.DB)
	if err != nil {
		log.Fatalf("load migrations: %v", err)
	}
	ctx := context.Background()

	switch cmd {
	case "up":
		results, err := p.Up(ctx)
		for _, r := range results {
			fmt.Println(r)
		}
		if err != nil {
			log.Fatalf("migrate up: %v", err)
		}
		if len(results) == 0 {
			fmt.Println("no pending migrations")
		}
	case "down":
		r, err := p.Down(ctx)
		if r != nil {
			fmt.Println(r)
		}
		if err != nil {
			log.Fatalf("migrate down: %v", err)
		}
	case "status":
		statuses, err := p.Status(ctx)
		if err != nil {
			log.Fatalf("migrate status: %v", err)
		}
		for _, s := range statuses {
			applied := "pending"
			if !s.AppliedAt.IsZero() {
				applied = "applied " + s.AppliedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("%05d  %-28s  %s\n", s.Source.Version, s.Source.Path, applied)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (use up, down or status)\n", cmd)
		os.Exit(2)
	}
}
