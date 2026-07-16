# Frontend Visuals & Map Guide

This document describes the Angular architecture, Leaflet map integration, and coordinate interpolation calculations of the Ride-Surge Live Engine.

---

## 🏛️ Angular Component Architecture

The frontend is implemented as a standalone Angular component in `/frontend/src/app`:

*   **`app.ts`**: The core controller. Manages state signals, coordinates Leaflet layers, runs the marker animation loop, and subscribes to websocket streams.
*   **`app.html`**: Layout grid. Floats glassmorphic sidebar panels over the full-height Leaflet container and formats the UNIX-like developer console.
*   **`app.css`**: Styling rules. Configures the backdrop blurs, slider knobs, terminal pulse states, and Leaflet marker animations.
*   **`services/websocket.service.ts`**: Connects to the backend WebSocket server (`/ws`). Maps incoming socket frames to RxJS Observables (`driverLocation$`, `surgeUpdate$`, etc.) to trigger reactive UI bindings.
*   **`services/ride.service.ts`**: Handles REST API calls like `/api/rides/request`, `/api/zones`, and simulator speed changes, as well as Nominatim lookup queries.

---

## 🗺️ Leaflet Map Management

We utilize Leaflet for real-time spatial display:

### 1. Layer Architecture
*   **Base Tiles**: Rendered using either OpenStreetMap or CartoDB Dark Matter.
*   **Zone Boundaries**: Rendered as Leaflet `L.polygon` layers. The fill opacity and border color reflect the zone's surge level.
*   **Driver Markers**: Custom `L.divIcon` markers containing colored indicator dots.
*   **Route Layers**: Temporary dashed `L.polyline` connecting pickup and destination coordinates during an active booking.

### 2. Map Theme Toggling
On clicking the theme button, the existing layer is dynamically disposed and replaced:
```typescript
const url = this.isDarkTheme
    ? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
    : 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
```

---

## 🏎️ Smooth Coordinate Interpolation

Instead of letting markers jump instantly when a coordinate update is received, we interpolate positions continuously in real time using `requestAnimationFrame`.

### The Calculation
For every frame (roughly every 16ms):
1.  We calculate the elapsed time since the transition started.
2.  Compute the progress factor $t$ (capped at `1.0`):
    $$t = \min\left(1.0, \frac{\text{Current Time} - \text{Start Time}}{\text{Duration}}\right)$$
3.  Interpolate the coordinates linearly between the start position and the target position:
    $$\text{Latitude} = \text{Lat}_{\text{start}} + (\text{Lat}_{\text{end}} - \text{Lat}_{\text{start}}) \times t$$
    $$\text{Longitude} = \text{Lng}_{\text{start}} + (\text{Lng}_{\text{end}} - \text{Lng}_{\text{start}}) \times t$$

### Speed Adjustments
The duration of the interpolation is scaled dynamically using the `simulationSpeed` factor:
$$\text{Duration} = \frac{1500\text{ ms}}{\text{Simulation Speed}}$$
This ensures that if the simulation is set to 5x speed, the gliding transitions accelerate accordingly to stay perfectly in sync with the backend updates!
