// Package mock generates synthetic events for local development and load testing.
package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/google/uuid"
)

// Bounding box (central Asturias) that every generated event falls within.
const (
	MinLon = -6.103899666699192
	MaxLon = -5.508319449733364
	MinLat = 43.20933179946499
	MaxLat = 43.55645129413614
)

var (
	// Inclusive date range for generated events.
	StartDate = time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	EndDate   = time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)

	Categories = []models.EventCategory{
		models.CategoryMusic,
		models.CategoryTheater,
		models.CategoryParties,
		models.CategorySports,
		models.CategoryFood,
		models.CategoryArt,
		models.CategoryWellness,
	}
)

// GenerateEvents returns n synthetic events. Categories and days are assigned
// round-robin so every category and every day in [StartDate, EndDate] is
// covered once n is large enough; the remaining fields are random but
// deterministic for a given seed.
func GenerateEvents(n int, seed int64) []*models.Event {
	r := rand.New(rand.NewSource(seed))
	days := int(EndDate.Sub(StartDate).Hours()/24) + 1

	events := make([]*models.Event, 0, n)
	for i := 0; i < n; i++ {
		seq := i + 1
		date := StartDate.AddDate(0, 0, i%days)

		// Start between 08:00 and 20:30 in 30-minute slots; lasts 1–3.5h so it ends the same day.
		startMin := 8*60 + r.Intn(26)*30
		endMin := startMin + 60 + r.Intn(6)*30

		lat := MinLat + r.Float64()*(MaxLat-MinLat)
		lon := MinLon + r.Float64()*(MaxLon-MinLon)

		// ~20% free events; the rest priced 5–100 € in 0.5 steps.
		var price float64
		if r.Intn(5) != 0 {
			price = 5 + float64(r.Intn(191))*0.5
		}

		events = append(events, &models.Event{
			ID:          uuid.NewString(),
			Title:       fmt.Sprintf("Event %04d", seq),
			Description: fmt.Sprintf("Description %04d", seq),
			Category:    Categories[i%len(Categories)],
			Date:        date.Format("2006-01-02"),
			StartTime:   formatHM(startMin),
			EndTime:     formatHM(endMin),
			Venue:       fmt.Sprintf("Venue %04d", seq),
			Address:     fmt.Sprintf("Address %04d", seq),
			Price:       price,
			Lat:         &lat,
			Lon:         &lon,
		})
	}
	return events
}

func formatHM(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}
