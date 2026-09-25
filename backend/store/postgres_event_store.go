package store

import (
	"database/sql"
	"errors"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/lib/pq"
)

type EventStore interface {
	List(date, category string) ([]*models.Event, error)
	Get(id string) (*models.Event, error)
	Create(e *models.Event) error
	Update(e *models.Event) error
	Delete(id string) error
}

type EventStoreDBConnection struct {
	db *sql.DB
}

func NewEventStore(db *sql.DB) (*EventStoreDBConnection, error) {
	s := &EventStoreDBConnection{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *EventStoreDBConnection) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id          TEXT PRIMARY KEY,
			title       TEXT NOT NULL,
			description TEXT,
			category    TEXT NOT NULL,
			start_time  timestamp,
			end_time    timestamp,
			venue       TEXT,
			address     TEXT,
			price       DOUBLE PRECISION NOT NULL DEFAULT 0,
			image_url   TEXT,
			artist_name TEXT,
			perks       TEXT[],
			is_saved    BOOLEAN NOT NULL DEFAULT FALSE,
			lat 		DOUBLE PRECISION,
			lon 		DOUBLE PRECISION
		)
	`)
	if err != nil {
		return err
	}
	// Drop legacy date column, backfilling NULLs first if it still exists.
	_, err = s.db.Exec(`
		DO $$ BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='events' AND column_name='date'
			) THEN
				UPDATE events
				SET
					start_time = COALESCE(start_time, date::timestamp),
					end_time   = COALESCE(end_time,   date::timestamp);
				ALTER TABLE events DROP COLUMN date;
			END IF;
		END $$;
	`)
	return err
}

func (s *EventStoreDBConnection) List(date, category string) ([]*models.Event, error) {
	rows, err := s.db.Query(`
		SELECT id, 
			   title,
			   description, 
			   category, 
			   start_time, 
			   end_time, 
			   price,
			   venue, 
			   address,
		       image_url, 
			   artist_name, 
			   perks, 
			   is_saved,
			   lat,
			   lon
		FROM events
		WHERE ($1='' OR start_time::date::text=$1) AND ($2='' OR category=$2)
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

func (s *EventStoreDBConnection) Get(id string) (*models.Event, error) {
	row := s.db.QueryRow(`
		SELECT id, title, description, category, start_time, end_time, price, venue, address,
		       image_url, artist_name, perks, is_saved, lat, lon
		FROM events WHERE id=$1
	`, id)
	e, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

func (s *EventStoreDBConnection) Create(e *models.Event) error {
	startTS, endTS, err := eventTimestamps(e)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO events (id, title, description, category, start_time, end_time, venue, address,
		                    price, image_url, artist_name, perks, is_saved, lat, lon)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, e.ID, e.Title, e.Description, e.Category, startTS, endTS, e.Venue, e.Address,
		e.Price, e.ImageURL, e.ArtistName, pq.Array(e.Perks), e.IsSaved, e.Lat, e.Lon)
	return err
}

func (s *EventStoreDBConnection) Update(e *models.Event) error {
	startTS, endTS, err := eventTimestamps(e)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`
		UPDATE events SET title=$2, description=$3, category=$4, start_time=$5, end_time=$6,
		venue=$7, address=$8, price=$9, image_url=$10, artist_name=$11, perks=$12, is_saved=$13,
		lat=$14, lon=$15
		WHERE id=$1
	`, e.ID, e.Title, e.Description, e.Category, startTS, endTS, e.Venue, e.Address,
		e.Price, e.ImageURL, e.ArtistName, pq.Array(e.Perks), e.IsSaved, e.Lat, e.Lon)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *EventStoreDBConnection) Delete(id string) error {
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
	var startTS, endTS sql.NullTime
	var description, venue, address, imageURL, artistName sql.NullString
	var lat, lon sql.NullFloat64
	if err := s.Scan(&e.ID, &e.Title, &description, &e.Category, &startTS, &endTS,
		&e.Price, &venue, &address, &imageURL, &artistName, &perks, &e.IsSaved, &lat, &lon); err != nil {
		return nil, err
	}
	e.Description = description.String
	e.Venue = venue.String
	e.Address = address.String
	e.ImageURL = imageURL.String
	e.ArtistName = artistName.String
	if startTS.Valid {
		e.Date = startTS.Time.Format("2006-01-02")
		e.StartTime = startTS.Time.Format("15:04")
	}
	if endTS.Valid {
		e.EndTime = endTS.Time.Format("15:04")
	}
	e.Perks = []string(perks)
	if lat.Valid && lon.Valid {
		e.Lat = &lat.Float64
		e.Lon = &lon.Float64
	}
	return e, nil
}
