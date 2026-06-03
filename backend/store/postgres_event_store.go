package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/lib/pq"
)

type PostgresEventStore struct {
	db *sql.DB
}

func NewPostgresEventStore(db *sql.DB) (*PostgresEventStore, error) {
	s := &PostgresEventStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PostgresEventStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id          TEXT PRIMARY KEY,
			title       TEXT NOT NULL,
			description TEXT,
			category    TEXT NOT NULL,
			date        date,
			start_time  timestamp,
			end_time    timestamp,
			venue       TEXT,
			address     TEXT,
			price       DOUBLE PRECISION NOT NULL DEFAULT 0,
			image_url   TEXT,
			artist_name TEXT,
			perks       TEXT[],
			is_saved    BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	return err
}

func (s *PostgresEventStore) List(date, category string) ([]*models.Event, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, category, date, start_time, end_time, venue, address,
		       price, image_url, artist_name, perks, is_saved
		FROM events
		WHERE ($1='' OR date::text=$1) AND ($2='' OR category=$2)
	`, date, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []*models.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *PostgresEventStore) Get(id string) (*models.Event, error) {
	row := s.db.QueryRow(`
		SELECT id, title, description, category, date, start_time, end_time, venue, address,
		       price, image_url, artist_name, perks, is_saved
		FROM events WHERE id=$1
	`, id)
	e, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

func eventTimestamps(e *models.Event) (dateVal time.Time, startTS time.Time, endTS time.Time, err error) {
	dateVal, err = time.Parse("2006-01-02", e.Date)
	if err != nil {
		return
	}
	startTS, err = time.Parse("2006-01-02 15:04", e.Date+" "+e.StartTime)
	if err != nil {
		return
	}
	endTS, err = time.Parse("2006-01-02 15:04", e.Date+" "+e.EndTime)
	return
}

func (s *PostgresEventStore) Create(e *models.Event) error {
	dateVal, startTS, endTS, err := eventTimestamps(e)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO events (id, title, description, category, date, start_time, end_time, venue, address,
		                    price, image_url, artist_name, perks, is_saved)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	`, e.ID, e.Title, e.Description, e.Category, dateVal, startTS, endTS, e.Venue, e.Address,
		e.Price, e.ImageURL, e.ArtistName, pq.Array(e.Perks), e.IsSaved)
	return err
}

func (s *PostgresEventStore) Update(e *models.Event) error {
	dateVal, startTS, endTS, err := eventTimestamps(e)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`
		UPDATE events SET title=$2, description=$3, category=$4, date=$5, start_time=$6, end_time=$7,
		venue=$8, address=$9, price=$10, image_url=$11, artist_name=$12, perks=$13, is_saved=$14
		WHERE id=$1
	`, e.ID, e.Title, e.Description, e.Category, dateVal, startTS, endTS, e.Venue, e.Address,
		e.Price, e.ImageURL, e.ArtistName, pq.Array(e.Perks), e.IsSaved)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresEventStore) Delete(id string) error {
	result, err := s.db.Exec(`DELETE FROM events WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanEvent(s scanner) (*models.Event, error) {
	e := &models.Event{}
	var perks pq.StringArray
	var dateVal, startTS, endTS time.Time
	if err := s.Scan(&e.ID, &e.Title, &e.Description, &e.Category, &dateVal, &startTS, &endTS,
		&e.Venue, &e.Address, &e.Price, &e.ImageURL, &e.ArtistName, &perks, &e.IsSaved); err != nil {
		return nil, err
	}
	e.Date = dateVal.Format("2006-01-02")
	e.StartTime = startTS.Format("15:04")
	e.EndTime = endTS.Format("15:04")
	e.Perks = []string(perks)
	return e, nil
}
