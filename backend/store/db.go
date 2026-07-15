package store

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB(dataSourceName string) {
	var err error
	DB, err = sql.Open("postgres", dataSourceName)
	if err != nil {
		log.Fatalf("Failed to open Postgres connection: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("Failed to ping Postgres: %v", err)
	}

	log.Println("Connected to PostgreSQL successfully")
	createTables()
}

func createTables() {
	queryDrivers := `
	CREATE TABLE IF NOT EXISTS drivers (
		id VARCHAR(50) PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		status VARCHAR(20) DEFAULT 'available',
		rating REAL DEFAULT 4.8
	);`

	queryRides := `
	CREATE TABLE IF NOT EXISTS rides (
		id SERIAL PRIMARY KEY,
		passenger_id VARCHAR(50) NOT NULL,
		driver_id VARCHAR(50) REFERENCES drivers(id),
		status VARCHAR(20) NOT NULL, -- 'requested', 'matched', 'completed', 'canceled'
		pickup_lat DOUBLE PRECISION NOT NULL,
		pickup_lng DOUBLE PRECISION NOT NULL,
		destination_lat DOUBLE PRECISION NOT NULL,
		destination_lng DOUBLE PRECISION NOT NULL,
		fare DOUBLE PRECISION NOT NULL,
		surge_multiplier DOUBLE PRECISION DEFAULT 1.0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := DB.Exec(queryDrivers)
	if err != nil {
		log.Fatalf("Error creating drivers table: %v", err)
	}

	_, err = DB.Exec(queryRides)
	if err != nil {
		log.Fatalf("Error creating rides table: %v", err)
	}

	log.Println("PostgreSQL tables checked/created successfully")
}

// SeedDrivers populates postgres with default simulation drivers
func SeedDrivers(drivers map[string]string) {
	for id, name := range drivers {
		query := `
		INSERT INTO drivers (id, name, status)
		VALUES ($1, $2, 'available')
		ON CONFLICT (id) DO UPDATE SET status = 'available';`
		_, err := DB.Exec(query, id, name)
		if err != nil {
			log.Printf("Failed to seed driver %s: %v", err)
		}
	}
}
