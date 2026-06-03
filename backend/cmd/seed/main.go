package main

import (
	"log"

	"github.com/Mario-Miguel/folixenda/backend/database"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func main() {
	database.Init(".env")

	userStore, err := store.NewPostgresUserStore(database.DB)
	if err != nil {
		log.Fatalf("user store: %v", err)
	}
	eventStore, err := store.NewPostgresEventStore(database.DB)
	if err != nil {
		log.Fatalf("event store: %v", err)
	}

	seedUsers(userStore)
	seedEvents(eventStore)

	log.Println("seed complete")
}

func seedUsers(s store.UserStore) {
	users := []*models.User{
		{ID: "1", Username: "admin", Email: "admin@folixenda.com", Name: "Admin", Role: "admin", Password: "admin123"},
		{ID: "2", Username: "consumer", Email: "consumer@example.com", Name: "Consumer User", Role: "consumer", Location: []float64{43.393283, -5.654910}, EventPreferences: []string{"Music", "Art"}, Password: "consumer123"},
		{ID: "3", Username: "publisher", Email: "publisher@example.com", Name: "Publisher User", Role: "publisher", SubscriptionType: "basic", PaymentMethod: "card", Password: "publisher123"},
	}
	for _, u := range users {
		if err := s.Create(u); err != nil {
			log.Printf("skip user %s: %v", u.Username, err)
		} else {
			log.Printf("created user %s", u.Username)
		}
	}
}

func seedEvents(s store.EventStore) {
	events := []*models.Event{
		{ID: "1", Title: "Sunset Jazz Festival", Description: "Experience the soulful rhythms of the city's finest jazz ensemble under the open sky.", Category: models.CategoryMusic, Date: "2024-10-05", StartTime: "18:00", EndTime: "22:00", Venue: "The Grand Plaza Courtyard", Address: "123 Harmonic Avenue, Music District, NY 10001", Price: 45, ArtistName: "Crescent City Quintet", Perks: []string{"Premium outdoor seating", "Complimentary welcome drink", "Digital event program"}, IsSaved: true},
		{ID: "2", Title: "Hamlet: Modern Reimagining", Description: "A groundbreaking adaptation of Shakespeare's classic tragedy set in a corporate dystopia.", Category: models.CategoryTheater, Date: "2024-10-05", StartTime: "19:30", EndTime: "21:30", Venue: "City Playhouse", Address: "45 Stage Lane, Arts Quarter, NY 10002", Price: 60},
		{ID: "3", Title: "Gallery Night Opening", Description: "Discover emerging local artists at the monthly gallery night. Wine and cheese included.", Category: models.CategoryArt, Date: "2024-10-05", StartTime: "17:00", EndTime: "21:00", Venue: "Downtown Art Gallery", Address: "78 Canvas Street, Gallery Row, NY 10003", Price: 15},
		{ID: "4", Title: "Jazz Night", Description: "An intimate evening of smooth jazz in a cozy underground venue.", Category: models.CategoryMusic, Date: "2024-10-02", StartTime: "20:00", EndTime: "23:00", Venue: "Blue Note Underground", Address: "12 Jazz Alley, Midtown, NY 10004", Price: 30},
		{ID: "5", Title: "Festival Gastronómico", Description: "Taste dishes from over 40 local restaurants and food trucks in one vibrant outdoor festival.", Category: models.CategoryFood, Date: "2024-10-05", StartTime: "12:00", EndTime: "20:00", Venue: "Central Park Lawn", Address: "Central Park West, NY 10023", Price: 20},
		{ID: "6", Title: "Neon Party", Description: "Dance the night away under neon lights with top DJs spinning the latest hits.", Category: models.CategoryParties, Date: "2024-10-08", StartTime: "22:00", EndTime: "04:00", Venue: "Electric Club", Address: "200 Neon Blvd, Downtown, NY 10005", Price: 25},
		{ID: "7", Title: "Yoga en el Parque", Description: "Start your morning with a guided yoga session in the park. All levels welcome.", Category: models.CategoryWellness, Date: "2024-10-15", StartTime: "08:00", EndTime: "09:30", Venue: "Riverside Park", Address: "Riverside Drive, NY 10024", Price: 0},
		{ID: "8", Title: "Neo-Soul Sunday", Description: "An afternoon of neo-soul music from the city's most exciting rising artists.", Category: models.CategoryMusic, Date: "2024-10-13", StartTime: "15:00", EndTime: "19:00", Venue: "Rooftop Lounge", Address: "55 Soul Street, Harlem, NY 10026", Price: 35},
		{ID: "9", Title: "Techno Warehouse", Description: "A raw industrial setting meets cutting-edge electronic music. One night only.", Category: models.CategoryMusic, Date: "2024-10-20", StartTime: "23:00", EndTime: "06:00", Venue: "The Warehouse", Address: "9 Industrial Ave, Brooklyn, NY 11201", Price: 40},
	}
	for _, e := range events {
		if err := s.Create(e); err != nil {
			log.Printf("skip event %s: %v", e.Title, err)
		} else {
			log.Printf("created event %s", e.Title)
		}
	}
}
