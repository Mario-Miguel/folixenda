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
	Upsert(ctx context.Context, u *models.User) error
}

type UserStoreDBConnection struct {
	db *sql.DB
}

// NewPostgresUserStore returns a store backed by db. The schema is created by the
// migrations in backend/migrations, not here.
func NewPostgresUserStore(db *sql.DB) *UserStoreDBConnection {
	return &UserStoreDBConnection{db: db}
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
	if err != nil {
		return nil, translateError(err)
	}
	return u, nil
}

// Update replaces the user's profile. Role and name are also written to
// Better Auth's auth."user" row in the same transaction: Better Auth is the
// source of the JWT claims, so without that the change would never reach the
// user's token (and Provision would revert it on the next upsert).
func (s *UserStoreDBConnection) Update(u *models.User) (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck
		}
	}()

	result, err := tx.Exec(`
			UPDATE users SET email=$2, name=$3, role=$4, subscription_type=$5,
			payment_method=$6, location=$7, event_preferences=$8 WHERE id=$1
		`, u.ID, u.Email, u.Name, u.Role, u.SubscriptionType, u.PaymentMethod,
		pq.Array(u.Location), pq.Array(u.EventPreferences))
	if err != nil {
		return translateError(err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}

	if _, err = tx.Exec(`
			UPDATE auth."user" SET role=$2, name=$3, "updatedAt"=now() WHERE id=$1
		`, u.ID, u.Role, u.Name); err != nil {
		return translateError(err)
	}
	return tx.Commit()
}

func (s *UserStoreDBConnection) Upsert(ctx context.Context, u *models.User) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE 
		SET email = EXCLUDED.email,
			name = EXCLUDED.name,
			role = EXCLUDED.role
	`, u.ID, u.Email, u.Name, u.Role)

	return err
}

// scanUser reads a users row; nullable text columns come back as "".
func scanUser(s scanner) (*models.User, error) {
	u := &models.User{}
	var name, subscriptionType, paymentMethod sql.NullString
	var location pq.Float64Array
	var prefs pq.StringArray
	if err := s.Scan(&u.ID, &u.Email, &name, &u.Role, &subscriptionType, &paymentMethod, &location, &prefs); err != nil {
		return nil, err
	}
	u.Name = name.String
	u.SubscriptionType = subscriptionType.String
	u.PaymentMethod = paymentMethod.String
	u.Location = []float64(location)
	u.EventPreferences = []string(prefs)
	return u, nil
}
