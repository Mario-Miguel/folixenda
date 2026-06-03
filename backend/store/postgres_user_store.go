package store

import (
	"database/sql"
	"errors"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) (*PostgresUserStore, error) {
	s := &PostgresUserStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PostgresUserStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id                TEXT PRIMARY KEY,
			username          TEXT UNIQUE NOT NULL,
			email             TEXT UNIQUE NOT NULL,
			password          TEXT NOT NULL,
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

func (s *PostgresUserStore) List() ([]*models.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, email, name, role, subscription_type, payment_method, location, event_preferences
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

func (s *PostgresUserStore) Get(id string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id, username, email, name, role, subscription_type, payment_method, location, event_preferences
		FROM users WHERE id=$1
	`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *PostgresUserStore) Create(u *models.User) error {
	if u.Password != "" {
		hashed, err := hashPassword(u.Password)
		if err != nil {
			return err
		}
		u.Password = hashed
	}
	_, err := s.db.Exec(`
		INSERT INTO users (id, username, email, name, role, subscription_type, payment_method, location, event_preferences, password)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, u.ID, u.Username, u.Email, u.Name, u.Role, u.SubscriptionType, u.PaymentMethod,
		pq.Array(u.Location), pq.Array(u.EventPreferences), u.Password)
	return err
}

func (s *PostgresUserStore) Update(u *models.User) error {
	var err error
	var result sql.Result
	if u.Password != "" {
		hashed, herr := hashPassword(u.Password)
		if herr != nil {
			return herr
		}
		result, err = s.db.Exec(`
			UPDATE users SET username=$2, email=$3, name=$4, role=$5, subscription_type=$6,
			payment_method=$7, location=$8, event_preferences=$9, password=$10 WHERE id=$1
		`, u.ID, u.Username, u.Email, u.Name, u.Role, u.SubscriptionType, u.PaymentMethod,
			pq.Array(u.Location), pq.Array(u.EventPreferences), hashed)
	} else {
		result, err = s.db.Exec(`
			UPDATE users SET username=$2, email=$3, name=$4, role=$5, subscription_type=$6,
			payment_method=$7, location=$8, event_preferences=$9 WHERE id=$1
		`, u.ID, u.Username, u.Email, u.Name, u.Role, u.SubscriptionType, u.PaymentMethod,
			pq.Array(u.Location), pq.Array(u.EventPreferences))
	}
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresUserStore) Delete(id string) error {
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

func (s *PostgresUserStore) Authenticate(username, password string) (*models.User, error) {
	var hashed string
	var location pq.Float64Array
	var prefs pq.StringArray
	u := &models.User{}
	err := s.db.QueryRow(`
		SELECT id, username, email, name, role, subscription_type, payment_method, location, event_preferences, password
		FROM users WHERE username=$1
	`, username).Scan(&u.ID, &u.Username, &u.Email, &u.Name, &u.Role, &u.SubscriptionType, &u.PaymentMethod, &location, &prefs, &hashed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
		return nil, ErrNotFound
	}
	u.Location = []float64(location)
	u.EventPreferences = []string(prefs)
	return u, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(s scanner) (*models.User, error) {
	u := &models.User{}
	var location pq.Float64Array
	var prefs pq.StringArray
	if err := s.Scan(&u.ID, &u.Username, &u.Email, &u.Name, &u.Role, &u.SubscriptionType, &u.PaymentMethod, &location, &prefs); err != nil {
		return nil, err
	}
	u.Location = []float64(location)
	u.EventPreferences = []string(prefs)
	return u, nil
}
