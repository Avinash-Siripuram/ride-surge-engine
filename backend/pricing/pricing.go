package pricing

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/Avinash-Siripuram/ride-surge-engine/backend/kafka"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/simulator"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/store"
)

type Zone struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	MinLat         float64 `json:"min_lat"`
	MaxLat         float64 `json:"max_lat"`
	MinLng         float64 `json:"min_lng"`
	MaxLng         float64 `json:"max_lng"`
	Surge          float64 `json:"surge"`
	DemandCounter  int     `json:"demand_counter"`
	SupplyCounter  int     `json:"supply_counter"`
}

var (
	Zones   = make(map[string]*Zone)
	zonesMu sync.RWMutex
)

// Initialize zones around Hyderabad
func InitZones() {
	zonesMu.Lock()
	defer zonesMu.Unlock()

	gridSize := 3 // 3x3 grid
	latStep := (simulator.GeoOffset * 2) / float64(gridSize)
	lngStep := (simulator.GeoOffset * 2) / float64(gridSize)

	for i := 0; i < gridSize; i++ {
		for j := 0; j < gridSize; j++ {
			zoneID := fmt.Sprintf("Zone-%d-%d", i, j)
			minLat := (simulator.CenterLat - simulator.GeoOffset) + float64(i)*latStep
			maxLat := minLat + latStep
			minLng := (simulator.CenterLng - simulator.GeoOffset) + float64(j)*lngStep
			maxLng := minLng + lngStep

			Zones[zoneID] = &Zone{
				ID:             zoneID,
				Name:           fmt.Sprintf("Hyderabad Sector %c%d", 'A'+i, j+1),
				MinLat:         minLat,
				MaxLat:         maxLat,
				MinLng:         minLng,
				MaxLng:         maxLng,
				Surge:          1.0,
				DemandCounter:  0,
				SupplyCounter:  0,
			}
		}
	}
}

// GetZoneForCoordinates maps GPS to a grid zone
func GetZoneForCoordinates(lat, lng float64) *Zone {
	zonesMu.RLock()
	defer zonesMu.RUnlock()

	for _, zone := range Zones {
		if lat >= zone.MinLat && lat <= zone.MaxLat && lng >= zone.MinLng && lng <= zone.MaxLng {
			return zone
		}
	}
	return nil
}

// RecordDemand increments demand in a zone
func RecordDemand(lat, lng float64) {
	zone := GetZoneForCoordinates(lat, lng)
	if zone != nil {
		zonesMu.Lock()
		zone.DemandCounter++
		zonesMu.Unlock()
	}
}

// StartSurgePricingEngine runs the loop to periodically recalculate surges
func StartSurgePricingEngine(interval time.Duration, onSurgeUpdate func(map[string]*Zone)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			zonesMu.Lock()
			// Update supply count from Redis
			for _, zone := range Zones {
				centerLat := (zone.MinLat + zone.MaxLat) / 2
				centerLng := (zone.MinLng + zone.MaxLng) / 2

				// Find active drivers in this zone using Redis Geo
				// Approximate half-width of zone in km is roughly 1.5 km
				drivers, err := store.FindNearbyDrivers(centerLat, centerLng, 2.0)
				if err != nil {
					log.Printf("Failed to count drivers for surge in %s: %v", zone.ID, err)
					zone.SupplyCounter = 0
				} else {
					// Only count available ones
					availCount := 0
					for _, dLoc := range drivers {
						if d, exists := simulator.ActiveDrivers[dLoc.Name]; exists && d.Status == "available" {
							availCount++
						}
					}
					zone.SupplyCounter = availCount
				}

				// Calculate surge pricing multiplier
				// Base pricing multiplier = 1.0
				// Ratio = Demand / Supply (if supply > 0)
				surge := 1.0
				if zone.DemandCounter > 0 {
					if zone.SupplyCounter == 0 {
						surge = 2.5 // Max cap when no supply
					} else {
						ratio := float64(zone.DemandCounter) / float64(zone.SupplyCounter)
						if ratio > 1.0 {
							// Surge grows log-scale or linearly with cap at 3.0
							surge = 1.0 + (ratio-1.0)*0.5
							surge = math.Min(3.0, math.Max(1.0, surge))
						}
					}
				}

				zone.Surge = math.Round(surge*10) / 10

				// Decay demand counter slightly so it isn't permanent
				zone.DemandCounter = int(float64(zone.DemandCounter) * 0.5)
			}

			// Broadcast surge updates
			onSurgeUpdate(Zones)

			// Publish to Kafka
			zonesBytes, _ := json.Marshal(Zones)
			kafka.PublishEvent(kafka.TopicSurgeCalculated, "global", zonesBytes)

			zonesMu.Unlock()
		}
	}()
}
