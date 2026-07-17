# Software Engineering Resume Guide - Ride-Surge Live Engine

This document contains high-impact technical descriptions, metrics, and bullet points for the **Ride-Surge Live Engine** project that you can add directly to your software engineering resume or feed into an AI resume assistant.

---

## 💼 Proposed Resume Project Entry

### **Ride-Surge Live Engine** | *Go, Angular, Apache Kafka, Redis, PostgreSQL, WebSockets*

*   **Real-time Event-Driven Architecture**: Designed and built a reactive, event-driven ride dispatch simulation engine using **Go** and **Apache Kafka** to stream, publish, and consume real-time driver coordinates and ride requests with sub-second latency.
*   **Geospatial Optimization**: Integrated a serverless **Redis Geo-Spatial Index** to perform low-latency radius searches (`GEORADIUS`), reducing nearest-available-driver lookup and match calculations to **under 10ms** for active ride bookings.
*   **Dynamic Spatial Surge Pricing**: Implemented a dynamic surge pricing algorithm in **Go** that monitors real-time supply/demand ratios within a geofenced 3x3 coordinate grid, dynamically updating zone multipliers (up to 3.0x) and broadcasting updates to clients.
*   **High-Performance Frontend Rendering**: Engineered a responsive glassmorphic **Angular** dashboard integrating Leaflet maps, featuring smooth marker location interpolation using `requestAnimationFrame` to eliminate marker jumpiness and reduce client CPU load.
*   **Database Scaling & Networking**: Configured a **Supabase PostgreSQL** database with transaction pooling over port `6543`, overcoming outbound container IPv6 NAT limitations and maintaining resilient data persistence for completed match workflows.
*   **Zero-Cost Multi-Cloud Infrastructure**: Wired and deployed a decoupled cloud infrastructure using **Render** (backend), **Vercel** (frontend), **Upstash** (Redis), **Aiven** (Kafka), and **Supabase** (Postgres) with automated rolling, zero-downtime deployment pipelines.

---

## 🛠️ Technology Stack (For your Skills section)

*   **Languages**: Go (Golang 1.25+), TypeScript, SQL, HTML5, Vanilla CSS
*   **Frameworks & Libraries**: Angular 18+, Gin Gonic (Go HTTP framework), Leaflet Map API, Gorilla WebSockets
*   **Databases & Caching**: PostgreSQL (Supabase), Redis (Upstash Geo-Sets)
*   **Infrastructure & DevOps**: Apache Kafka (Aiven), Vercel, Render, Git, GitHub Actions, Docker (Dockerfile)

---

## 📈 System Metrics & "Why it was built this way" (Q&A Prep)

If asked about the project's engineering details in an interview, here are the key technical justifications to use:

1.  **Why Go?**
    *   *Answer*: Go's lightweight runtime, high-performance concurrency model (Goroutines), and low memory footprint made it ideal for simulating multiple concurrent driver agents moving in real-time.
2.  **Why Apache Kafka instead of standard HTTP?**
    *   *Answer*: Ride dispatch is a high-volume, event-driven stream. Kafka acts as a durable, distributed write-ahead log that decouples coordinate tracking from database operations, ensuring the system remains responsive even under high load.
3.  **Why Redis for driver matching?**
    *   *Answer*: Relational databases (like PostgreSQL) are too slow for calculating distances dynamically across moving drivers. Redis holds the spatial index in-memory and provides built-in `GEORADIUS` commands to calculate the nearest driver within 5km in under 10ms.
4.  **Why the Supabase Connection Pooler on Port 6543?**
    *   *Answer*: Standard free containers (Render/Back4app) restrict outbound IPv6 calls. Supabase direct connections use IPv6, which caused connection timeouts. Switching to the transaction pooler on port 6543 resolved this by routing traffic over IPv4.
