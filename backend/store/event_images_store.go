package store

import (
	"database/sql"
	"errors"
	"slices"

	"github.com/lib/pq"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

// EventImageStore keeps the rows of images uploaded for events. Objects in
// storage are handled by the caller. Every write locks the event row, so
// writes to one event's images are serialized (and serialized with Delete).
type EventImageStore interface {
	// AddImage inserts img (ID, EventID, ObjectKey, Kind, ContentType,
	// SizeBytes and optional Width/Height set) and fills its Position and URL.
	// A cover replaces the existing cover, which is returned so its object can
	// be deleted. A gallery image goes last; ErrGalleryFull when the gallery
	// already has maxGallery images. ErrNotFound when the event is missing.
	AddImage(img *models.EventImage, maxGallery int) (replaced *models.EventImage, err error)
	// DeleteImage removes one image of the event and compacts the gallery
	// positions. ErrNotFound when the event has no such image.
	DeleteImage(eventID, imageID string) (*models.EventImage, error)
	// ReorderImages sets the gallery order to imageIDs, which must list every
	// gallery image exactly once (else ErrInvalidInput), and returns all the
	// event's images, cover first.
	ReorderImages(eventID string, imageIDs []string) ([]models.EventImage, error)
}

// lockEvent locks the event row for the rest of tx, or returns ErrNotFound.
func lockEvent(tx *sql.Tx, eventID string) error {
	var id string
	err := tx.QueryRow(`SELECT id FROM events WHERE id=$1 FOR UPDATE`, eventID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *EventStoreDBConnection) AddImage(img *models.EventImage, maxGallery int) (*models.EventImage, error) {
	var replaced *models.EventImage
	err := s.inTx(func(tx *sql.Tx) error {
		if err := lockEvent(tx, img.EventID); err != nil {
			return err
		}
		img.Position = 0
		if img.Kind == models.ImageKindCover {
			old := models.EventImage{EventID: img.EventID, Kind: models.ImageKindCover}
			err := tx.QueryRow(`DELETE FROM event_images WHERE event_id=$1 AND kind='cover' RETURNING id, object_key`,
				img.EventID).Scan(&old.ID, &old.ObjectKey)
			switch {
			case err == nil:
				replaced = &old
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
		} else {
			var count int
			if err := tx.QueryRow(`SELECT COUNT(*), COALESCE(MAX(position) + 1, 0) FROM event_images
				WHERE event_id=$1 AND kind='gallery'`, img.EventID).Scan(&count, &img.Position); err != nil {
				return err
			}
			if count >= maxGallery {
				return ErrGalleryFull
			}
		}
		_, err := tx.Exec(`INSERT INTO event_images (id, event_id, object_key, kind, position, content_type,
			size_bytes, width, height) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			img.ID, img.EventID, img.ObjectKey, img.Kind, img.Position, img.ContentType, img.SizeBytes,
			img.Width, img.Height)
		return translateError(err)
	})
	if err != nil {
		return nil, err
	}
	img.URL = s.imageURL.url(img.ObjectKey)
	return replaced, nil
}

// compactGallerySQL renumbers the gallery 0..n-1, keeping the order.
const compactGallerySQL = `UPDATE event_images SET position = o.rn - 1
	FROM (SELECT id, row_number() OVER (ORDER BY position, created_at) AS rn
	      FROM event_images WHERE event_id=$1 AND kind='gallery') o
	WHERE event_images.id = o.id AND event_images.position <> o.rn - 1`

func (s *EventStoreDBConnection) DeleteImage(eventID, imageID string) (*models.EventImage, error) {
	img := &models.EventImage{ID: imageID, EventID: eventID}
	err := s.inTx(func(tx *sql.Tx) error {
		if err := lockEvent(tx, eventID); err != nil {
			return err
		}
		err := tx.QueryRow(`DELETE FROM event_images WHERE event_id=$1 AND id=$2 RETURNING object_key, kind`,
			eventID, imageID).Scan(&img.ObjectKey, &img.Kind)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if img.Kind == models.ImageKindGallery {
			_, err = tx.Exec(compactGallerySQL, eventID)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	img.URL = s.imageURL.url(img.ObjectKey)
	return img, nil
}

func (s *EventStoreDBConnection) ReorderImages(eventID string, imageIDs []string) ([]models.EventImage, error) {
	var out []models.EventImage
	err := s.inTx(func(tx *sql.Tx) error {
		if err := lockEvent(tx, eventID); err != nil {
			return err
		}
		var current pq.StringArray
		if err := tx.QueryRow(`SELECT COALESCE(array_agg(id), '{}') FROM event_images
			WHERE event_id=$1 AND kind='gallery'`, eventID).Scan(&current); err != nil {
			return err
		}
		if !sameSet(current, imageIDs) {
			return ErrInvalidInput
		}
		if _, err := tx.Exec(`UPDATE event_images SET position = t.pos - 1
			FROM unnest($2::text[]) WITH ORDINALITY AS t(id, pos)
			WHERE event_images.event_id = $1 AND event_images.id = t.id`,
			eventID, pq.StringArray(imageIDs)); err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT id, object_key, kind, position, width, height FROM event_images
			WHERE event_id=$1 ORDER BY kind = 'gallery', position, created_at`, eventID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = []models.EventImage{}
		for rows.Next() {
			img := models.EventImage{EventID: eventID}
			var w, h sql.NullInt64
			if err := rows.Scan(&img.ID, &img.ObjectKey, &img.Kind, &img.Position, &w, &h); err != nil {
				return err
			}
			img.Width, img.Height = nullInt(w), nullInt(h)
			img.URL = s.imageURL.url(img.ObjectKey)
			out = append(out, img)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// sameSet reports whether got lists every id of want (distinct ids) exactly
// once: sorted copies are equal only if got has no duplicates nor extras.
func sameSet(want, got []string) bool {
	a, b := slices.Clone(want), slices.Clone(got)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func nullInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int64)
	return &i
}
