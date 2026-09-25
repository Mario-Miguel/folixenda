package store

import (
	"database/sql"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

type UserSavedEventStore interface {
	List() ([]*models.UserSavedEvents, error)
	Get(userID string) (*models.UserSavedEvents, error)
	Create(se *models.UserSavedEvent) error
	Delete(userId string, eventId string) error
	DeleteById(id string) error
}

type UserSavedEventStoreDBConnection struct {
	db *sql.DB
}

func NewUserSavedEventsStore(db *sql.DB) (*UserSavedEventStoreDBConnection, error) {
	s := &UserSavedEventStoreDBConnection{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *UserSavedEventStoreDBConnection) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_saved_events (
			id                TEXT PRIMARY KEY,
			id_user           TEXT NOT NULL REFERENCES users(id),
			id_event          TEXT NOT NULL REFERENCES events(id),
			created_at		  TIMESTAMP DEFAULT now()
		)
	`)
	return err
}

func (s *UserSavedEventStoreDBConnection) List() ([]*models.UserSavedEvents, error) {
	rows, err := s.db.Query(`
		SELECT e.*, se.id_user, se.created_at
		FROM user_saved_events se INNER JOIN events e ON e.id = se.id_event
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.UserSavedEvents
	byUser := map[string]*models.UserSavedEvents{}
	for rows.Next() {
		userID, se, err := scanSavedEvent(rows)
		if err != nil {
			return nil, err
		}
		u, ok := byUser[userID]
		if !ok {
			u = &models.UserSavedEvents{UserId: userID, SavedEvents: []models.SavedEvent{}}
			byUser[userID] = u
			result = append(result, u)
		}
		u.SavedEvents = append(u.SavedEvents, *se)
	}
	return result, rows.Err()
}

// Get returns the events saved by a user; SavedEvents is empty (never nil) when there are none.
func (s *UserSavedEventStoreDBConnection) Get(userID string) (*models.UserSavedEvents, error) {
	rows, err := s.db.Query(`
		SELECT e.*, se.id_user, se.created_at
		FROM user_saved_events se INNER JOIN events e ON e.id = se.id_event
		WHERE se.id_user = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &models.UserSavedEvents{UserId: userID, SavedEvents: []models.SavedEvent{}}
	for rows.Next() {
		_, se, err := scanSavedEvent(rows)
		if err != nil {
			return nil, err
		}
		result.SavedEvents = append(result.SavedEvents, *se)
	}
	return result, rows.Err()
}

func (s *UserSavedEventStoreDBConnection) Create(se *models.UserSavedEvent) error {
	_, err := s.db.Exec(`
		INSERT INTO user_saved_events (id, id_user, id_event)
		VALUES ($1,$2,$3)
	`, se.ID, se.UserId, se.EventId)
	return err
}

func (s *UserSavedEventStoreDBConnection) DeleteById(id string) error {
	result, err := s.db.Exec(`DELETE FROM user_saved_events WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *UserSavedEventStoreDBConnection) Delete(userId string, eventId string) error {
	result, err := s.db.Exec(`DELETE FROM user_saved_events WHERE user_id=$1 AND event_id=$2`, userId, eventId)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSavedEvent(s scanner) (userID string, se *models.SavedEvent, err error) {
	var createdAt sql.NullTime
	e, err := scanEvent(extraScanner{s: s, extra: []any{&userID, &createdAt}})
	if err != nil {
		return "", nil, err
	}
	se = &models.SavedEvent{Event: *e}
	if createdAt.Valid {
		se.CreatedAt = createdAt.Time.Format(time.RFC3339)
	}
	return userID, se, nil
}
