package store

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

// TestEventStoreUpdate_NeverWritesPublisherID inspects the raw UPDATE SQL.
func TestEventStoreUpdate_NeverWritesPublisherID(t *testing.T) {
	var gotSQL string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
		func(_, actual string) error {
			if strings.HasPrefix(strings.TrimSpace(actual), "UPDATE events") {
				gotSQL = actual
			}
			return nil
		})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	e := validEvent()
	e.PublisherID = "intruder"
	if err := (&EventStoreDBConnection{db: db}).Update(e); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gotSQL, "publisher_id") || !strings.Contains(gotSQL, "updated_at=now()") {
		t.Fatalf("UPDATE sql = %s", gotSQL)
	}
}

// ---- publisher profiles ----

var profileCols = []string{"user_id", "org_name", "website", "phone", "description", "created_at", "updated_at"}

func TestPublisherProfileStoreGet(t *testing.T) {
	created := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		want    *models.PublisherProfile
		wantErr error
	}{
		{"found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q("SELECT "+publisherProfileColumns) + `\s+` + q("FROM publisher_profiles WHERE user_id=$1")).
				WithArgs("u1").WillReturnRows(sqlmock.NewRows(profileCols).
				AddRow("u1", "Acme", "https://acme.example", "+34 1", "desc", created, created.Add(time.Hour)))
		}, &models.PublisherProfile{UserID: "u1", OrgName: "Acme", Website: "https://acme.example", Phone: "+34 1",
			Description: "desc", CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-01T11:00:00Z"}, nil},
		{"NULL optionals", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(profileCols).
				AddRow("u1", "Acme", nil, nil, nil, created, created))
		}, &models.PublisherProfile{UserID: "u1", OrgName: "Acme",
			CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-01T10:00:00Z"}, nil},
		{"not found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(profileCols))
		}, nil, ErrNotFound},
		{"malformed uuid", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(&pq.Error{Code: "22P02"})
		}, nil, ErrNotFound},
		{"db error", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(errDB)
		}, nil, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			got, err := NewPublisherProfileStore(db).Get("u1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPublisherProfileStoreUpsert(t *testing.T) {
	created := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	upsert := q("INSERT INTO publisher_profiles (user_id, org_name, website, phone, description)") + `(.|\n)*` +
		q("ON CONFLICT (user_id) DO UPDATE") + `(.|\n)*` + q("updated_at  = now()") + `(.|\n)*` +
		q("RETURNING "+publisherProfileColumns)
	tests := []struct {
		name    string
		in      *models.PublisherProfile
		setup   func(sqlmock.Sqlmock)
		want    *models.PublisherProfile
		wantErr error
	}{
		{"full", &models.PublisherProfile{UserID: "u1", OrgName: "Acme", Website: "https://a.example", Phone: "1", Description: "d"},
			func(m sqlmock.Sqlmock) {
				m.ExpectQuery(upsert).WithArgs("u1", "Acme", "https://a.example", "1", "d").
					WillReturnRows(sqlmock.NewRows(profileCols).
						AddRow("u1", "Acme", "https://a.example", "1", "d", created, created.Add(time.Minute)))
			}, &models.PublisherProfile{UserID: "u1", OrgName: "Acme", Website: "https://a.example", Phone: "1",
				Description: "d", CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-01T10:01:00Z"}, nil},
		{"empty optionals sent as NULL", &models.PublisherProfile{UserID: "u1", OrgName: "Acme"},
			func(m sqlmock.Sqlmock) {
				m.ExpectQuery(upsert).WithArgs("u1", "Acme", nil, nil, nil).
					WillReturnRows(sqlmock.NewRows(profileCols).AddRow("u1", "Acme", nil, nil, nil, created, created))
			}, &models.PublisherProfile{UserID: "u1", OrgName: "Acme",
				CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-01T10:00:00Z"}, nil},
		{"missing user (fk)", &models.PublisherProfile{UserID: "u1", OrgName: "Acme"}, func(m sqlmock.Sqlmock) {
			m.ExpectQuery("INSERT").WillReturnError(&pq.Error{Code: "23503"})
		}, nil, ErrNotFound},
		{"db error", &models.PublisherProfile{UserID: "u1", OrgName: "Acme"}, func(m sqlmock.Sqlmock) {
			m.ExpectQuery("INSERT").WillReturnError(errDB)
		}, nil, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			got, err := NewPublisherProfileStore(db).Upsert(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// ---- notification preferences ----

var prefsCols = []string{"saved_event_reminders", "category_news", "categories", "newsletter", "publisher_activity"}

func TestNotificationPreferencesStoreGet(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		want    *models.NotificationPreferences
		wantErr error
	}{
		{"found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q("SELECT "+notificationPreferencesColumns) + `\s+` +
				q("FROM notification_preferences WHERE user_id=$1")).WithArgs("u1").
				WillReturnRows(sqlmock.NewRows(prefsCols).AddRow(false, true, "{Music,Art}", true, false))
		}, &models.NotificationPreferences{CategoryNews: true, Newsletter: true,
			Categories: []models.EventCategory{"Music", "Art"}}, nil},
		{"empty categories are [] not nil", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(prefsCols).AddRow(true, false, "{}", false, true))
		}, &models.NotificationPreferences{SavedEventReminders: true, PublisherActivity: true,
			Categories: []models.EventCategory{}}, nil},
		{"not found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(prefsCols))
		}, nil, ErrNotFound},
		{"malformed uuid", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(&pq.Error{Code: "22P02"})
		}, nil, ErrNotFound},
		{"db error", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(errDB)
		}, nil, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			got, err := NewNotificationPreferencesStore(db).Get("u1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNotificationPreferencesStoreUpsert(t *testing.T) {
	upsert := q("INSERT INTO notification_preferences") + `(.|\n)*` + q("ON CONFLICT (user_id) DO UPDATE") +
		`(.|\n)*` + q("updated_at            = now()") + `(.|\n)*` + q("RETURNING "+notificationPreferencesColumns)
	in := &models.NotificationPreferences{SavedEventReminders: true, CategoryNews: true,
		Categories: []models.EventCategory{"Music", "Food"}, Newsletter: false, PublisherActivity: true}
	tests := []struct {
		name    string
		in      *models.NotificationPreferences
		setup   func(sqlmock.Sqlmock)
		want    *models.NotificationPreferences
		wantErr error
	}{
		{"saved", in, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(upsert).WithArgs("u1", true, true, `{"Music","Food"}`, false, true).
				WillReturnRows(sqlmock.NewRows(prefsCols).AddRow(true, true, "{Music,Food}", false, true))
		}, in, nil},
		{"no categories sent as empty array", &models.NotificationPreferences{}, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(upsert).WithArgs("u1", false, false, `{}`, false, false).
				WillReturnRows(sqlmock.NewRows(prefsCols).AddRow(false, false, "{}", false, false))
		}, &models.NotificationPreferences{Categories: []models.EventCategory{}}, nil},
		{"missing user (fk)", in, func(m sqlmock.Sqlmock) {
			m.ExpectQuery("INSERT").WillReturnError(&pq.Error{Code: "23503"})
		}, nil, ErrNotFound},
		{"db error", in, func(m sqlmock.Sqlmock) {
			m.ExpectQuery("INSERT").WillReturnError(errDB)
		}, nil, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			got, err := NewNotificationPreferencesStore(db).Upsert("u1", tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestConstructors(t *testing.T) {
	db, _ := newMock(t)
	if NewEventStore(db, nil).db != db || NewPostgresUserStore(db).db != db || NewUserSavedEventsStore(db, nil).db != db ||
		NewPublisherProfileStore(db).db != db || NewNotificationPreferencesStore(db).db != db {
		t.Fatal("constructors must keep the db")
	}
}
