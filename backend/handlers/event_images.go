package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register decoders for image.DecodeConfig
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

// Event images live in object storage under events/{eventId}/{imageId}.{ext};
// their rows are in event_images. Only the event's owner (or an admin) may
// manage them, with the same rule as PUT /api/events/{id} (see loadOwned).
//
// TODO: deleting an account in Better Auth cascades the publisher's events
// and image rows in Postgres, but nothing removes their objects from the
// bucket: they become orphans. Clean them up (e.g. a periodic job that
// deletes objects under events/ without an event_images row).

const (
	// MaxImageBytes is the largest accepted image file.
	MaxImageBytes = 5 << 20
	// MaxGalleryImages is the most gallery images per event (plus one cover).
	MaxGalleryImages = 10
	// maxUploadBody bounds the whole multipart body: the file plus headers
	// and the small "kind" field.
	maxUploadBody = MaxImageBytes + 64<<10
	// uploadDeadline replaces the server's read/write timeouts for uploads,
	// which can take longer than an ordinary request on slow links.
	uploadDeadline = 2 * time.Minute
)

// imageTypes maps the accepted content types to their file extension.
var imageTypes = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// sniffImageType detects the content type from the file's magic bytes.
func sniffImageType(data []byte) string {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return http.DetectContentType(data)
}

var errImageTooBig = errors.New("image too big")

// readImageFile reads the "file" part of a parsed multipart form.
func readImageFile(r *http.Request) ([]byte, error) {
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if header.Size > MaxImageBytes {
		return nil, errImageTooBig
	}
	return io.ReadAll(file)
}

// uploadImage handles POST /api/events/{id}/images (multipart: file, kind).
// 201 with the EventImage; 400 bad kind/file, 404 event, 409 gallery full,
// 413 file over MaxImageBytes.
func (h *EventsHandler) uploadImage(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(uploadDeadline)) // unsupported by some writers (tests)
	_ = rc.SetWriteDeadline(time.Now().Add(uploadDeadline))

	event := h.loadOwned(w, r)
	if event == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("image must be at most %d MB", MaxImageBytes>>20))
			return
		}
		writeError(w, http.StatusBadRequest, "expected a multipart/form-data body")
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck

	kind := r.FormValue("kind")
	if kind != models.ImageKindCover && kind != models.ImageKindGallery {
		writeError(w, http.StatusBadRequest, `kind must be "cover" or "gallery"`)
		return
	}
	data, err := readImageFile(r)
	if err != nil {
		if errors.Is(err, errImageTooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("image must be at most %d MB", MaxImageBytes>>20))
			return
		}
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}

	contentType := sniffImageType(data)
	ext, ok := imageTypes[contentType]
	if !ok {
		writeError(w, http.StatusBadRequest, "only JPEG, PNG and WebP images are accepted")
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		writeError(w, http.StatusBadRequest, "the file is not a valid image")
		return
	}

	if kind == models.ImageKindGallery {
		count := 0
		for _, img := range event.Images {
			if img.Kind == models.ImageKindGallery {
				count++
			}
		}
		if count >= MaxGalleryImages { // early check; AddImage enforces it atomically
			writeError(w, http.StatusConflict, fmt.Sprintf("an event can have at most %d gallery images", MaxGalleryImages))
			return
		}
	}

	id := uuid.NewString()
	img := &models.EventImage{
		ID:          id,
		EventID:     event.ID,
		Kind:        kind,
		ObjectKey:   fmt.Sprintf("events/%s/%s.%s", event.ID, id, ext),
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		Width:       &cfg.Width,
		Height:      &cfg.Height,
	}
	if err := h.objects.Put(r.Context(), img.ObjectKey, bytes.NewReader(data), img.SizeBytes, contentType); err != nil {
		slog.Error("upload image object", "key", img.ObjectKey, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to store image")
		return
	}

	replaced, err := h.images.AddImage(img, MaxGalleryImages)
	if err != nil {
		h.deleteObjects(r.Context(), img.ObjectKey) // the row wasn't written
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "event not found")
		case errors.Is(err, store.ErrGalleryFull):
			writeError(w, http.StatusConflict, fmt.Sprintf("an event can have at most %d gallery images", MaxGalleryImages))
		default:
			writeError(w, http.StatusInternalServerError, "failed to save image")
		}
		return
	}
	if replaced != nil {
		h.deleteObjects(r.Context(), replaced.ObjectKey)
	}
	img.URL = h.objects.URL(img.ObjectKey)
	writeJSON(w, http.StatusCreated, img)
}

// deleteImage handles DELETE /api/events/{id}/images/{imageId} → 204.
func (h *EventsHandler) deleteImage(w http.ResponseWriter, r *http.Request) {
	event := h.loadOwned(w, r)
	if event == nil {
		return
	}
	img, err := h.images.DeleteImage(event.ID, r.PathValue("imageId"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "image not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete image")
		return
	}
	h.deleteObjects(r.Context(), img.ObjectKey)
	w.WriteHeader(http.StatusNoContent)
}

type reorderImagesInput struct {
	ImageIDs []string `json:"imageIds"`
}

// reorderImages handles PUT /api/events/{id}/images/order with
// {"imageIds": [...]} listing every gallery image exactly once → 200 with
// all the event's images.
func (h *EventsHandler) reorderImages(w http.ResponseWriter, r *http.Request) {
	event := h.loadOwned(w, r)
	if event == nil {
		return
	}
	var in reorderImagesInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ImageIDs == nil {
		writeError(w, http.StatusBadRequest, `body must be {"imageIds": [...]}`)
		return
	}
	images, err := h.images.ReorderImages(event.ID, in.ImageIDs)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "imageIds must list every gallery image exactly once")
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "event not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to reorder images")
		}
		return
	}
	writeJSON(w, http.StatusOK, images)
}
