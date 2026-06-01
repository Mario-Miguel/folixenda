package models

// EventCategory represents the type/genre of an event.
type EventCategory string

const (
	CategoryMusic   EventCategory = "Music"
	CategoryTheater EventCategory = "Theater"
	CategoryParties EventCategory = "Parties"
	CategorySports  EventCategory = "Sports"
	CategoryFood    EventCategory = "Food"
	CategoryArt     EventCategory = "Art"
	CategoryWellness EventCategory = "Wellness"
)

// Event represents a local event.
type Event struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Category    EventCategory `json:"category"`
	Date        string        `json:"date"`      // YYYY-MM-DD
	StartTime   string        `json:"startTime"` // HH:MM
	EndTime     string        `json:"endTime"`   // HH:MM
	Venue       string        `json:"venue"`
	Address     string        `json:"address"`
	Price       float64       `json:"price"`
	ImageURL    string        `json:"imageUrl,omitempty"`
	ArtistName  string        `json:"artistName,omitempty"`
	Perks       []string      `json:"perks,omitempty"`
	IsSaved     bool          `json:"isSaved"`
}
