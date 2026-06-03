package store

import (
	"errors"
	"sync"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"golang.org/x/crypto/bcrypt"
)

// UserStore is the interface that any user persistence backend must satisfy.
// Swap MemoryUserStore for a PostgresUserStore in main.go without touching handler code.
type UserStore interface {
	List() ([]*models.User, error)
	Get(id string) (*models.User, error)
	Create(u *models.User) error
	Update(u *models.User) error
	Delete(id string) error
	Authenticate(username, password string) (*models.User, error)
}

type MemoryUserStore struct {
	mu    sync.RWMutex
	users map[string]*models.User
}

func NewMemoryUserStore() *MemoryUserStore {
	s := &MemoryUserStore{users: make(map[string]*models.User)}
	s.seed()
	return s
}

func safeUser(u *models.User) *models.User {
	c := *u
	c.Password = ""
	return &c
}

func (s *MemoryUserStore) List() ([]*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*models.User, 0, len(s.users))
	for _, u := range s.users {
		result = append(result, safeUser(u))
	}
	return result, nil
}

func (s *MemoryUserStore) Get(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return safeUser(u), nil
}

func hashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func (s *MemoryUserStore) Create(u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[u.ID]; exists {
		return errors.New("user already exists")
	}
	if u.Password != "" {
		hashed, err := hashPassword(u.Password)
		if err != nil {
			return err
		}
		u.Password = hashed
	}
	s.users[u.ID] = u
	return nil
}

func (s *MemoryUserStore) Update(u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.users[u.ID]
	if !ok {
		return ErrNotFound
	}
	if u.Password == "" {
		u.Password = existing.Password
	} else {
		hashed, err := hashPassword(u.Password)
		if err != nil {
			return err
		}
		u.Password = hashed
	}
	s.users[u.ID] = u
	return nil
}

func (s *MemoryUserStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.users, id)
	return nil
}

func (s *MemoryUserStore) Authenticate(username, password string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username != username {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
			return nil, ErrNotFound
		}
		return safeUser(u), nil
	}
	return nil, ErrNotFound
}

func (s *MemoryUserStore) seed() {
	users := []*models.User{
		{
			ID:       "1",
			Username: "admin",
			Email:    "admin@folixenda.com",
			Name:     "Admin",
			Role:     "admin",
			Password: "admin123",
		},
		{
			ID:               "2",
			Username:         "consumer",
			Email:            "consumer@example.com",
			Name:             "Consumer User",
			Role:             "consumer",
			Location:         []float64{43.393283, -5.654910},
			EventPreferences: []string{"Music", "Art"},
			Password:         "consumer123",
		},
		{
			ID:               "3",
			Username:         "publisher",
			Email:            "publisher@example.com",
			Name:             "Publisher User",
			Role:             "publisher",
			SubscriptionType: "basic",
			PaymentMethod:    "card",
			Password:         "publisher123",
		},
	}
	for _, u := range users {
		hashed, err := hashPassword(u.Password)
		if err != nil {
			panic("seed: failed to hash password: " + err.Error())
		}
		u.Password = hashed
		s.users[u.ID] = u
	}
}
