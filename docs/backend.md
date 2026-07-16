# Backend Code & API Guide

This document describes the Go backend services, data schemas, message broker topics, and API route contracts of the Ride-Surge Engine.

---

## 📂 Go Package Structure

The backend is built as a modular Go application located in the `/backend` directory:

*   **`main.go`**: The server bootstrapper. It connects to the databases, spawns the simulator/pricing loops, spins up the Gorilla WebSocket hub, and configures the Gin routing engine.
*   **`simulator/`**: Manages the list of 15 drivers, generates their starting locations, and walks them randomly every 1.5 seconds. Implements thread-safe speed scaling via atomic duration swaps.
*   **`pricing/`**: Defines the 3x3 Hyderabad grid layout. Calculates surge multipliers dynamically:
    $$\text{Surge} = 1.0 + \max\left(0, \frac{\text{Demand} - \text{Supply}}{\text{Supply} + 1}\right) \times 0.5$$
    with a maximum cap of `3.0x`.
*   **`matching/`**: Performs ride matching. Queries Upstash Redis to locate available drivers within 5km of the pickup coordinate, selects the nearest, assigns them, and flags them as `busy`.
*   **`store/`**: Data access layers:
    *   *Redis*: Configures connection handles and updates spatial indices.
    *   *Postgres*: Controls the connection pool and seeds the initial drivers list.
*   **`kafka/`**: Sets up SASL SCRAM-SHA-256 secure authentication over TLS, initializes required topics on startup, and exposes producer helpers to write events.

---

## 🗄️ Database Schemas

### 1. PostgreSQL (Supabase)
We maintain two tables in our Supabase schema:

#### `drivers` Table:
Stores static driver registration details:
```sql
CREATE TABLE IF NOT EXISTS drivers (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### `rides` Table:
Stores the history of matched rides:
```sql
CREATE TABLE IF NOT EXISTS rides (
    id SERIAL PRIMARY KEY,
    passenger_id VARCHAR(50) NOT NULL,
    driver_id VARCHAR(50) REFERENCES drivers(id),
    pickup_lat DOUBLE PRECISION NOT NULL,
    pickup_lng DOUBLE PRECISION NOT NULL,
    destination_lat DOUBLE PRECISION NOT NULL,
    destination_lng DOUBLE PRECISION NOT NULL,
    fare DOUBLE PRECISION NOT NULL,
    surge DOUBLE PRECISION NOT NULL,
    status VARCHAR(20) NOT NULL, -- 'matched', 'completed', 'failed'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 2. Redis Key Structure
We use a single Redis Geo-Spatial key:
*   **Key**: `drivers` (Geo-Set)
*   **Command**: `GEOADD drivers <lng> <lat> <driver_id>`
*   **Lookup**: `GEORADIUS drivers <lng> <lat> 5 km WITHDIST ASC LIMIT 1`

---

## 📡 Message Broker (Kafka Topics)

We stream JSON event records to four topics on Aiven Kafka:
1.  **`driver-location-updated`**: Broadcasts latitude, longitude, and availability status changes for simulated drivers.
2.  **`ride-requested`**: Triggers when a passenger requests a ride.
3.  **`ride-matched`**: Triggers when the matching algorithm successfully assigns a driver.
4.  **`surge-calculated`**: Emitted when the pricing engine updates zone multipliers.

---

## 🔌 API Routes Specifications

### 1. Post Simulation Speed
*   **Path**: `POST /api/simulator/speed`
*   **Request Body**:
    ```json
    {
      "speed": 5.0
    }
    ```
*   **Response Body (200 OK)**:
    ```json
    {
      "status": "success",
      "interval_ms": 300
    }
    ```

### 2. Request a Ride
*   **Path**: `POST /api/rides/request`
*   **Request Body**:
    ```json
    {
      "passenger_id": "user-1",
      "pickup_lat": 17.4344,
      "pickup_lng": 78.3866,
      "destination_lat": 17.4567,
      "destination_lng": 78.4012
    }
    ```
*   **Response Body (200 OK)**:
    ```json
    {
      "status": "matched",
      "ride_id": 14,
      "driver": {
        "id": "Ramesh",
        "name": "Ramesh",
        "latitude": 17.4350,
        "longitude": 78.3872,
        "status": "busy"
      }
    }
    ```

### 3. Get Pricing Zones
*   **Path**: `GET /api/zones`
*   **Response Body (200 OK)**:
    ```json
    {
      "Zone-0-0": {
        "id": "Zone-0-0",
        "name": "Hyderabad Sector A1",
        "min_lat": 17.3894,
        "max_lat": 17.4194,
        "min_lng": 78.3416,
        "max_lng": 78.3716,
        "surge": 1.25,
        "demand_counter": 2,
        "supply_counter": 4
      }
    }
    ```

### 4. Real-time Events WebSocket
*   **Path**: `GET /ws`
*   **Protocol**: WebSocket Upgrade (`ws:` / `wss:`)
*   **Connection Frame**: Establishes raw socket connection. The backend writes events as JSON text strings with a type selector (e.g. `DRIVER_LOCATION`, `SURGE_UPDATE`, `RIDE_MATCHED`).
