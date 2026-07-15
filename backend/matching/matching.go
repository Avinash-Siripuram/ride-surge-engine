package matching

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Avinash-Siripuram/ride-surge-engine/backend/kafka"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/simulator"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/store"
)

type RideRequest struct {
	ID             int     `json:"id,omitempty"`
	PassengerID    string  `json:"passenger_id"`
	PickupLat      float64 `json:"pickup_lat"`
	PickupLng      float64 `json:"pickup_lng"`
	DestinationLat float64 `json:"destination_lat"`
	DestinationLng float64 `json:"destination_lng"`
	Fare           float64 `json:"fare"`
	Surge          float64 `json:"surge"`
	Status         string  `json:"status"`
}

func MatchRide(req *RideRequest) (*simulator.Driver, error) {
	// 1. Find nearest drivers in Redis (5 km radius)
	nearby, err := store.FindNearbyDrivers(req.PickupLat, req.PickupLng, 5.0)
	if err != nil {
		return nil, err
	}

	var selectedDriver *simulator.Driver
	for _, geoLoc := range nearby {
		driver, exists := simulator.ActiveDrivers[geoLoc.Name]
		if exists && driver.Status == "available" {
			selectedDriver = driver
			break
		}
	}

	if selectedDriver == nil {
		return nil, fmt.Errorf("no available drivers nearby")
	}

	// 2. Mark driver as busy
	selectedDriver.Status = "busy"
	_, err = store.DB.Exec("UPDATE drivers SET status = 'busy' WHERE id = $1", selectedDriver.ID)
	if err != nil {
		log.Printf("Failed to update driver status in Postgres: %v", err)
	}

	// 3. Save Ride to Postgres
	var rideID int
	err = store.DB.QueryRow(`
		INSERT INTO rides (passenger_id, driver_id, status, pickup_lat, pickup_lng, destination_lat, destination_lng, fare, surge_multiplier)
		VALUES ($1, $2, 'matched', $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		req.PassengerID, selectedDriver.ID, req.PickupLat, req.PickupLng, req.DestinationLat, req.DestinationLng, req.Fare, req.Surge,
	).Scan(&rideID)
	if err != nil {
		log.Printf("Failed to save ride in DB: %v", err)
	}
	req.ID = rideID
	req.Status = "matched"

	// 4. Publish to Kafka
	reqBytes, _ := json.Marshal(req)
	kafka.PublishEvent(kafka.TopicRideMatched, fmt.Sprintf("%d", rideID), reqBytes)

	// 5. Simulate ride transit (completes after 12 seconds)
	go func(d *simulator.Driver, rId int) {
		log.Printf("Simulating ride %d. Driver %s is on the way...", rId, d.Name)
		time.Sleep(12 * time.Second)

		// Complete ride
		_, err := store.DB.Exec("UPDATE rides SET status = 'completed' WHERE id = $1", rId)
		if err != nil {
			log.Printf("Failed to complete ride: %v", err)
		}

		d.Status = "available"
		_, err = store.DB.Exec("UPDATE drivers SET status = 'available' WHERE id = $1", d.ID)
		if err != nil {
			log.Printf("Failed to release driver: %v", err)
		}
		log.Printf("Ride %d completed. Driver %s is available.", rId, d.Name)
	}(selectedDriver, rideID)

	return selectedDriver, nil
}
