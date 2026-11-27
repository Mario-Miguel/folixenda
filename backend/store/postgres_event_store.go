package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/lib/pq"
)

type EventStore interface {
	List(f EventFilter) ([]*models.Event, error)
	Get(id string) (*models.Event, error)
	// Create inserts the event and its sessions (at least one) in one
	// transaction.
	Create(e *models.Event) error
	// Update replaces the event's fields and sessions in one transaction.
	Update(e *models.Event) error
	// Delete removes the event (its sessions and image rows cascade) and
	// returns the object keys of its images, for the caller to delete from
	// object storage after the commit.
	Delete(id string) (imageKeys []string, err error)
	// ListByPublisher returns every event owned by publisherID (past and
	// future), ordered by start time, with how many users saved each one.
	ListByPublisher(publisherID string) ([]*models.PublisherEvent, error)
}

// URLFunc turns an object key into its public URL.
type URLFunc func(key string) string

func (f URLFunc) url(key string) string {
	if f == nil {
		return key
	}
	return f(key)
}

type EventStoreDBConnection struct {
	db       *sql.DB
	imageURL URLFunc
}

// NewEventStore returns a store backed by db; imageURL builds the URLs of
// uploaded images (nil leaves the object key). The schema is created by the
// migrations in backend/migrations, not here.
func NewEventStore(db *sql.DB, imageURL URLFunc) *EventStoreDBConnection {
	return &EventStoreDBConnection{db: db, imageURL: imageURL}
}

// List returns the events matching f. Callers should Validate f first.
func (s *EventStoreDBConnection) List(f EventFilter) ([]*models.Event, error) {
	query, args := buildListEventsQuery(f)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []*models.Event
	for rows.Next() {
		e, err := scanEvent(rows, s.imageURL)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Get returns one event. When it has an owner with a publisher profile, the
// profile is joined in as Organizer.
func (s *EventStoreDBConnection) Get(id string) (*models.Event, error) {
	row := s.db.QueryRow(`
		SELECT `+eventColumnsE+`,
		       pp.org_name, pp.website, pp.phone, pp.description
		FROM events e
		LEFT JOIN publisher_profiles pp ON pp.user_id = e.publisher_id
		WHERE e.id=$1
	`, id)
	var orgName, website, phone, description sql.NullString
	e, err := scanEvent(extraScanner{s: row, extra: []any{&orgName, &website, &phone, &description}}, s.imageURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if orgName.Valid {
		e.Organizer = &models.Organizer{
			ID:          e.PublisherID,
			OrgName:     orgName.String,
			Website:     website.String,
			Phone:       phone.String,
			Description: description.String,
		}
	}
	return e, nil
}

// ListByPublisher counts saves with a LEFT JOIN + GROUP BY (one query, events
// without saves get 0). Grouping by e.id is enough: it's the primary key.
func (s *EventStoreDBConnection) ListByPublisher(publisherID string) ([]*models.PublisherEvent, error) {
	rows, err := s.db.Query(`
		SELECT `+eventColumnsE+`, COUNT(se.id)
		FROM events e
		LEFT JOIN user_saved_events se ON se.event_id = e.id
		WHERE e.publisher_id = $1
		GROUP BY e.id
		ORDER BY e.start_time, e.id
	`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []*models.PublisherEvent{}
	for rows.Next() {
		var count int
		e, err := scanEvent(extraScanner{s: rows, extra: []any{&count}}, s.imageURL)
		if err != nil {
			return nil, err
		}
		events = append(events, &models.PublisherEvent{Event: *e, SavesCount: count})
	}
	return events, rows.Err()
}

const (
	// deleteSessionsSQL drops the event's sessions, including the default one
	// the events_default_session trigger adds on INSERT.
	deleteSessionsSQL = `DELETE FROM event_sessions WHERE event_id=$1`
	insertSessionsSQL = `INSERT INTO event_sessions (event_id, start_time, end_time)
		SELECT $1, t.s, t.e FROM unnest($2::timestamp[], $3::timestamp[]) AS t(s, e)`
)

// inTx runs fn in a transaction, committing when it returns nil.
func (s *EventStoreDBConnection) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback() //nolint:errcheck
		return err
	}
	return tx.Commit()
}

// replaceSessions deletes the event's sessions and inserts rows'.
func replaceSessions(tx *sql.Tx, eventID string, rows sessionRows) error {
	if _, err := tx.Exec(deleteSessionsSQL, eventID); err != nil {
		return err
	}
	_, err := tx.Exec(insertSessionsSQL, eventID, pq.StringArray(rows.starts), pq.StringArray(rows.ends))
	return translateError(err)
}

func (s *EventStoreDBConnection) Create(e *models.Event) error {
	rows, err := buildSessionRows(e.Sessions)
	if err != nil {
		return err
	}
	return s.inTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
		INSERT INTO events (id, title, description, category, start_time, end_time, venue, address,
		                    price, image_url, artist_name, perks, is_saved, lat, lon, publisher_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, e.ID, e.Title, e.Description, e.Category, rows.spanStart, rows.spanEnd, e.Venue, e.Address,
			e.Price, e.StoredImageURL, e.ArtistName, pq.Array(e.Perks), e.IsSaved, e.Lat, e.Lon,
			nullIfEmpty(e.PublisherID))
		if err != nil {
			return translateError(err)
		}
		return replaceSessions(tx, e.ID, rows)
	})
}

// Update replaces the event's fields and sessions and bumps updated_at. It
// never changes publisher_id (ownership); e.PublisherID is ignored.
func (s *EventStoreDBConnection) Update(e *models.Event) error {
	rows, err := buildSessionRows(e.Sessions)
	if err != nil {
		return err
	}
	return s.inTx(func(tx *sql.Tx) error {
		result, err := tx.Exec(`
		UPDATE events SET title=$2, description=$3, category=$4, start_time=$5, end_time=$6,
		venue=$7, address=$8, price=$9, image_url=$10, artist_name=$11, perks=$12, is_saved=$13,
		lat=$14, lon=$15, updated_at=now()
		WHERE id=$1
	`, e.ID, e.Title, e.Description, e.Category, rows.spanStart, rows.spanEnd, e.Venue, e.Address,
			e.Price, e.StoredImageURL, e.ArtistName, pq.Array(e.Perks), e.IsSaved, e.Lat, e.Lon)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return replaceSessions(tx, e.ID, rows)
	})
}

// Delete locks the event row first so a concurrent image upload (which also
// locks it, see AddImage) either finishes before and is returned here, or
// finds the event gone.
func (s *EventStoreDBConnection) Delete(id string) ([]string, error) {
	var keys []string
	err := s.inTx(func(tx *sql.Tx) error {
		if err := lockEvent(tx, id); err != nil {
			return err
		}
		var arr pq.StringArray
		if err := tx.QueryRow(`SELECT COALESCE(array_agg(object_key), '{}') FROM event_images WHERE event_id=$1`, id).
			Scan(&arr); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM events WHERE id=$1`, id); err != nil {
			return err
		}
		keys = arr
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// imageJSON is one element of imagesColumn.
type imageJSON struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Kind     string `json:"kind"`
	Position int    `json:"position"`
	Width    *int   `json:"width"`
	Height   *int   `json:"height"`
}

func scanEvent(s scanner, imageURL URLFunc) (*models.Event, error) {
	e := &models.Event{}
	var perks pq.StringArray
	var startTS, endTS sql.NullTime
	var description, venue, address, imageURLCol, artistName sql.NullString
	var publisherID sql.NullString
	var lat, lon sql.NullFloat64
	var sessionsJSON, imagesJSON []byte
	if err := s.Scan(&e.ID, &e.Title, &description, &e.Category, &startTS, &endTS,
		&e.Price, &venue, &address, &imageURLCol, &artistName, &perks, &e.IsSaved, &lat, &lon,
		&publisherID, &sessionsJSON, &imagesJSON); err != nil {
		return nil, err
	}
	e.PublisherID = publisherID.String
	e.Description = description.String
	e.Venue = venue.String
	e.Address = address.String
	e.StoredImageURL = imageURLCol.String
	e.ArtistName = artistName.String
	e.Perks = []string(perks)
	if lat.Valid && lon.Valid {
		e.Lat = &lat.Float64
		e.Lon = &lon.Float64
	}

	if len(sessionsJSON) > 0 {
		if err := json.Unmarshal(sessionsJSON, &e.Sessions); err != nil {
			return nil, fmt.Errorf("event %s sessions: %w", e.ID, err)
		}
	}
	if len(e.Sessions) == 0 && startTS.Valid {
		// No session rows (shouldn't happen after migration 00004): fall back
		// to the event's own span as a single session.
		end := startTS.Time
		if endTS.Valid {
			end = endTS.Time
		}
		e.Sessions = []models.EventSession{{Date: startTS.Time.Format(dateLayout),
			StartTime: startTS.Time.Format("15:04"), EndTime: end.Format("15:04")}}
	}
	e.ApplySessions()

	var images []imageJSON
	if len(imagesJSON) > 0 {
		if err := json.Unmarshal(imagesJSON, &images); err != nil {
			return nil, fmt.Errorf("event %s images: %w", e.ID, err)
		}
	}
	e.Images = make([]models.EventImage, 0, len(images))
	for _, img := range images {
		e.Images = append(e.Images, models.EventImage{ID: img.ID, URL: imageURL.url(img.Key), Kind: img.Kind,
			Position: img.Position, Width: img.Width, Height: img.Height, EventID: e.ID, ObjectKey: img.Key})
	}
	e.ApplyImages()
	return e, nil
}
