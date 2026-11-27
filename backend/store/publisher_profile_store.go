package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

type PublisherProfileStore interface {
	// Get returns the profile of userID, or ErrNotFound.
	Get(userID string) (*models.PublisherProfile, error)
	// Upsert creates or replaces the profile of p.UserID and returns the
	// stored row. Empty optional fields are stored as NULL.
	Upsert(p *models.PublisherProfile) (*models.PublisherProfile, error)
}

type PublisherProfileStoreDBConnection struct {
	db *sql.DB
}

// NewPublisherProfileStore returns a store backed by db. The schema is created
// by the migrations in backend/migrations, not here.
func NewPublisherProfileStore(db *sql.DB) *PublisherProfileStoreDBConnection {
	return &PublisherProfileStoreDBConnection{db: db}
}

const publisherProfileColumns = `user_id, org_name, website, phone, description, created_at, updated_at`

func (s *PublisherProfileStoreDBConnection) Get(userID string) (*models.PublisherProfile, error) {
	row := s.db.QueryRow(`
		SELECT `+publisherProfileColumns+`
		FROM publisher_profiles WHERE user_id=$1
	`, userID)
	p, err := scanPublisherProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, translateError(err)
	}
	return p, nil
}

// Upsert keeps created_at on updates and bumps updated_at. A missing users
// row (foreign key) comes back as ErrNotFound.
func (s *PublisherProfileStoreDBConnection) Upsert(p *models.PublisherProfile) (*models.PublisherProfile, error) {
	row := s.db.QueryRow(`
		INSERT INTO publisher_profiles (user_id, org_name, website, phone, description)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET org_name    = EXCLUDED.org_name,
		    website     = EXCLUDED.website,
		    phone       = EXCLUDED.phone,
		    description = EXCLUDED.description,
		    updated_at  = now()
		RETURNING `+publisherProfileColumns,
		p.UserID, p.OrgName, nullIfEmpty(p.Website), nullIfEmpty(p.Phone), nullIfEmpty(p.Description))
	saved, err := scanPublisherProfile(row)
	if err != nil {
		return nil, translateError(err)
	}
	return saved, nil
}

func scanPublisherProfile(s scanner) (*models.PublisherProfile, error) {
	p := &models.PublisherProfile{}
	var website, phone, description sql.NullString
	var createdAt, updatedAt time.Time
	if err := s.Scan(&p.UserID, &p.OrgName, &website, &phone, &description, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	p.Website = website.String
	p.Phone = phone.String
	p.Description = description.String
	p.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	p.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return p, nil
}
