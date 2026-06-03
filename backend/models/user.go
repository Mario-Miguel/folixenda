package models

type User struct {
	ID               string    `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	Role             string    `json:"role"`
	SubscriptionType string    `json:"subscriptionType,omitempty"`
	PaymentMethod    string    `json:"paymentMethod,omitempty"`
	Location         []float64 `json:"location,omitempty"`
	EventPreferences []string  `json:"eventPreferences,omitempty"`
	Password         string    `json:"password,omitempty"` // write-only; cleared before API responses
}
