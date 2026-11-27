package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

var errDB = errors.New("db down")

func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return db, mock
}

func q(s string) string { return regexp.QuoteMeta(s) }

var eventCols = []string{"id", "title", "description", "category", "start_time", "end_time", "price",
	"venue", "address", "image_url", "artist_name", "perks", "is_saved", "lat", "lon"}

func addEventRow(rows *sqlmock.Rows, id string, extra ...driver.Value) *sqlmock.Rows {
	start := time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC)
	vals := []driver.Value{id, "Title " + id, nil, "Music", start, start.Add(2 * time.Hour), 10.0,
		"Venue", nil, nil, nil, "{a}", false, 43.0, -5.0}
	return rows.AddRow(append(vals, extra...)...)
}

func validEvent() *models.Event {
	return &models.Event{ID: "e1", Title: "T", Category: "Music", Date: "2026-11-27", StartTime: "20:00", EndTime: "22:00"}
}

// ---- events ----

func TestEventStoreList(t *testing.T) {
	t.Run("filter becomes query args", func(t *testing.T) {
		db, mock := newMock(t)
		s := &EventStoreDBConnection{db: db}
		mock.ExpectQuery(q("WHERE start_time::date >= $1::date AND start_time::date <= $2::date AND category = $3")).
			WithArgs("2026-11-01", "2026-11-30", "Music").
			WillReturnRows(addEventRow(addEventRow(sqlmock.NewRows(eventCols), "e1"), "e2"))
		got, err := s.List(EventFilter{From: "2026-11-01", To: "2026-11-30", Category: "Music"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].ID != "e1" || got[1].ID != "e2" || got[0].Date != "2026-11-27" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("query error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnError(errDB)
		if _, err := (&EventStoreDBConnection{db: db}).List(EventFilter{}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("scan error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("e1"))
		if _, err := (&EventStoreDBConnection{db: db}).List(EventFilter{}); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("row error", func(t *testing.T) {
		db, mock := newMock(t)
		rows := addEventRow(sqlmock.NewRows(eventCols), "e1").RowError(0, errDB)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)
		if _, err := (&EventStoreDBConnection{db: db}).List(EventFilter{}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestEventStoreGet(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{"found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q("FROM events WHERE id=$1")).WithArgs("e1").
				WillReturnRows(addEventRow(sqlmock.NewRows(eventCols), "e1"))
		}, nil},
		{"not found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(eventCols))
		}, ErrNotFound},
		{"db error", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(errDB)
		}, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			e, err := (&EventStoreDBConnection{db: db}).Get("e1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && e.ID != "e1" {
				t.Fatalf("e = %+v", e)
			}
		})
	}
}

func TestEventStoreCreate(t *testing.T) {
	start := time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		event   *models.Event
		setup   func(sqlmock.Sqlmock)
		wantErr error
		anyErr  bool
	}{
		{"ok", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectExec(q("INSERT INTO events")).
				WithArgs("e1", "T", "", models.EventCategory("Music"), start, start.Add(2*time.Hour),
					"", "", 0.0, "", "", sqlmock.AnyArg(), false, nil, nil).
				WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil, false},
		{"duplicate", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectExec("INSERT").WillReturnError(&pq.Error{Code: "23505"})
		}, ErrConflict, false},
		{"db error", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectExec("INSERT").WillReturnError(errDB)
		}, errDB, false},
		{"bad timestamps never hit the db", &models.Event{ID: "x", Date: "nope"}, func(sqlmock.Sqlmock) {}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			err := (&EventStoreDBConnection{db: db}).Create(tt.event)
			if tt.anyErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEventStoreUpdateDelete(t *testing.T) {
	type op func(*EventStoreDBConnection) error
	update := func(s *EventStoreDBConnection) error { return s.Update(validEvent()) }
	del := func(s *EventStoreDBConnection) error { return s.Delete("e1") }
	tests := []struct {
		name    string
		op      op
		setup   func(sqlmock.Sqlmock)
		wantErr error
		anyErr  bool
	}{
		{"update ok", update, func(m sqlmock.Sqlmock) {
			m.ExpectExec(q("UPDATE events SET")).WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil, false},
		{"update missing", update, func(m sqlmock.Sqlmock) {
			m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 0))
		}, ErrNotFound, false},
		{"update db error", update, func(m sqlmock.Sqlmock) {
			m.ExpectExec("UPDATE").WillReturnError(errDB)
		}, errDB, false},
		{"update bad timestamps", func(s *EventStoreDBConnection) error {
			return s.Update(&models.Event{ID: "e1"})
		}, func(sqlmock.Sqlmock) {}, nil, true},
		{"delete ok", del, func(m sqlmock.Sqlmock) {
			m.ExpectExec(q("DELETE FROM events WHERE id=$1")).WithArgs("e1").WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil, false},
		{"delete missing", del, func(m sqlmock.Sqlmock) {
			m.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 0))
		}, ErrNotFound, false},
		{"delete db error", del, func(m sqlmock.Sqlmock) {
			m.ExpectExec("DELETE").WillReturnError(errDB)
		}, errDB, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			err := tt.op(&EventStoreDBConnection{db: db})
			if tt.anyErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// ---- users ----

var userCols = []string{"id", "email", "name", "role", "subscription_type", "payment_method", "location", "event_preferences"}

func TestUserStoreListGet(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery(q("FROM users")).WillReturnRows(sqlmock.NewRows(userCols).
			AddRow("u1", "a@b.c", "Ana", "admin", "pro", "card", "{1,2}", "{Music}").
			AddRow("u2", "b@b.c", nil, "consumer", nil, nil, nil, nil))
		got, err := (&UserStoreDBConnection{db: db}).List()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Name != "Ana" || got[1].Name != "" || got[0].Location[1] != 2 {
			t.Fatalf("got %+v %+v", got[0], got[1])
		}
	})
	t.Run("list query error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnError(errDB)
		if _, err := (&UserStoreDBConnection{db: db}).List(); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("list scan error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("u1"))
		if _, err := (&UserStoreDBConnection{db: db}).List(); err == nil {
			t.Fatal("expected error")
		}
	})

	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{"get found with NULL columns", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q("FROM users WHERE id=$1")).WithArgs("u1").
				WillReturnRows(sqlmock.NewRows(userCols).AddRow("u1", "a@b.c", nil, "consumer", nil, nil, nil, nil))
		}, nil},
		{"get not found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(userCols))
		}, ErrNotFound},
		{"get malformed uuid", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(&pq.Error{Code: "22P02"})
		}, ErrNotFound},
		{"get db error", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(errDB)
		}, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			u, err := (&UserStoreDBConnection{db: db}).Get("u1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (u.ID != "u1" || u.Role != "consumer") {
				t.Fatalf("u = %+v", u)
			}
			if tt.wantErr != nil && u != nil {
				t.Fatalf("u = %+v, want nil", u)
			}
		})
	}
}

func TestUserStoreUpdate_SyncsBetterAuth(t *testing.T) {
	u := &models.User{ID: "u1", Email: "a@b.c", Name: "Ana", Role: "publisher",
		SubscriptionType: "pro", EventPreferences: []string{"Art"}}
	updateUsers := q("UPDATE users SET email=$2, name=$3, role=$4")
	updateAuth := q(`UPDATE auth."user" SET role=$2, name=$3, "updatedAt"=now() WHERE id=$1`)

	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{"both tables updated in one transaction", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(updateUsers).
				WithArgs("u1", "a@b.c", "Ana", "publisher", "pro", "", sqlmock.AnyArg(), sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(updateAuth).WithArgs("u1", "publisher", "Ana").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}, nil},
		{"begin fails", func(m sqlmock.Sqlmock) {
			m.ExpectBegin().WillReturnError(errDB)
		}, errDB},
		{"users update fails → rollback, auth untouched", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(updateUsers).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB},
		{"user missing → rollback, auth untouched", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(updateUsers).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectRollback()
		}, ErrNotFound},
		{"unique violation → conflict", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(updateUsers).WillReturnError(&pq.Error{Code: "23505"})
			m.ExpectRollback()
		}, ErrConflict},
		{"auth update fails → rollback", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(updateUsers).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(updateAuth).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB},
		{"commit fails", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(updateUsers).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(updateAuth).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit().WillReturnError(errDB)
		}, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			err := (&UserStoreDBConnection{db: db}).Update(u)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserStoreUpsert(t *testing.T) {
	t.Run("upsert", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectExec(q("INSERT INTO users (id, email, name, role)")+`(.|\n)*`+q("ON CONFLICT (id) DO UPDATE")).
			WithArgs("u1", "a@b.c", "Ana", "consumer").WillReturnResult(sqlmock.NewResult(0, 1))
		err := (&UserStoreDBConnection{db: db}).Upsert(context.Background(),
			&models.User{ID: "u1", Email: "a@b.c", Name: "Ana", Role: "consumer"})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("upsert error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectExec("INSERT").WillReturnError(errDB)
		if err := (&UserStoreDBConnection{db: db}).Upsert(context.Background(), &models.User{}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

}

// ---- saved events ----

var savedCols = append(append([]string{}, eventCols...), "user_id", "created_at")

func TestSavedStoreListGet(t *testing.T) {
	savedAt := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	t.Run("list groups by user preserving order", func(t *testing.T) {
		db, mock := newMock(t)
		rows := sqlmock.NewRows(savedCols)
		addEventRow(rows, "e1", "u1", savedAt)
		addEventRow(rows, "e2", "u2", savedAt)
		addEventRow(rows, "e3", "u1", nil)
		mock.ExpectQuery(q("FROM user_saved_events se INNER JOIN events e")).WillReturnRows(rows)
		got, err := (&UserSavedEventStoreDBConnection{db: db}).List()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].UserId != "u1" || len(got[0].SavedEvents) != 2 ||
			got[0].SavedEvents[1].ID != "e3" || got[1].UserId != "u2" ||
			got[0].SavedEvents[0].CreatedAt != "2026-11-01T10:00:00Z" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("list query error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnError(errDB)
		if _, err := (&UserSavedEventStoreDBConnection{db: db}).List(); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("list scan error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("x"))
		if _, err := (&UserSavedEventStoreDBConnection{db: db}).List(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("get", func(t *testing.T) {
		db, mock := newMock(t)
		rows := sqlmock.NewRows(savedCols)
		addEventRow(rows, "e1", "u1", savedAt)
		mock.ExpectQuery(q("WHERE se.user_id = $1")).WithArgs("u1").WillReturnRows(rows)
		got, err := (&UserSavedEventStoreDBConnection{db: db}).Get("u1")
		if err != nil || got.UserId != "u1" || len(got.SavedEvents) != 1 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("get none is empty slice", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(savedCols))
		got, err := (&UserSavedEventStoreDBConnection{db: db}).Get("u1")
		if err != nil || got.SavedEvents == nil || len(got.SavedEvents) != 0 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("get query error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnError(errDB)
		if _, err := (&UserSavedEventStoreDBConnection{db: db}).Get("u1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("get scan error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("x"))
		if _, err := (&UserSavedEventStoreDBConnection{db: db}).Get("u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSavedStoreWrites(t *testing.T) {
	se := &models.UserSavedEvent{ID: "s1", UserId: "u1", EventId: "e1"}
	create := func(s *UserSavedEventStoreDBConnection) error { return s.Create(se) }
	del := func(s *UserSavedEventStoreDBConnection) error { return s.Delete("u1", "e1") }
	delByID := func(s *UserSavedEventStoreDBConnection) error { return s.DeleteById("s1") }

	tests := []struct {
		name    string
		op      func(*UserSavedEventStoreDBConnection) error
		setup   func(sqlmock.Sqlmock)
		wantErr error
	}{
		{"create ok", create, func(m sqlmock.Sqlmock) {
			m.ExpectExec(q("INSERT INTO user_saved_events (id, user_id, event_id)")).
				WithArgs("s1", "u1", "e1").WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil},
		{"create duplicate", create, func(m sqlmock.Sqlmock) {
			m.ExpectExec("INSERT").WillReturnError(&pq.Error{Code: "23505"})
		}, ErrConflict},
		{"create unknown event", create, func(m sqlmock.Sqlmock) {
			m.ExpectExec("INSERT").WillReturnError(&pq.Error{Code: "23503"})
		}, ErrNotFound},
		{"create db error", create, func(m sqlmock.Sqlmock) {
			m.ExpectExec("INSERT").WillReturnError(errDB)
		}, errDB},
		{"delete ok", del, func(m sqlmock.Sqlmock) {
			m.ExpectExec(q("DELETE FROM user_saved_events WHERE user_id=$1 AND event_id=$2")).
				WithArgs("u1", "e1").WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil},
		{"delete missing", del, func(m sqlmock.Sqlmock) {
			m.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 0))
		}, ErrNotFound},
		{"delete db error", del, func(m sqlmock.Sqlmock) {
			m.ExpectExec("DELETE").WillReturnError(errDB)
		}, errDB},
		{"delete by id ok", delByID, func(m sqlmock.Sqlmock) {
			m.ExpectExec(q("DELETE FROM user_saved_events WHERE id=$1")).WithArgs("s1").
				WillReturnResult(sqlmock.NewResult(0, 1))
		}, nil},
		{"delete by id missing", delByID, func(m sqlmock.Sqlmock) {
			m.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 0))
		}, ErrNotFound},
		{"delete by id db error", delByID, func(m sqlmock.Sqlmock) {
			m.ExpectExec("DELETE").WillReturnError(errDB)
		}, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			if err := tt.op(&UserSavedEventStoreDBConnection{db: db}); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSavedStoreIsSaved(t *testing.T) {
	for _, tt := range []struct {
		name string
		rows *sqlmock.Rows
		err  error
		want bool
	}{
		{"saved", sqlmock.NewRows([]string{"exists"}).AddRow(true), nil, true},
		{"not saved", sqlmock.NewRows([]string{"exists"}).AddRow(false), nil, false},
		{"db error", nil, errDB, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			e := mock.ExpectQuery(q("SELECT EXISTS (SELECT 1 FROM user_saved_events WHERE user_id=$1 AND event_id=$2)")).
				WithArgs("u1", "e1")
			if tt.err != nil {
				e.WillReturnError(tt.err)
			} else {
				e.WillReturnRows(tt.rows)
			}
			got, err := (&UserSavedEventStoreDBConnection{db: db}).IsSaved("u1", "e1")
			if !errors.Is(err, tt.err) || got != tt.want {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}
