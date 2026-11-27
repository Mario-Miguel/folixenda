package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/geocode"
)

type fakeGeocoder struct {
	results   []geocode.Result
	reverse   *geocode.Result
	err       error
	lastQ     string
	lastLimit int
	lastPoint [2]float64
}

func (f *fakeGeocoder) Search(_ context.Context, q string, limit int) ([]geocode.Result, error) {
	f.lastQ, f.lastLimit = q, limit
	return f.results, f.err
}

func (f *fakeGeocoder) Reverse(_ context.Context, lat, lon float64) (*geocode.Result, error) {
	f.lastPoint = [2]float64{lat, lon}
	return f.reverse, f.err
}

func geocodeMux(g Geocoder) *http.ServeMux {
	mux := http.NewServeMux()
	NewGeocodeHandler(g).Register(mux, fakeProtect)
	return mux
}

var _ Geocoder = (*geocode.Client)(nil)

func TestGeocodeSearch(t *testing.T) {
	uria := geocode.Result{Label: "Calle Uria 10, Oviedo, Asturias", Address: "Calle Uria 10", Lat: 43.36, Lon: -5.85}
	tests := []struct {
		name       string
		who        identity
		query      string
		results    []geocode.Result
		err        error
		wantStatus int
		wantQ      string
		wantLimit  int
		wantBody   string
	}{
		{"unauthenticated", anon, "?q=uria", nil, nil, 401, "", 0, ""},
		{"consumer", consumer, "?q=uria", nil, nil, 403, "", 0, ""},
		{"publisher, default limit, trimmed", publisher, "?q=%20%20uria%2010%20", []geocode.Result{uria}, nil, 200, "uria 10", 8,
			`[{"label":"Calle Uria 10, Oviedo, Asturias","address":"Calle Uria 10","lat":43.36,"lon":-5.85}]`},
		{"admin, limit clamped", admin, "?q=uria&limit=50", nil, nil, 200, "uria", 10, "[]"},
		{"explicit limit", publisher, "?q=uria&limit=3", nil, nil, 200, "uria", 3, "[]"},
		{"too short", publisher, "?q=%20ab%20", nil, nil, 400, "", 0, ""},
		{"three runes ok", publisher, "?q=%C3%B1%C3%B1%C3%B1", nil, nil, 200, "ñññ", 8, "[]"},
		{"bad limit", publisher, "?q=uria&limit=x", nil, nil, 400, "", 0, ""},
		{"zero limit", publisher, "?q=uria&limit=0", nil, nil, 400, "", 0, ""},
		{"upstream error", publisher, "?q=uria", nil, errDB, 502, "uria", 8, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &fakeGeocoder{results: tt.results, err: tt.err}
			rec := do(t, geocodeMux(g), "GET", "/api/geocode/search"+tt.query, "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if g.lastQ != tt.wantQ || g.lastLimit != tt.wantLimit {
				t.Fatalf("called with %q/%d", g.lastQ, g.lastLimit)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody+"\n" {
				t.Fatalf("body = %s", rec.Body)
			}
		})
	}
}

func TestGeocodeReverse(t *testing.T) {
	res := &geocode.Result{Label: "X", Address: "X", Lat: 43, Lon: -5}
	tests := []struct {
		name       string
		who        identity
		query      string
		result     *geocode.Result
		err        error
		wantStatus int
	}{
		{"unauthenticated", anon, "?lat=43&lon=-5", res, nil, 401},
		{"consumer", consumer, "?lat=43&lon=-5", res, nil, 403},
		{"ok", publisher, "?lat=43.36&lon=-5.85", res, nil, 200},
		{"edges ok", admin, "?lat=-90&lon=180", res, nil, 200},
		{"lat out of range", publisher, "?lat=91&lon=0", res, nil, 400},
		{"lon out of range", publisher, "?lat=0&lon=-180.1", res, nil, 400},
		{"missing lon", publisher, "?lat=0", res, nil, 400},
		{"nan", publisher, "?lat=NaN&lon=0", res, nil, 400},
		{"not found", publisher, "?lat=0&lon=0", nil, geocode.ErrNotFound, 404},
		{"upstream error", publisher, "?lat=0&lon=0", nil, errDB, 502},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &fakeGeocoder{reverse: tt.result, err: tt.err}
			rec := do(t, geocodeMux(g), "GET", "/api/geocode/reverse"+tt.query, "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.name == "ok" && (g.lastPoint != [2]float64{43.36, -5.85} || decode[geocode.Result](t, rec).Label != "X") {
				t.Fatalf("point %v body %s", g.lastPoint, rec.Body)
			}
		})
	}
}
