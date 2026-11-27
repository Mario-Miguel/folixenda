package models

// PublisherProfile is the organization data of a publisher account.
// Timestamps are RFC 3339.
type PublisherProfile struct {
	UserID      string `json:"userId"`
	OrgName     string `json:"orgName"`
	Website     string `json:"website,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// NotificationPreferences are a user's stored notification settings.
// Categories is never nil, so it serializes as [] rather than null.
type NotificationPreferences struct {
	SavedEventReminders bool            `json:"savedEventReminders"`
	CategoryNews        bool            `json:"categoryNews"`
	Categories          []EventCategory `json:"categories"`
	Newsletter          bool            `json:"newsletter"`
	PublisherActivity   bool            `json:"publisherActivity"`
}

// DefaultNotificationPreferences are the settings of a user who never saved
// any (they match the column defaults of notification_preferences).
func DefaultNotificationPreferences() *NotificationPreferences {
	return &NotificationPreferences{
		SavedEventReminders: true,
		CategoryNews:        false,
		Categories:          []EventCategory{},
		Newsletter:          false,
		PublisherActivity:   true,
	}
}
