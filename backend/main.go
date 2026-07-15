package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Avinash-Siripuram/ride-surge-engine/backend/hub"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/kafka"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/matching"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/pricing"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/simulator"
	"github.com/Avinash-Siripuram/ride-surge-engine/backend/store"
)

func main() {
	log.Println("Starting Ride-Matching & Dynamic Pricing Engine...")

	// 1. Initialize Stores
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	pgDsn := os.Getenv("DATABASE_URL")
	if pgDsn == "" {
		pgDsn = "host=localhost port=5435 user=postgres password=postgrespassword dbname=ridesurge sslmode=disable"
	}

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	store.InitRedis(redisAddr)
	store.InitDB(pgDsn)
	kafka.InitKafka(kafkaBroker)
	defer kafka.CloseKafka()

	// 2. Initialize simulation data
	driverCount := 15
	simulator.GenerateDrivers(driverCount)

	// Seed drivers into Postgres
	driverSeedData := make(map[string]string)
	for id, d := range simulator.ActiveDrivers {
		driverSeedData[id] = d.Name
	}
	store.SeedDrivers(driverSeedData)

	// 3. Initialize pricing zones
	pricing.InitZones()

	// 4. Start WebSocket Hub
	wsHub := hub.NewHub()
	go wsHub.Run()

	// 5. Start Simulator & store updates in Redis
	// Move drivers every 1.5 seconds and broadcast coordinates + publish to Kafka
	simulator.StartSimulation(1500*time.Millisecond, func(d *simulator.Driver) {
		// Update Redis Geo index
		err := store.UpdateDriverLocation(d.ID, d.Latitude, d.Longitude)
		if err != nil {
			log.Printf("Error updating driver %s in Redis: %v", d.ID, err)
		}

		// Publish location update to Kafka
		driverBytes, _ := json.Marshal(d)
		kafka.PublishEvent(kafka.TopicLocationUpdated, d.ID, driverBytes)

		// Broadcast coordinates to websocket clients
		wsHub.BroadcastJSON("DRIVER_LOCATION", d)
	})

	// 6. Start Pricing Engine (runs every 5 seconds)
	pricing.StartSurgePricingEngine(5*time.Second, func(zones map[string]*pricing.Zone) {
		wsHub.BroadcastJSON("SURGE_UPDATE", zones)
	})

	// 7. Setup REST + WebSocket server
	router := gin.Default()

	// CORS Middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Endpoints
	router.GET("/ws", func(c *gin.Context) {
		hub.ServeWS(wsHub, c.Writer, c.Request)
	})

	router.GET("/api/zones", func(c *gin.Context) {
		c.JSON(http.StatusOK, pricing.Zones)
	})

	router.GET("/api/drivers", func(c *gin.Context) {
		c.JSON(http.StatusOK, simulator.ActiveDrivers)
	})

	router.POST("/api/rides/request", func(c *gin.Context) {
		var req matching.RideRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Calculate base fare ($2 base + $1.5 per random distance)
		req.Fare = 5.0

		// Record demand in the corresponding zone
		pricing.RecordDemand(req.PickupLat, req.PickupLng)

		// Fetch current surge multiplier for pickup zone
		zone := pricing.GetZoneForCoordinates(req.PickupLat, req.PickupLng)
		req.Surge = 1.0
		if zone != nil {
			req.Surge = zone.Surge
			req.Fare = req.Fare * req.Surge
		}

		// Publish RideRequested to Kafka
		reqBytes, _ := json.Marshal(req)
		kafka.PublishEvent(kafka.TopicRideRequested, req.PassengerID, reqBytes)

		// Broadcast ride requested state
		wsHub.BroadcastJSON("RIDE_REQUESTED", req)

		// Trigger matching algorithm
		driver, err := matching.MatchRide(&req)
		if err != nil {
			req.Status = "failed"
			wsHub.BroadcastJSON("RIDE_FAILED", req)
			c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": err.Error()})
			return
		}

		// Broadcast ride matched state
		wsHub.BroadcastJSON("RIDE_MATCHED", map[string]interface{}{
			"ride":   req,
			"driver": driver,
		})

		c.JSON(http.StatusOK, gin.H{
			"status": "matched",
			"ride":   req,
			"driver": driver,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server listening on port %s\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to run HTTP server: %v", err)
	}
}
