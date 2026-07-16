# Ride-Surge Architecture Guide

This document details the system design, real-time data flows, and architectural relationships of the Ride-Surge Live Engine.

---

## 🏗️ System Architecture Overview

The Ride-Surge Live Engine is built on a **reactive event-driven architecture**. It uses real-time data streaming to simulate driver movement, calculate surge multipliers based on spatial supply/demand, and perform geospatial ride matching.

### High-Level Topology

```mermaid
graph TD
    subgraph Vercel Frontend
        A[Angular Single Page App]
        Leaflet[Leaflet Map Layer]
        Logs[Terminal Log Viewer]
    end

    subgraph Render Go Backend
        B[Go Gin Web Server]
        Sim[Simulation Loop]
        PriceEng[Surge Pricing Engine]
        MatchEng[Geospatial Matcher]
        Hub[WebSocket Hub]
    end

    subgraph Cloud Storage & Event Streaming
        Supabase[(Supabase PostgreSQL)]
        Upstash[(Upstash Redis Geo-Index)]
        Aiven[Aiven Apache Kafka]
    end

    %% Web Connections
    A <-->|WebSockets /ws| Hub
    A -->|HTTP POST /api/rides/request| B

    %% Simulator Loop writes
    Sim -->|GEOADD| Upstash
    Sim -->|Publish Location| Aiven
    Sim -->|Broadcast Coordinates| Hub

    %% Ride Request flow
    B -->|HTTP Request Received| PriceEng
    PriceEng -->|Publish Request| Aiven
    MatchEng -->|GEORADIUS Query| Upstash
    MatchEng -->|Insert Ride Record| Supabase
    MatchEng -->|Publish Match| Aiven
    
    %% Event Pipeline
    Aiven -->|Kafka Log Consumer| Hub
```

---

## 🔄 Real-Time Event Flows

Here is the exact step-by-step lifecycle of events during simulation and booking operations:

### 1. Driver Location updates (Every 1.5s / Simulation Speed)
1. The **Simulator** runs a loop modifying driver coordinates.
2. The new coordinate is sent to **Upstash Redis** using `GEOADD` to update the spatial index.
3. The coordinate update is published to **Aiven Kafka** on the `driver-location-updated` topic.
4. The Go **WebSocket Hub** broadcasts a `DRIVER_LOCATION` message to all connected Angular clients.
5. The **Leaflet Map** receives the update and smoothly glides (interpolates) the corresponding marker.

### 2. Ride Request & Dispatch Pipeline
```mermaid
sequenceDiagram
    autonumber
    actor Passenger as Passenger (Vercel)
    participant Backend as Go Backend (Render)
    participant Redis as Redis Geo-Index (Upstash)
    participant Kafka as Kafka Broker (Aiven)
    participant Postgres as PostgreSQL (Supabase)

    Passenger->>Backend: HTTP POST /api/rides/request (Pickup/Dest coords)
    Note over Backend: Record demand in zone & update surge multiplier
    Backend->>Kafka: Publish Event (Topic: ride-requested)
    Backend->>Redis: GEORADIUS query (Find nearest available driver within 5km)
    
    alt Driver Found
        Redis-->>Backend: Returns nearest driver (e.g. Ramesh)
        Note over Backend: Set driver status to "busy"
        Backend->>Postgres: INSERT INTO rides (id, passenger_id, driver_id, fare, surge, status)
        Backend->>Kafka: Publish Event (Topic: ride-matched)
        Backend-->>Passenger: WebSocket Broadcast [RIDE_MATCHED] (Display Route on Map)
    else No Driver Found
        Backend->>Kafka: Publish Event (Topic: ride-failed)
        Backend-->>Passenger: WebSocket Broadcast [RIDE_FAILED] (Show warning)
    end
```

---

## 🛠️ Technology Stack Breakdown

*   **Frontend**: Angular 18+, TypeScript, Leaflet Map API, HTML5 Canvas, and Vanilla CSS with Frosted-Glass overlays.
*   **Backend Server**: Go (Golang 1.25.0+), Gin Gonic HTTP Router, Gorilla WebSockets Hub.
*   **Message Broker**: Aiven Apache Kafka (GCP / Singapore) using SASL_SSL SCRAM-SHA-256 for secure event streaming.
*   **Geospatial Cache**: Upstash Redis Server utilizing Redis `GEO` operations for low-latency location indexing.
*   **Relational Database**: Supabase PostgreSQL with transaction pooling over port `6543`.
