package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

var (
	lockSQL       = q("SELECT id FROM events WHERE id=$1 FOR UPDATE")
	deleteCover   = q("DELETE FROM event_images WHERE event_id=$1 AND kind='cover' RETURNING id, object_key")
	galleryStats  = q("SELECT COUNT(*), COALESCE(MAX(position) + 1, 0) FROM event_images")
	insertImage   = q("INSERT INTO event_images (id, event_id, object_key, kind, position, content_type,")
	deleteImage   = q("DELETE FROM event_images WHERE event_id=$1 AND id=$2 RETURNING object_key, kind")
	compactSQL    = q(compactGallerySQL)
	galleryIDs    = q("SELECT COALESCE(array_agg(id), '{}') FROM event_images")
	reorderSQL    = q("UPDATE event_images SET position = t.pos - 1")
	listImagesSQL = q("SELECT id, object_key, kind, position, width, height FROM event_images")
)

func locked(m sqlmock.Sqlmock) {
	m.ExpectBegin()
	m.ExpectQuery(lockSQL).WithArgs("e1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("e1"))
}

func imgStore(t *testing.T, setup func(sqlmock.Sqlmock)) *EventStoreDBConnection {
	db, mock := newMock(t)
	setup(mock)
	return NewEventStore(db, cdn)
}

func newImage(kind string) *models.EventImage {
	w, h := 640, 480
	return &models.EventImage{ID: "i1", EventID: "e1", ObjectKey: "events/e1/i1.png", Kind: kind,
		ContentType: "image/png", SizeBytes: 123, Width: &w, Height: &h}
}

func TestAddImage(t *testing.T) {
	tests := []struct {
		name         string
		kind         string
		setup        func(sqlmock.Sqlmock)
		wantErr      error
		wantPos      int
		wantReplaced *models.EventImage
	}{
		{"first cover", models.ImageKindCover, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteCover).WithArgs("e1").WillReturnRows(sqlmock.NewRows([]string{"id", "object_key"}))
			m.ExpectExec(insertImage).WithArgs("i1", "e1", "events/e1/i1.png", "cover", 0, "image/png", int64(123), 640, 480).
				WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}, nil, 0, nil},
		{"cover replaces the old one", models.ImageKindCover, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteCover).WillReturnRows(sqlmock.NewRows([]string{"id", "object_key"}).AddRow("old", "events/e1/old.jpg"))
			m.ExpectExec(insertImage).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}, nil, 0, &models.EventImage{ID: "old", EventID: "e1", Kind: "cover", ObjectKey: "events/e1/old.jpg"}},
		{"cover delete error", models.ImageKindCover, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteCover).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, 0, nil},
		{"gallery goes last", models.ImageKindGallery, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryStats).WithArgs("e1").WillReturnRows(sqlmock.NewRows([]string{"c", "p"}).AddRow(3, 3))
			m.ExpectExec(insertImage).WithArgs("i1", "e1", "events/e1/i1.png", "gallery", 3, "image/png", int64(123), 640, 480).
				WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}, nil, 3, nil},
		{"gallery full", models.ImageKindGallery, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryStats).WillReturnRows(sqlmock.NewRows([]string{"c", "p"}).AddRow(10, 10))
			m.ExpectRollback()
		}, ErrGalleryFull, 0, nil},
		{"gallery stats error", models.ImageKindGallery, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryStats).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, 0, nil},
		{"event missing", models.ImageKindGallery, func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lockSQL).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectRollback()
		}, ErrNotFound, 0, nil},
		{"lock error", models.ImageKindGallery, func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lockSQL).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, 0, nil},
		{"insert conflict", models.ImageKindCover, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteCover).WillReturnRows(sqlmock.NewRows([]string{"id", "object_key"}))
			m.ExpectExec(insertImage).WillReturnError(&pq.Error{Code: "23505"})
			m.ExpectRollback()
		}, ErrConflict, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := newImage(tt.kind)
			replaced, err := imgStore(t, tt.setup).AddImage(img, 10)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(replaced, tt.wantReplaced) {
				t.Fatalf("replaced = %+v, want %+v", replaced, tt.wantReplaced)
			}
			if err == nil && (img.Position != tt.wantPos || img.URL != "https://cdn/events/e1/i1.png") {
				t.Fatalf("img = %+v", img)
			}
		})
	}
}

func TestDeleteImage(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr error
		want    *models.EventImage
	}{
		{"gallery image: positions compacted", func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteImage).WithArgs("e1", "i1").
				WillReturnRows(sqlmock.NewRows([]string{"k", "kind"}).AddRow("events/e1/i1.png", "gallery"))
			m.ExpectExec(compactSQL).WithArgs("e1").WillReturnResult(sqlmock.NewResult(0, 2))
			m.ExpectCommit()
		}, nil, &models.EventImage{ID: "i1", EventID: "e1", Kind: "gallery", ObjectKey: "events/e1/i1.png",
			URL: "https://cdn/events/e1/i1.png"}},
		{"cover: nothing to compact", func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteImage).WillReturnRows(sqlmock.NewRows([]string{"k", "kind"}).AddRow("k", "cover"))
			m.ExpectCommit()
		}, nil, &models.EventImage{ID: "i1", EventID: "e1", Kind: "cover", ObjectKey: "k", URL: "https://cdn/k"}},
		{"no such image", func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteImage).WillReturnRows(sqlmock.NewRows([]string{"k", "kind"}))
			m.ExpectRollback()
		}, ErrNotFound, nil},
		{"event missing", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lockSQL).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectRollback()
		}, ErrNotFound, nil},
		{"delete error", func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteImage).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, nil},
		{"compact error", func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(deleteImage).WillReturnRows(sqlmock.NewRows([]string{"k", "kind"}).AddRow("k", "gallery"))
			m.ExpectExec(compactSQL).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := imgStore(t, tt.setup).DeleteImage("e1", "i1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReorderImages(t *testing.T) {
	imgCols := []string{"id", "object_key", "kind", "position", "width", "height"}
	tests := []struct {
		name    string
		ids     []string
		setup   func(sqlmock.Sqlmock)
		wantErr error
		wantIDs []string
	}{
		{"ok", []string{"g2", "g1"}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WithArgs("e1").WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{g1,g2}"))
			m.ExpectExec(reorderSQL).WithArgs("e1", pq.StringArray{"g2", "g1"}).WillReturnResult(sqlmock.NewResult(0, 2))
			m.ExpectQuery(listImagesSQL).WithArgs("e1").WillReturnRows(sqlmock.NewRows(imgCols).
				AddRow("c", "kc", "cover", 0, 100, 50).
				AddRow("g2", "k2", "gallery", 0, nil, nil).
				AddRow("g1", "k1", "gallery", 1, nil, nil))
			m.ExpectCommit()
		}, nil, []string{"c", "g2", "g1"}},
		{"missing id", []string{"g1"}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{g1,g2}"))
			m.ExpectRollback()
		}, ErrInvalidInput, nil},
		{"duplicate id", []string{"g1", "g1"}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{g1,g2}"))
			m.ExpectRollback()
		}, ErrInvalidInput, nil},
		{"unknown id", []string{"g1", "zz"}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{g1,g2}"))
			m.ExpectRollback()
		}, ErrInvalidInput, nil},
		{"empty gallery, empty list", []string{}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{}"))
			m.ExpectExec(reorderSQL).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(listImagesSQL).WillReturnRows(sqlmock.NewRows(imgCols))
			m.ExpectCommit()
		}, nil, []string{}},
		{"event missing", []string{}, func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(lockSQL).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectRollback()
		}, ErrNotFound, nil},
		{"ids query error", []string{}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, nil},
		{"update error", []string{}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{}"))
			m.ExpectExec(reorderSQL).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, nil},
		{"list error", []string{}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{}"))
			m.ExpectExec(reorderSQL).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(listImagesSQL).WillReturnError(errDB)
			m.ExpectRollback()
		}, errDB, nil},
		{"scan error", []string{}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{}"))
			m.ExpectExec(reorderSQL).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(listImagesSQL).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("x"))
			m.ExpectRollback()
		}, nil, nil},
		{"row error", []string{}, func(m sqlmock.Sqlmock) {
			locked(m)
			m.ExpectQuery(galleryIDs).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow("{}"))
			m.ExpectExec(reorderSQL).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(listImagesSQL).WillReturnRows(sqlmock.NewRows(imgCols).AddRow("c", "k", "cover", 0, nil, nil).RowError(0, errDB))
			m.ExpectRollback()
		}, errDB, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := imgStore(t, tt.setup).ReorderImages("e1", tt.ids)
			if tt.name == "scan error" {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != nil {
					t.Fatalf("got %+v", got)
				}
				return
			}
			var ids []string
			for _, img := range got {
				ids = append(ids, img.ID)
			}
			if got == nil || (len(ids) > 0 && !reflect.DeepEqual(ids, tt.wantIDs)) || len(got) != len(tt.wantIDs) {
				t.Fatalf("ids = %v, want %v", ids, tt.wantIDs)
			}
			if len(got) > 0 && (got[0].URL != "https://cdn/kc" || *got[0].Width != 100 || got[1].Width != nil || got[2].Position != 1) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
