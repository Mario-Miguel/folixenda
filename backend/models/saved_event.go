package models

// SavedEvent is an Event saved by a user, plus the time it was saved.
type SavedEvent struct {
	Event
	CreatedAt string `json:"savedAt"`
}

type UserSavedEvent struct {
	ID      string `json:"id"`
	UserId  string `json:"userId"`
	EventId string `json:"eventID"`
}

type UserSavedEvents struct {
	UserId      string       `json:"userId"`
	SavedEvents []SavedEvent `json:"savedEvents"`
}
