package handlers

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Mario-Miguel/folixenda/backend/geocode"
	"github.com/Mario-Miguel/folixenda/backend/middleware"
)

// Geocoder is what GeocodeHandler needs from geocode.Client.
type Geocoder interface {
	Search(ctx context.Context, q string, limit int) ([]geocode.Result, error)
	Reverse(ctx context.Context, lat, lon float64) (*geocode.Result, error)
}

// GeocodeHandler proxies address search for the event editor, so browsers
// never call CartoCiudad / Nominatim directly.
type GeocodeHandler struct {
	geo Geocoder
}

func NewGeocodeHandler(g Geocoder) *GeocodeHandler {
	return &GeocodeHandler{geo: g}
}

const (
	geocodeMinQuery     = 3
	geocodeDefaultLimit = 8
	geocodeMaxLimit     = 10
)

// Register mounts the geocode routes behind protect + RequireCanPublish: only
// accounts that can publish may use them (keeps anonymous traffic away from
// the upstream services and their usage policies).
func (h *GeocodeHandler) Register(mux *http.ServeMux, protect Middleware) {
	mux.Handle("GET /api/geocode/search", chain(h.search, protect, middleware.RequireCanPublish))
	mux.Handle("GET /api/geocode/reverse", chain(h.reverse, protect, middleware.RequireCanPublish))
}

// search handles GET /api/geocode/search?q=&limit= → 200 [GeocodeResult]
// (possibly empty); 400 bad q/limit; 502 when the providers fail.
func (h *GeocodeHandler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) < geocodeMinQuery {
		writeError(w, http.StatusBadRequest, "q must have at least 3 characters")
		return
	}
	limit := geocodeDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = min(n, geocodeMaxLimit)
	}
	results, err := h.geo.Search(r.Context(), q, limit)
	if err != nil {
		slog.Warn("geocode search", "err", err)
		writeError(w, http.StatusBadGateway, "address search is unavailable")
		return
	}
	if results == nil {
		results = []geocode.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

func parseCoord(v string, limit float64) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || math.IsNaN(f) || math.Abs(f) > limit {
		return 0, false
	}
	return f, true
}

// reverse handles GET /api/geocode/reverse?lat=&lon= → 200 GeocodeResult;
// 400 bad coordinates; 404 nothing there; 502 providers failing.
func (h *GeocodeHandler) reverse(w http.ResponseWriter, r *http.Request) {
	lat, ok1 := parseCoord(r.URL.Query().Get("lat"), 90)
	lon, ok2 := parseCoord(r.URL.Query().Get("lon"), 180)
	if !ok1 || !ok2 {
		writeError(w, http.StatusBadRequest, "lat must be in [-90, 90] and lon in [-180, 180]")
		return
	}
	res, err := h.geo.Reverse(r.Context(), lat, lon)
	switch {
	case errors.Is(err, geocode.ErrNotFound):
		writeError(w, http.StatusNotFound, "no address found at that point")
	case err != nil:
		slog.Warn("geocode reverse", "err", err)
		writeError(w, http.StatusBadGateway, "address search is unavailable")
	default:
		writeJSON(w, http.StatusOK, res)
	}
}
