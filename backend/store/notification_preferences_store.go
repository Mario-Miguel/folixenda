package store

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

type NotificationPreferencesStore interface {
	// Get returns the stored preferences of userID, or ErrNotFound when the
	// user never saved any (callers fall back to the defaults).
	Get(userID string) (*models.NotificationPreferences, error)
	// Upsert creates or replaces the preferences of userID and returns the
	// stored values.
	Upsert(userID string, p *models.NotificationPreferences) (*models.NotificationPreferences, error)
}

type NotificationPreferencesStoreDBConnection struct {
	db *sql.DB
}

// NewNotificationPreferencesStore returns a store backed by db. The schema is
// created by the migrations in backend/migrations, not here.
func NewNotificationPreferencesStore(db *sql.DB) *NotificationPreferencesStoreDBConnection {
	return &NotificationPreferencesStoreDBConnection{db: db}
}

const notificationPreferencesColumns = `saved_event_reminders, category_news, categories, newsletter, publisher_activity`

func (s *NotificationPreferencesStoreDBConnection) Get(userID string) (*models.NotificationPreferences, error) {
	row := s.db.QueryRow(`
		SELECT `+notificationPreferencesColumns+`
		FROM notification_preferences WHERE user_id=$1
	`, userID)
	p, err := scanNotificationPreferences(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, translateError(err)
	}
	return p, nil
}

// Upsert bumps updated_at. A missing users row (foreign key) comes back as
// ErrNotFound.
func (s *NotificationPreferencesStoreDBConnection) Upsert(userID string, p *models.NotificationPreferences) (*models.NotificationPreferences, error) {
	categories := make([]string, len(p.Categories))
	for i, c := range p.Categories {
		categories[i] = string(c)
	}
	row := s.db.QueryRow(`
		INSERT INTO notification_preferences
		       (user_id, saved_event_reminders, category_news, categories, newsletter, publisher_activity)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE
		SET saved_event_reminders = EXCLUDED.saved_event_reminders,
		    category_news         = EXCLUDED.category_news,
		    categories            = EXCLUDED.categories,
		    newsletter            = EXCLUDED.newsletter,
		    publisher_activity    = EXCLUDED.publisher_activity,
		    updated_at            = now()
		RETURNING `+notificationPreferencesColumns,
		userID, p.SavedEventReminders, p.CategoryNews, pq.Array(categories), p.Newsletter, p.PublisherActivity)
	saved, err := scanNotificationPreferences(row)
	if err != nil {
		return nil, translateError(err)
	}
	return saved, nil
}

// scanNotificationPreferences always returns a non-nil Categories slice.
func scanNotificationPreferences(s scanner) (*models.NotificationPreferences, error) {
	p := &models.NotificationPreferences{}
	var categories pq.StringArray
	if err := s.Scan(&p.SavedEventReminders, &p.CategoryNews, &categories, &p.Newsletter, &p.PublisherActivity); err != nil {
		return nil, err
	}
	p.Categories = make([]models.EventCategory, len(categories))
	for i, c := range categories {
		p.Categories[i] = models.EventCategory(c)
	}
	return p, nil
}
