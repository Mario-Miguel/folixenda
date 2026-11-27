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
	IsSaved(userID, eventID string) (bool, error)
}

type UserSavedEventStoreDBConnection struct {
	db       *sql.DB
	imageURL URLFunc
}

// NewUserSavedEventsStore returns a store backed by db; imageURL builds the
// URLs of uploaded images (see NewEventStore). The schema is created by the
// migrations in backend/migrations, not here.
func NewUserSavedEventsStore(db *sql.DB, imageURL URLFunc) *UserSavedEventStoreDBConnection {
	return &UserSavedEventStoreDBConnection{db: db, imageURL: imageURL}
}

func (s *UserSavedEventStoreDBConnection) List() ([]*models.UserSavedEvents, error) {
	rows, err := s.db.Query(`
		SELECT ` + eventColumnsE + `,
			se.user_id,
			se.created_at
		FROM user_saved_events se INNER JOIN events e ON e.id = se.event_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.UserSavedEvents
	byUser := map[string]*models.UserSavedEvents{}
	for rows.Next() {
		userID, se, err := scanSavedEvent(rows, s.imageURL)
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
		SELECT `+eventColumnsE+`,
			se.user_id,
			se.created_at
		FROM user_saved_events se INNER JOIN events e ON e.id = se.event_id
		WHERE se.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &models.UserSavedEvents{UserId: userID, SavedEvents: []models.SavedEvent{}}
	for rows.Next() {
		_, se, err := scanSavedEvent(rows, s.imageURL)
		if err != nil {
			return nil, err
		}
		result.SavedEvents = append(result.SavedEvents, *se)
	}
	return result, rows.Err()
}

func (s *UserSavedEventStoreDBConnection) Create(se *models.UserSavedEvent) error {
	_, err := s.db.Exec(`
		INSERT INTO user_saved_events (id, user_id, event_id)
		VALUES ($1,$2,$3)
	`, se.ID, se.UserId, se.EventId)
	return translateError(err)
}

// IsSaved reports whether userID has saved eventID.
func (s *UserSavedEventStoreDBConnection) IsSaved(userID, eventID string) (bool, error) {
	var saved bool
	err := s.db.QueryRow(`
		SELECT EXISTS (SELECT 1 FROM user_saved_events WHERE user_id=$1 AND event_id=$2)
	`, userID, eventID).Scan(&saved)
	return saved, err
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
		return translateError(err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSavedEvent(s scanner, imageURL URLFunc) (userID string, se *models.SavedEvent, err error) {
	var createdAt sql.NullTime
	e, err := scanEvent(extraScanner{s: s, extra: []any{&userID, &createdAt}}, imageURL)
	if err != nil {
		return "", nil, err
	}
	se = &models.SavedEvent{Event: *e}
	if createdAt.Valid {
		se.CreatedAt = createdAt.Time.Format(time.RFC3339)
	}
	return userID, se, nil
}
