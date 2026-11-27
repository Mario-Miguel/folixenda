package models

import (
	"cmp"
	"slices"
)

// EventCategory represents the type/genre of an event.
type EventCategory string

const (
	CategoryMusic    EventCategory = "Music"
	CategoryTheater  EventCategory = "Theater"
	CategoryParties  EventCategory = "Parties"
	CategorySports   EventCategory = "Sports"
	CategoryFood     EventCategory = "Food"
	CategoryArt      EventCategory = "Art"
	CategoryWellness EventCategory = "Wellness"
)

// Categories lists every valid EventCategory.
var Categories = []EventCategory{
	CategoryMusic,
	CategoryTheater,
	CategoryParties,
	CategorySports,
	CategoryFood,
	CategoryArt,
	CategoryWellness,
}

// Valid reports whether c is one of Categories.
func (c EventCategory) Valid() bool {
	return slices.Contains(Categories, c)
}

// Event represents a local event.
//
// Sessions are the event's days (always at least one for events with a start
// time). Date/StartTime/EndTime mirror the first session and EndDate is the
// last session's date; they are derived by ApplySessions and ignored in
// request bodies.
type Event struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Category    EventCategory  `json:"category"`
	Date        string         `json:"date"`      // YYYY-MM-DD of the first session
	StartTime   string         `json:"startTime"` // HH:MM of the first session
	EndTime     string         `json:"endTime"`   // HH:MM of the first session
	EndDate     string         `json:"endDate"`   // YYYY-MM-DD of the last session
	Sessions    []EventSession `json:"sessions"`
	Venue       string         `json:"venue"`
	Address     string         `json:"address"`
	Price       float64        `json:"price"`
	// ImageURL is the cover image's URL when one was uploaded, else
	// StoredImageURL (e.g. an image URL found by the scraper).
	ImageURL string `json:"imageUrl,omitempty"`
	// StoredImageURL is the events.image_url column.
	StoredImageURL string       `json:"-"`
	Images         []EventImage `json:"images"` // cover first, then gallery by position
	ArtistName     string       `json:"artistName,omitempty"`
	Perks          []string     `json:"perks,omitempty"`
	IsSaved        bool         `json:"isSaved"`
	Lat            *float64     `json:"lat,omitempty"` // nil when the event has no coordinates
	Lon            *float64     `json:"lon,omitempty"`
	// PublisherID is the publisher that owns the event; "" (NULL) for scraped events.
	PublisherID string `json:"publisherId,omitempty"`
	// Organizer is only filled by GET /api/events/{id}, when the owner has a profile.
	Organizer *Organizer `json:"organizer,omitempty"`
}

// EventSession is one day of an event. An EndTime earlier than StartTime
// means the session ends after midnight, on the next day.
type EventSession struct {
	Date      string `json:"date"`      // YYYY-MM-DD
	StartTime string `json:"startTime"` // HH:MM
	EndTime   string `json:"endTime"`   // HH:MM
}

// CompareSessions orders sessions by date, then start time.
func CompareSessions(a, b EventSession) int {
	return cmp.Or(cmp.Compare(a.Date, b.Date), cmp.Compare(a.StartTime, b.StartTime))
}

// ApplySessions sorts e.Sessions and derives Date, StartTime and EndTime
// (first session) and EndDate (last session) from them. Sessions is never nil
// afterwards; with no sessions the derived fields are left unchanged.
func (e *Event) ApplySessions() {
	if e.Sessions == nil {
		e.Sessions = []EventSession{}
	}
	slices.SortFunc(e.Sessions, CompareSessions)
	if len(e.Sessions) == 0 {
		return
	}
	first, last := e.Sessions[0], e.Sessions[len(e.Sessions)-1]
	e.Date, e.StartTime, e.EndTime = first.Date, first.StartTime, first.EndTime
	e.EndDate = last.Date
}

// Image kinds: at most one cover per event, plus a gallery.
const (
	ImageKindCover   = "cover"
	ImageKindGallery = "gallery"
)

// EventImage is an image uploaded for an event and kept in object storage.
type EventImage struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Kind     string `json:"kind"`     // ImageKindCover or ImageKindGallery
	Position int    `json:"position"` // gallery order, 0-based; 0 for the cover
	Width    *int   `json:"width,omitempty"`
	Height   *int   `json:"height,omitempty"`

	EventID     string `json:"-"`
	ObjectKey   string `json:"-"` // key in the bucket, e.g. events/{eventId}/{id}.jpg
	ContentType string `json:"-"`
	SizeBytes   int64  `json:"-"`
}

// Cover returns the event's cover image, or nil.
func (e *Event) Cover() *EventImage {
	for i := range e.Images {
		if e.Images[i].Kind == ImageKindCover {
			return &e.Images[i]
		}
	}
	return nil
}

// ApplyImages makes Images non-nil and sets ImageURL to the cover's URL, or
// to StoredImageURL when there is no cover.
func (e *Event) ApplyImages() {
	if e.Images == nil {
		e.Images = []EventImage{}
	}
	e.ImageURL = e.StoredImageURL
	if c := e.Cover(); c != nil {
		e.ImageURL = c.URL
	}
}

// Organizer is the public data of the publisher that owns an event.
type Organizer struct {
	ID          string `json:"id"`
	OrgName     string `json:"orgName"`
	Website     string `json:"website,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Description string `json:"description,omitempty"`
}

// PublisherEvent is a publisher's own event plus how many users saved it.
type PublisherEvent struct {
	Event
	SavesCount int `json:"savesCount"`
}
