// Command mockevents generates synthetic events and either writes them as JSON
// or inserts them into the Postgres database.
//
//	go run ./cmd/mockevents -n 1000 -out events.json   # write JSON file ("-" for stdout)
//	go run ./cmd/mockevents -n 1000 -db                # insert into Postgres (uses .env)
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/Mario-Miguel/folixenda/backend/database"
	"github.com/Mario-Miguel/folixenda/backend/mock"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func main() {
	n := flag.Int("n", 1000, "number of events to generate")
	seed := flag.Int64("seed", 42, "random seed")
	out := flag.String("out", "-", "output JSON file (\"-\" for stdout)")
	toDB := flag.Bool("db", false, "insert events into Postgres instead of writing JSON")
	flag.Parse()

	events := mock.GenerateEvents(*n, *seed)

	if *toDB {
		database.Init(".env")
		s, err := store.NewEventStore(database.DB)
		if err != nil {
			log.Fatalf("event store: %v", err)
		}
		inserted := 0
		for _, e := range events {
			if err := s.Create(e); err != nil {
				log.Printf("skip event %s: %v", e.Title, err)
				continue
			}
			inserted++
		}
		log.Printf("inserted %d/%d mock events", inserted, len(events))
		return
	}

	w := os.Stdout
	if *out != "-" {
		f, err := os.Create(*out)
		if err != nil {
			log.Fatalf("create %s: %v", *out, err)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(events); err != nil {
		log.Fatalf("encode: %v", err)
	}
	if *out != "-" {
		log.Printf("wrote %d mock events to %s", len(events), *out)
	}
}
