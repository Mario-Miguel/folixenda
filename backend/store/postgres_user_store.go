package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/lib/pq"
)

type UserStore interface {
	List() ([]*models.User, error)
	Get(id string) (*models.User, error)
	Update(u *models.User) error
	Delete(id string) error
	Upsert(ctx context.Context, u *models.User) error
}

type UserStoreDBConnection struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) (*UserStoreDBConnection, error) {
	s := &UserStoreDBConnection{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *UserStoreDBConnection) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id                TEXT PRIMARY KEY,
			email             TEXT UNIQUE NOT NULL,
			name              TEXT,
			role              TEXT NOT NULL,
			subscription_type TEXT,
			payment_method    TEXT,
			location          DOUBLE PRECISION[],
			event_preferences TEXT[]
		)
	`)
	return err
}

func (s *UserStoreDBConnection) List() ([]*models.User, error) {
	rows, err := s.db.Query(`
		SELECT id, email, name, role, subscription_type, payment_method, location, event_preferences
		FROM users
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserStoreDBConnection) Get(id string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id, email, name, role, subscription_type, payment_method, location, event_preferences
		FROM users WHERE id=$1
	`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *UserStoreDBConnection) Update(u *models.User) error {
	var err error
	var result sql.Result
	result, err = s.db.Exec(`
			UPDATE users SET email=$2, name=$3, role=$4, subscription_type=$5,
			payment_method=$6, location=$7, event_preferences=$8 WHERE id=$1
		`, u.ID, u.Email, u.Name, u.Role, u.SubscriptionType, u.PaymentMethod,
		pq.Array(u.Location), pq.Array(u.EventPreferences))
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *UserStoreDBConnection) Upsert(ctx context.Context, u *models.User) error {
	_, err := s.db.Exec(`
		INSERT INTO users (id, email, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE 
		SET email = EXCLUDED.email,
			name = EXCLUDED.name,
			role = EXCLUDED.role
	`, u.ID, u.Email, u.Name, u.Role)

	return err
}

func (s *UserStoreDBConnection) Delete(id string) error {
	result, err := s.db.Exec(`DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanUser(s scanner) (*models.User, error) {
	u := &models.User{}
	var location pq.Float64Array
	var prefs pq.StringArray
	if err := s.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.SubscriptionType, &u.PaymentMethod, &location, &prefs); err != nil {
		return nil, err
	}
	u.Location = []float64(location)
	u.EventPreferences = []string(prefs)
	return u, nil
}
