package simulator

import (
	"math/rand"
	"time"
)

type Driver struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Status    string  `json:"status"` // "available", "busy", "offline"
}

// Center of Hyderabad, India
const (
	CenterLat = 17.385044
	CenterLng = 78.486671
	GeoOffset = 0.035 // Boundary range for simulator
)

// ActiveDrivers stores our in-memory driver references
var ActiveDrivers = make(map[string]*Driver)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateDrivers initializes a set of simulated drivers
func GenerateDrivers(count int) {
	names := []string{
		"Ramesh", "Suresh", "Rahul", "Priya", "Amit",
		"Sunita", "Vikram", "Neha", "Rajesh", "Kiran",
		"Anjali", "Arjun", "Deepika", "Karthik", "Sneha",
	}

	for i := 1; i <= count; i++ {
		driverID := string(rune('A' + (i-1)%26)) + string(rune('0'+(i/26))) // A0, B0...
		if i <= len(names) {
			driverID = names[i-1]
		} else {
			driverID = names[rand.Intn(len(names))] + "-" + driverID
		}

		// Random initial coordinate within bounds
		lat := CenterLat + (rand.Float64()*2-1)*GeoOffset
		lng := CenterLng + (rand.Float64()*2-1)*GeoOffset

		ActiveDrivers[driverID] = &Driver{
			ID:        driverID,
			Name:      driverID,
			Latitude:  lat,
			Longitude: lng,
			Status:    "available",
		}
	}
}

// StartSimulation moves drivers randomly and triggers callbacks
func StartSimulation(interval time.Duration, onUpdate func(*Driver)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			for _, driver := range ActiveDrivers {
				if driver.Status == "busy" {
					continue
				}

				// Small random walk (approx 10-50 meters)
				latDelta := (rand.Float64() - 0.5) * 0.0005
				lngDelta := (rand.Float64() - 0.5) * 0.0005

				driver.Latitude += latDelta
				driver.Longitude += lngDelta

				// Stay within boundary
				if driver.Latitude > CenterLat+GeoOffset || driver.Latitude < CenterLat-GeoOffset {
					driver.Latitude = CenterLat + (rand.Float64()*2-1)*GeoOffset
				}
				if driver.Longitude > CenterLng+GeoOffset || driver.Longitude < CenterLng-GeoOffset {
					driver.Longitude = CenterLng + (rand.Float64()*2-1)*GeoOffset
				}

				onUpdate(driver)
			}
		}
	}()
}
