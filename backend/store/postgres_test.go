package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
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
	"venue", "address", "image_url", "artist_name", "perks", "is_saved", "lat", "lon", "publisher_id", "sessions", "images"}

func addEventRow(rows *sqlmock.Rows, id string, extra ...driver.Value) *sqlmock.Rows {
	return addOwnedEventRow(rows, id, nil, extra...)
}

// addOwnedEventRow adds an events row whose publisher_id is publisherID (nil = scraped).
func addOwnedEventRow(rows *sqlmock.Rows, id string, publisherID driver.Value, extra ...driver.Value) *sqlmock.Rows {
	start := time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC)
	vals := []driver.Value{id, "Title " + id, nil, "Music", start, start.Add(2 * time.Hour), 10.0,
		"Venue", nil, nil, nil, "{a}", false, 43.0, -5.0, publisherID,
		[]byte(`[{"date":"2026-11-27","startTime":"20:00","endTime":"22:00"}]`), []byte(`[]`)}
	return rows.AddRow(append(vals, extra...)...)
}

func validEvent() *models.Event {
	return &models.Event{ID: "e1", Title: "T", Category: "Music",
		Sessions: []models.EventSession{{Date: "2026-11-27", StartTime: "20:00", EndTime: "22:00"}}}
}

var (
	deleteSessions = q("DELETE FROM event_sessions WHERE event_id=$1")
	insertSessions = q("INSERT INTO event_sessions (event_id, start_time, end_time)") + `\s+` +
		q("SELECT $1, t.s, t.e FROM unnest($2::timestamp[], $3::timestamp[]) AS t(s, e)")
)

// expectSessions expects the delete + insert that replace an event's sessions.
func expectSessions(m sqlmock.Sqlmock, id string, starts, ends []string) {
	m.ExpectExec(deleteSessions).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(insertSessions).WithArgs(id, pq.StringArray(starts), pq.StringArray(ends)).
		WillReturnResult(sqlmock.NewResult(0, int64(len(starts))))
}

// ---- events ----

func TestEventStoreList(t *testing.T) {
	t.Run("filter becomes query args", func(t *testing.T) {
		db, mock := newMock(t)
		s := &EventStoreDBConnection{db: db}
		mock.ExpectQuery(q("WHERE s.event_id = e.id AND s.start_time >= $1::date AND s.start_time < $2::date + 1) m ON m.first_start IS NOT NULL")+
			`\s+`+q("WHERE e.category = $3")+`\s+`+q("ORDER BY m.first_start, e.id")).
			WithArgs("2026-11-01", "2026-11-30", "Music").
			WillReturnRows(addEventRow(addEventRow(sqlmock.NewRows(eventCols), "e1"), "e2"))
		got, err := s.List(EventFilter{From: "2026-11-01", To: "2026-11-30", Category: "Music"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].ID != "e1" || got[1].ID != "e2" || got[0].Date != "2026-11-27" ||
			len(got[0].Sessions) != 1 || got[0].Images == nil {
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

var getEventCols = append(append([]string{}, eventCols...), "org_name", "website", "phone", "description")

func TestEventStoreGet(t *testing.T) {
	getQuery := q("FROM events e\n\t\tLEFT JOIN publisher_profiles pp ON pp.user_id = e.publisher_id\n\t\tWHERE e.id=$1")
	tests := []struct {
		name          string
		setup         func(sqlmock.Sqlmock)
		wantErr       error
		wantPublisher string
		wantOrganizer *models.Organizer
	}{
		{"scraped event: no publisher, no organizer", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(q("SELECT " + eventColumnsE + ",")).WithArgs("e1").
				WillReturnRows(addEventRow(sqlmock.NewRows(getEventCols), "e1", nil, nil, nil, nil))
		}, nil, "", nil},
		{"publisher event with profile → organizer", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(getQuery).WithArgs("e1").
				WillReturnRows(addOwnedEventRow(sqlmock.NewRows(getEventCols), "e1", "pub-1",
					"Acme", "https://acme.example", nil, "We host"))
		}, nil, "pub-1", &models.Organizer{ID: "pub-1", OrgName: "Acme", Website: "https://acme.example", Description: "We host"}},
		{"publisher event without profile → no organizer", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(getQuery).WithArgs("e1").
				WillReturnRows(addOwnedEventRow(sqlmock.NewRows(getEventCols), "e1", "pub-1", nil, nil, nil, nil))
		}, nil, "pub-1", nil},
		{"not found", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(getEventCols))
		}, ErrNotFound, "", nil},
		{"db error", func(m sqlmock.Sqlmock) {
			m.ExpectQuery("SELECT").WillReturnError(errDB)
		}, errDB, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			e, err := (&EventStoreDBConnection{db: db}).Get("e1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if e != nil {
					t.Fatalf("e = %+v, want nil", e)
				}
				return
			}
			if e.ID != "e1" || e.PublisherID != tt.wantPublisher {
				t.Fatalf("e = %+v", e)
			}
			if !reflect.DeepEqual(e.Organizer, tt.wantOrganizer) {
				t.Fatalf("organizer = %+v, want %+v", e.Organizer, tt.wantOrganizer)
			}
		})
	}
}

func TestEventStoreListByPublisher(t *testing.T) {
	cols := append(append([]string{}, eventCols...), "count")
	t.Run("one grouped query with save counts", func(t *testing.T) {
		db, mock := newMock(t)
		rows := sqlmock.NewRows(cols)
		addOwnedEventRow(rows, "e1", "pub-1", int64(3))
		addOwnedEventRow(rows, "e2", "pub-1", int64(0))
		mock.ExpectQuery(q("SELECT "+eventColumnsE+", COUNT(se.id)") + `\s+` +
			q("FROM events e") + `\s+` +
			q("LEFT JOIN user_saved_events se ON se.event_id = e.id") + `\s+` +
			q("WHERE e.publisher_id = $1") + `\s+` + q("GROUP BY e.id") + `\s+` + q("ORDER BY e.start_time, e.id")).
			WithArgs("pub-1").WillReturnRows(rows)
		got, err := (&EventStoreDBConnection{db: db}).ListByPublisher("pub-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].ID != "e1" || got[0].SavesCount != 3 || got[1].SavesCount != 0 ||
			got[0].PublisherID != "pub-1" || got[0].Date != "2026-11-27" {
			t.Fatalf("got %+v %+v", got[0], got[1])
		}
	})
	t.Run("none is empty slice", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(cols))
		got, err := (&EventStoreDBConnection{db: db}).ListByPublisher("pub-1")
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %v err %v", got, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnError(errDB)
		if _, err := (&EventStoreDBConnection{db: db}).ListByPublisher("pub-1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("scan error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("e1"))
		if _, err := (&EventStoreDBConnection{db: db}).ListByPublisher("pub-1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("row error", func(t *testing.T) {
		db, mock := newMock(t)
		rows := addOwnedEventRow(sqlmock.NewRows(cols), "e1", "pub-1", int64(1)).RowError(0, errDB)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)
		if _, err := (&EventStoreDBConnection{db: db}).ListByPublisher("pub-1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestEventStoreCreate(t *testing.T) {
	start := time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC)
	one := []string{"2026-11-27 20:00:00"}
	oneEnd := []string{"2026-11-27 22:00:00"}
	insertEvent := q("INSERT INTO events") + `(.|\n)*` + q("lat, lon, publisher_id)")
	tests := []struct {
		name    string
		event   *models.Event
		setup   func(sqlmock.Sqlmock)
		wantErr error
		anyErr  bool
	}{
		{"ok, unowned → NULL publisher_id, sessions replaced in the same tx", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(insertEvent).
				WithArgs("e1", "T", "", models.EventCategory("Music"), start, start.Add(2*time.Hour),
					"", "", 0.0, "", "", sqlmock.AnyArg(), false, nil, nil, nil).
				WillReturnResult(sqlmock.NewResult(0, 1))
			expectSessions(m, "e1", one, oneEnd)
			m.ExpectCommit()
		}, nil, false},
		{"ok, owned, stored image url, multi-day span", func() *models.Event {
			e := validEvent()
			e.PublisherID, e.StoredImageURL = "pub-1", "https://img"
			e.Sessions = append(e.Sessions, models.EventSession{Date: "2026-11-29", StartTime: "22:00", EndTime: "02:00"})
			return e
		}(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(q("INSERT INTO events")).
				WithArgs("e1", "T", "", models.EventCategory("Music"), start, time.Date(2026, 11, 30, 2, 0, 0, 0, time.UTC),
					"", "", 0.0, "https://img", "", sqlmock.AnyArg(), false, nil, nil, "pub-1").
				WillReturnResult(sqlmock.NewResult(0, 1))
			expectSessions(m, "e1", []string{"2026-11-27 20:00:00", "2026-11-29 22:00:00"},
				[]string{"2026-11-27 22:00:00", "2026-11-30 02:00:00"})
			m.ExpectCommit()
		}, nil, false},
		{"overnight end is stored on the next day",
			func() *models.Event {
				e := validEvent()
				e.Sessions[0].StartTime, e.Sessions[0].EndTime = "22:00", "02:00"
				return e
			}(),
			func(m sqlmock.Sqlmock) {
				night := time.Date(2026, 11, 27, 22, 0, 0, 0, time.UTC)
				m.ExpectBegin()
				m.ExpectExec(q("INSERT INTO events")).
					WithArgs("e1", "T", "", models.EventCategory("Music"), night, night.Add(4*time.Hour),
						"", "", 0.0, "", "", sqlmock.AnyArg(), false, nil, nil, nil).
					WillReturnResult(sqlmock.NewResult(0, 1))
				expectSessions(m, "e1", []string{"2026-11-27 22:00:00"}, []string{"2026-11-28 02:00:00"})
				m.ExpectCommit()
			}, nil, false},
		{"unknown publisher", func() *models.Event { e := validEvent(); e.PublisherID = "nobody"; return e }(),
			func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec("INSERT").WillReturnError(&pq.Error{Code: "23503"})
				m.ExpectRollback()
			}, ErrNotFound, false},
		{"duplicate", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("INSERT").WillReturnError(&pq.Error{Code: "23505"})
			m.ExpectRollback()
		}, ErrConflict, false},
		{"db error", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("INSERT").WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, false},
		{"begin error", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin().WillReturnError(errDB)
		}, errDB, false},
		{"delete sessions error → rollback", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(deleteSessions).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, false},
		{"insert sessions error → rollback", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(deleteSessions).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(insertSessions).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, false},
		{"commit error", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(0, 1))
			expectSessions(m, "e1", one, oneEnd)
			m.ExpectCommit().WillReturnError(errDB)
		}, errDB, false},
		{"bad sessions never hit the db", &models.Event{ID: "x", Sessions: []models.EventSession{{Date: "nope"}}},
			func(sqlmock.Sqlmock) {}, nil, true},
		{"no sessions never hit the db", &models.Event{ID: "x"}, func(sqlmock.Sqlmock) {}, nil, true},
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

func TestEventStoreUpdate(t *testing.T) {
	start := time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC)
	updateSQL := q("UPDATE events SET") + `(.|\n)*` + q("lat=$14, lon=$15, updated_at=now()") + `\s+` + q("WHERE id=$1")
	tests := []struct {
		name    string
		event   *models.Event
		setup   func(sqlmock.Sqlmock)
		wantErr error
		anyErr  bool
	}{
		{"ok: bumps updated_at, never touches publisher_id, replaces sessions", func() *models.Event {
			e := validEvent()
			e.PublisherID = "intruder"
			return e
		}(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(updateSQL).
				WithArgs("e1", "T", "", models.EventCategory("Music"), start, start.Add(2*time.Hour),
					"", "", 0.0, "", "", sqlmock.AnyArg(), false, nil, nil).
				WillReturnResult(sqlmock.NewResult(0, 1))
			expectSessions(m, "e1", []string{"2026-11-27 20:00:00"}, []string{"2026-11-27 22:00:00"})
			m.ExpectCommit()
		}, nil, false},
		{"missing", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectRollback()
		}, ErrNotFound, false},
		{"db error", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("UPDATE").WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, false},
		{"sessions error → rollback", validEvent(), func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(deleteSessions).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, false},
		{"bad sessions", &models.Event{ID: "e1"}, func(sqlmock.Sqlmock) {}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			err := (&EventStoreDBConnection{db: db}).Update(tt.event)
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

func TestEventStoreDelete(t *testing.T) {
	lock := q("SELECT id FROM events WHERE id=$1 FOR UPDATE")
	keys := q("SELECT COALESCE(array_agg(object_key), '{}') FROM event_images WHERE event_id=$1")
	del := q("DELETE FROM events WHERE id=$1")
	tests := []struct {
		name     string
		setup    func(sqlmock.Sqlmock)
		wantKeys []string
		wantErr  error
	}{
		{"ok returns the image keys", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lock).WithArgs("e1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("e1"))
			m.ExpectQuery(keys).WithArgs("e1").WillReturnRows(sqlmock.NewRows([]string{"k"}).AddRow("{a,b}"))
			m.ExpectExec(del).WithArgs("e1").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}, []string{"a", "b"}, nil},
		{"missing", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lock).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectRollback()
		}, nil, ErrNotFound},
		{"lock error", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lock).WillReturnError(errDB)
			m.ExpectRollback()
		}, nil, errDB},
		{"keys error", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lock).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("e1"))
			m.ExpectQuery(keys).WillReturnError(errDB)
			m.ExpectRollback()
		}, nil, errDB},
		{"delete error", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lock).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("e1"))
			m.ExpectQuery(keys).WillReturnRows(sqlmock.NewRows([]string{"k"}).AddRow("{}"))
			m.ExpectExec(del).WillReturnError(errDB)
			m.ExpectRollback()
		}, nil, errDB},
		{"commit error", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lock).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("e1"))
			m.ExpectQuery(keys).WillReturnRows(sqlmock.NewRows([]string{"k"}).AddRow("{}"))
			m.ExpectExec(del).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit().WillReturnError(errDB)
		}, nil, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMock(t)
			tt.setup(mock)
			got, err := (&EventStoreDBConnection{db: db}).Delete("e1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.wantKeys) {
				t.Fatalf("keys = %v, want %v", got, tt.wantKeys)
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
