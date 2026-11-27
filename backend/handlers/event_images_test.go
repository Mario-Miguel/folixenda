package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{236, 91, 19, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 3, 2)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// webpBytes is a 1×1 lossless WebP.
func webpBytes(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// multipartBody builds a multipart form; file == nil omits the file part.
func multipartBody(t *testing.T, kind string, file []byte) (string, string) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	if kind != "" {
		mw.WriteField("kind", kind) //nolint:errcheck
	}
	if file != nil {
		fw, _ := mw.CreateFormFile("file", "upload.bin")
		fw.Write(file) //nolint:errcheck
	}
	mw.Close()
	return b.String(), mw.FormDataContentType()
}

type imagesFixture struct {
	events  *fakeEventStore
	images  *fakeImageStore
	objects *fakeObjects
	mux     *http.ServeMux
}

func newImagesFixture() *imagesFixture {
	f := &imagesFixture{events: newFakeEventStore(sampleEvents()...), objects: newFakeObjects()}
	f.images = &fakeImageStore{events: f.events}
	f.mux = http.NewServeMux()
	NewEventsHandler(f.events, f.images, f.objects).Register(f.mux, fakeProtect)
	return f
}

func (f *imagesFixture) upload(t *testing.T, eventID, kind string, file []byte, who identity) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, kind, file)
	req := httptest.NewRequest("POST", "/api/events/"+eventID+"/images", strings.NewReader(body))
	req.Header.Set("Content-Type", ct)
	if who.sub != "" {
		req.Header.Set(hdrSub, who.sub)
		req.Header.Set(hdrRole, who.role)
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

func TestUploadImage(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		eventID    string
		kind       string
		file       func(*testing.T) []byte
		wantStatus int
		wantType   string
	}{
		{"unauthenticated", anon, "e1", "cover", func(t *testing.T) []byte { return pngBytes(t, 4, 3) }, 401, ""},
		{"consumer", consumer, "e1", "cover", func(t *testing.T) []byte { return pngBytes(t, 4, 3) }, 403, ""},
		{"other publisher's event", publisher, "e2", "cover", func(t *testing.T) []byte { return pngBytes(t, 4, 3) }, 403, ""},
		{"scraped event, publisher", publisher, "e3", "cover", func(t *testing.T) []byte { return pngBytes(t, 4, 3) }, 403, ""},
		{"missing event", admin, "nope", "cover", func(t *testing.T) []byte { return pngBytes(t, 4, 3) }, 404, ""},
		{"png cover", publisher, "e1", "cover", func(t *testing.T) []byte { return pngBytes(t, 4, 3) }, 201, "image/png"},
		{"jpeg gallery by admin on scraped event", admin, "e3", "gallery", jpegBytes, 201, "image/jpeg"},
		{"webp gallery", publisher, "e1", "gallery", webpBytes, 201, "image/webp"},
		{"bad kind", publisher, "e1", "banner", func(t *testing.T) []byte { return pngBytes(t, 4, 3) }, 400, ""},
		{"no file", publisher, "e1", "cover", nil, 400, ""},
		{"gif rejected", publisher, "e1", "cover", func(*testing.T) []byte { return []byte("GIF89a\x01\x00\x01\x00") }, 400, ""},
		{"text rejected", publisher, "e1", "cover", func(*testing.T) []byte { return []byte("hello") }, 400, ""},
		{"truncated png", publisher, "e1", "cover", func(t *testing.T) []byte { return pngBytes(t, 4, 3)[:20] }, 400, ""},
		{"too big", publisher, "e1", "cover", func(*testing.T) []byte {
			return append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxImageBytes)...)
		}, 413, ""},
		{"body over the request limit", publisher, "e1", "cover", func(*testing.T) []byte {
			return append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, maxUploadBody)...)
		}, 413, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newImagesFixture()
			var file []byte
			if tt.file != nil {
				file = tt.file(t)
			}
			rec := f.upload(t, tt.eventID, tt.kind, file, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 201 {
				if len(f.objects.keys()) != 0 {
					t.Fatalf("objects left behind: %v", f.objects.keys())
				}
				return
			}
			img := decode[models.EventImage](t, rec)
			ext := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp"}[tt.wantType]
			key := fmt.Sprintf("events/%s/%s.%s", tt.eventID, img.ID, ext)
			if img.URL != "http://cdn/"+key || img.Kind != tt.kind || img.Width == nil || img.Height == nil {
				t.Fatalf("img = %+v", img)
			}
			if f.objects.types[key] != tt.wantType || !bytes.Equal(f.objects.objects[key], file) {
				t.Fatalf("stored %v as %q", f.objects.keys(), f.objects.types[key])
			}
			if !strings.Contains(rec.Body.String(), `"position":`) || strings.Contains(rec.Body.String(), "events/"+tt.eventID+"/"+img.ID+"."+ext+`"`) &&
				!strings.Contains(rec.Body.String(), `"url":"http://cdn/`) {
				t.Fatalf("body %s", rec.Body)
			}
		})
	}
}

func TestUploadImage_Dimensions(t *testing.T) {
	f := newImagesFixture()
	img := decode[models.EventImage](t, f.upload(t, "e1", "cover", pngBytes(t, 7, 5), publisher))
	if *img.Width != 7 || *img.Height != 5 || img.Position != 0 {
		t.Fatalf("img = %+v", img)
	}
	img = decode[models.EventImage](t, f.upload(t, "e1", "gallery", webpBytes(t), publisher))
	if *img.Width != 1 || *img.Height != 1 {
		t.Fatalf("webp = %+v", img)
	}
}

func TestUploadImage_CoverReplacesOld(t *testing.T) {
	f := newImagesFixture()
	first := decode[models.EventImage](t, f.upload(t, "e1", "cover", pngBytes(t, 2, 2), publisher))
	second := decode[models.EventImage](t, f.upload(t, "e1", "cover", jpegBytes(t), publisher))
	keys := f.objects.keys()
	if len(keys) != 1 || !strings.Contains(keys[0], second.ID) {
		t.Fatalf("objects = %v", keys)
	}
	if len(f.objects.deleted) != 1 || !strings.Contains(f.objects.deleted[0], first.ID) {
		t.Fatalf("deleted = %v", f.objects.deleted)
	}
	// The event's imageUrl now comes from the new cover.
	e := decode[models.Event](t, do(t, f.mux, "GET", "/api/events/e1", "", anon))
	if e.ImageURL != second.URL || len(e.Images) != 1 {
		t.Fatalf("event = %+v", e)
	}
}

func TestUploadImage_GalleryLimit(t *testing.T) {
	f := newImagesFixture()
	for i := range MaxGalleryImages {
		img := decode[models.EventImage](t, f.upload(t, "e1", "gallery", pngBytes(t, 1, 1), publisher))
		if img.Position != i {
			t.Fatalf("image %d position %d", i, img.Position)
		}
	}
	if rec := f.upload(t, "e1", "gallery", pngBytes(t, 1, 1), publisher); rec.Code != 409 {
		t.Fatalf("11th gallery image: status %d", rec.Code)
	}
	if rec := f.upload(t, "e1", "cover", pngBytes(t, 1, 1), publisher); rec.Code != 201 {
		t.Fatalf("a cover is still allowed: status %d", rec.Code)
	}
}

func TestUploadImage_StoreFailures(t *testing.T) {
	t.Run("object put fails", func(t *testing.T) {
		f := newImagesFixture()
		f.objects.putErr = errDB
		if rec := f.upload(t, "e1", "cover", pngBytes(t, 1, 1), publisher); rec.Code != 500 {
			t.Fatalf("status %d", rec.Code)
		}
	})
	for _, tt := range []struct {
		name   string
		err    error
		status int
	}{
		{"event deleted meanwhile", store.ErrNotFound, 404},
		{"gallery filled meanwhile", store.ErrGalleryFull, 409},
		{"db error", errDB, 500},
	} {
		t.Run(tt.name+": uploaded object removed", func(t *testing.T) {
			f := newImagesFixture()
			f.images.err = tt.err
			rec := f.upload(t, "e1", "gallery", pngBytes(t, 1, 1), publisher)
			if rec.Code != tt.status || len(f.objects.keys()) != 0 || len(f.objects.deleted) != 1 {
				t.Fatalf("status %d objects %v deleted %v", rec.Code, f.objects.keys(), f.objects.deleted)
			}
		})
	}
	t.Run("not multipart", func(t *testing.T) {
		f := newImagesFixture()
		if rec := do(t, f.mux, "POST", "/api/events/e1/images", `{"kind":"cover"}`, publisher); rec.Code != 400 {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

func TestSniffImageType(t *testing.T) {
	if got := sniffImageType(webpBytes(t)); got != "image/webp" {
		t.Fatal(got)
	}
	if got := sniffImageType([]byte("RIFF\x00\x00\x00\x00WAVE")); got == "image/webp" {
		t.Fatal("WAVE is not webp")
	}
	if got := sniffImageType(jpegBytes(t)); got != "image/jpeg" {
		t.Fatal(got)
	}
}

func TestDeleteImage(t *testing.T) {
	setup := func(t *testing.T) (*imagesFixture, models.EventImage) {
		f := newImagesFixture()
		img := decode[models.EventImage](t, f.upload(t, "e1", "gallery", pngBytes(t, 1, 1), publisher))
		return f, img
	}
	t.Run("owner deletes: row and object gone", func(t *testing.T) {
		f, img := setup(t)
		rec := do(t, f.mux, "DELETE", "/api/events/e1/images/"+img.ID, "", publisher)
		if rec.Code != 204 || len(f.objects.keys()) != 0 || len(f.events.events["e1"].Images) != 0 {
			t.Fatalf("status %d objects %v", rec.Code, f.objects.keys())
		}
	})
	for _, tt := range []struct {
		name    string
		who     identity
		path    string
		dbErr   error
		status  int
		deleted bool
	}{
		{"unauthenticated", anon, "/api/events/e1/images/X", nil, 401, false},
		{"other publisher", otherPublisher, "/api/events/e1/images/X", nil, 403, false},
		{"admin", admin, "/api/events/e1/images/X", nil, 204, true},
		{"unknown image", publisher, "/api/events/e1/images/nope", nil, 404, false},
		{"unknown event", publisher, "/api/events/nope/images/X", nil, 404, false},
		{"db error", publisher, "/api/events/e1/images/X", errDB, 500, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, img := setup(t)
			f.images.err = tt.dbErr
			rec := do(t, f.mux, "DELETE", strings.Replace(tt.path, "X", img.ID, 1), "", tt.who)
			if rec.Code != tt.status || (len(f.objects.keys()) == 0) != tt.deleted {
				t.Fatalf("status %d objects %v", rec.Code, f.objects.keys())
			}
		})
	}
}

func TestReorderImages(t *testing.T) {
	setup := func(t *testing.T) (*imagesFixture, []string) {
		f := newImagesFixture()
		var ids []string
		for range 3 {
			ids = append(ids, decode[models.EventImage](t, f.upload(t, "e1", "gallery", pngBytes(t, 1, 1), publisher)).ID)
		}
		decode[models.EventImage](t, f.upload(t, "e1", "cover", pngBytes(t, 1, 1), publisher))
		return f, ids
	}
	t.Run("ok", func(t *testing.T) {
		f, ids := setup(t)
		body := fmt.Sprintf(`{"imageIds":["%s","%s","%s"]}`, ids[2], ids[0], ids[1])
		rec := do(t, f.mux, "PUT", "/api/events/e1/images/order", body, publisher)
		if rec.Code != 200 {
			t.Fatalf("status %d %s", rec.Code, rec.Body)
		}
		got := decode[[]models.EventImage](t, rec)
		pos := map[string]int{}
		for _, img := range got {
			pos[img.ID] = img.Position
		}
		if len(got) != 4 || pos[ids[2]] != 0 || pos[ids[0]] != 1 || pos[ids[1]] != 2 {
			t.Fatalf("got %+v", got)
		}
	})
	for _, tt := range []struct {
		name   string
		who    identity
		event  string
		body   func(ids []string) string
		dbErr  error
		status int
	}{
		{"unauthenticated", anon, "e1", func([]string) string { return `{"imageIds":[]}` }, nil, 401},
		{"other publisher", otherPublisher, "e1", func([]string) string { return `{"imageIds":[]}` }, nil, 403},
		{"missing event", admin, "nope", func([]string) string { return `{"imageIds":[]}` }, nil, 404},
		{"bad json", publisher, "e1", func([]string) string { return `{` }, nil, 400},
		{"no imageIds", publisher, "e1", func([]string) string { return `{}` }, nil, 400},
		{"missing one", publisher, "e1", func(ids []string) string { return `{"imageIds":["` + ids[0] + `"]}` }, nil, 400},
		{"duplicate", publisher, "e1", func(ids []string) string {
			return fmt.Sprintf(`{"imageIds":["%s","%s","%s"]}`, ids[0], ids[0], ids[1])
		}, nil, 400},
		{"event gone meanwhile", publisher, "e1", func([]string) string { return `{"imageIds":[]}` }, store.ErrNotFound, 404},
		{"db error", publisher, "e1", func([]string) string { return `{"imageIds":[]}` }, errDB, 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, ids := setup(t)
			f.images.err = tt.dbErr
			rec := do(t, f.mux, "PUT", "/api/events/"+tt.event+"/images/order", tt.body(ids), tt.who)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.status, rec.Body)
			}
		})
	}
}
