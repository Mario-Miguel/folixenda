package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/middleware"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

// MeHandler serves the authenticated user's own data. The user always comes
// from the token (claims.Subject), never from the URL.
type MeHandler struct {
	users    store.UserStore
	profiles store.PublisherProfileStore
	prefs    store.NotificationPreferencesStore
	events   store.EventStore
}

func NewMeHandler(users store.UserStore, profiles store.PublisherProfileStore,
	prefs store.NotificationPreferencesStore, events store.EventStore) *MeHandler {
	return &MeHandler{users: users, profiles: profiles, prefs: prefs, events: events}
}

// Register mounts the /api/me routes, all authenticated:
//   - GET /api/me, PUT /api/me/notifications: any role
//   - /api/me/publisher-profile, GET /api/me/events: publishers only (admins
//     manage events through the admin panel)
func (h *MeHandler) Register(mux *http.ServeMux, protect Middleware) {
	publisherOnly := middleware.RequireRole(auth.RolePublisher)

	mux.Handle("GET /api/me", chain(h.get, protect))
	mux.Handle("PUT /api/me/notifications", chain(h.updateNotifications, protect))
	mux.Handle("GET /api/me/publisher-profile", chain(h.getPublisherProfile, protect, publisherOnly))
	mux.Handle("PUT /api/me/publisher-profile", chain(h.putPublisherProfile, protect, publisherOnly))
	mux.Handle("GET /api/me/events", chain(h.listEvents, protect, publisherOnly))
}

type meResponse struct {
	User             *models.User                    `json:"user"`
	PublisherProfile *models.PublisherProfile        `json:"publisherProfile"` // null when none
	Notifications    *models.NotificationPreferences `json:"notifications"`
}

// get handles GET /api/me. The publisher profile is only looked up for
// publishers; notifications fall back to the defaults when never saved.
func (h *MeHandler) get(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	user, err := h.users.Get(claims.Subject)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	resp := meResponse{User: user}
	if claims.Role == auth.RolePublisher {
		profile, err := h.profiles.Get(claims.Subject)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "failed to fetch publisher profile")
			return
		}
		resp.PublisherProfile = profile
	}

	prefs, err := h.prefs.Get(claims.Subject)
	switch {
	case errors.Is(err, store.ErrNotFound):
		prefs = models.DefaultNotificationPreferences()
	case err != nil:
		writeError(w, http.StatusInternalServerError, "failed to fetch notification preferences")
		return
	}
	resp.Notifications = prefs
	writeJSON(w, http.StatusOK, resp)
}

// normalizeNotifications validates the categories and dedupes them, keeping
// the first occurrence's order. Categories is never nil afterwards.
func normalizeNotifications(p *models.NotificationPreferences) string {
	out := make([]models.EventCategory, 0, len(p.Categories))
	for _, c := range p.Categories {
		if !c.Valid() {
			return fmt.Sprintf("unknown category %q", c)
		}
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	p.Categories = out
	return ""
}

// updateNotifications handles PUT /api/me/notifications: the body is the full
// set of preferences (absent booleans are false).
func (h *MeHandler) updateNotifications(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	var prefs models.NotificationPreferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := normalizeNotifications(&prefs); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	saved, err := h.prefs.Upsert(claims.Subject, &prefs)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save notification preferences")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *MeHandler) getPublisherProfile(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	profile, err := h.profiles.Get(claims.Subject)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "publisher profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch publisher profile")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// publisherProfileInput is the body of PUT /api/me/publisher-profile.
type publisherProfileInput struct {
	OrgName     string `json:"orgName"`
	Website     string `json:"website"`
	Phone       string `json:"phone"`
	Description string `json:"description"`
}

const (
	orgNameMin     = 2
	orgNameMax     = 120
	websiteMax     = 300
	phoneMax       = 30
	descriptionMax = 1000
)

// validatePublisherProfile trims every field in place and returns the first
// validation error, or "". Lengths are counted in characters (runes).
func validatePublisherProfile(in *publisherProfileInput) string {
	in.OrgName = strings.TrimSpace(in.OrgName)
	in.Website = strings.TrimSpace(in.Website)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Description = strings.TrimSpace(in.Description)

	if n := utf8.RuneCountInString(in.OrgName); n < orgNameMin || n > orgNameMax {
		return fmt.Sprintf("orgName must be between %d and %d characters", orgNameMin, orgNameMax)
	}
	if in.Website != "" {
		if utf8.RuneCountInString(in.Website) > websiteMax {
			return fmt.Sprintf("website must be at most %d characters", websiteMax)
		}
		u, err := url.Parse(in.Website)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "website must be an absolute http(s) URL"
		}
	}
	if utf8.RuneCountInString(in.Phone) > phoneMax {
		return fmt.Sprintf("phone must be at most %d characters", phoneMax)
	}
	for _, c := range in.Phone {
		if !(c >= '0' && c <= '9') && !strings.ContainsRune(" +-().", c) {
			return "phone may only contain digits, spaces and + - ( ) ."
		}
	}
	if utf8.RuneCountInString(in.Description) > descriptionMax {
		return fmt.Sprintf("description must be at most %d characters", descriptionMax)
	}
	return ""
}

// putPublisherProfile handles PUT /api/me/publisher-profile: creates or
// replaces the caller's profile. Empty optional fields are stored as NULL.
func (h *MeHandler) putPublisherProfile(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	var in publisherProfileInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validatePublisherProfile(&in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	saved, err := h.profiles.Upsert(&models.PublisherProfile{
		UserID:      claims.Subject,
		OrgName:     in.OrgName,
		Website:     in.Website,
		Phone:       in.Phone,
		Description: in.Description,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save publisher profile")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type publisherEventsResponse struct {
	Events []*models.PublisherEvent `json:"events"`
	Total  int                      `json:"total"`
}

// listEvents handles GET /api/me/events: all of the caller's events, past and
// future, by start time, with their save counts.
func (h *MeHandler) listEvents(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	events, err := h.events.ListByPublisher(claims.Subject)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	if events == nil {
		events = []*models.PublisherEvent{}
	}
	writeJSON(w, http.StatusOK, publisherEventsResponse{Events: events, Total: len(events)})
}
