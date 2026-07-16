import { Component, OnInit, OnDestroy, AfterViewInit, signal, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { WebSocketService } from './services/websocket.service';
import { RideService, RideRequestPayload } from './services/ride.service';
import { Subscription } from 'rxjs';
import * as L from 'leaflet';

interface ZoneData {
	id: string;
	name: string;
	min_lat: number;
	max_lat: number;
	min_lng: number;
	max_lng: number;
	surge: number;
	demand_counter: number;
	supply_counter: number;
}

@Component({
	selector: 'app-root',
	standalone: true,
	imports: [CommonModule, FormsModule],
	templateUrl: './app.html',
	styleUrl: './app.css'
})
export class App implements OnInit, OnDestroy, AfterViewInit {
	// Inject Services
	protected readonly wsService = inject(WebSocketService);
	private readonly rideService = inject(RideService);

	// Signals for UI State Binding
	protected readonly activeDriversCount = signal(0);
	protected readonly maxSurge = signal(1.0);
	protected readonly bookingStatus = signal<'Idle' | 'Searching' | 'Matched'>('Idle');
	protected readonly matchedDriver = signal<any>(null);
	protected readonly matchedRide = signal<any>(null);
	protected readonly busyDriversCount = signal(0);

	// Simulation controls
	protected simulationSpeed = 1.0;
	protected isDarkTheme = true;

	// Local properties
	protected availableZones: ZoneData[] = [];
	protected searchQueries = { pickup: '', destination: '' };
	protected searchResults = { pickup: [] as any[], destination: [] as any[] };
	protected selectedLocations = { pickup: null as any, destination: null as any };
	private searchDebounces = { pickup: null as any, destination: null as any };
	private searchMarkers = { pickup: null as L.Marker | null, destination: null as L.Marker | null };
	protected eventLogs: Array<{ title: string; description: string; type: string; time: Date }> = [];

	// Animation & state maps
	private driverAnimations = new Map<string, { startLat: number; startLng: number; endLat: number; endLng: number; startTime: number; duration: number }>();
	private driverStatuses = new Map<string, string>();
	private animationFrameId: number | null = null;

	// Leaflet Map References
	private map!: L.Map;
	private driverMarkers = new Map<string, L.Marker>();
	private zonePolygons = new Map<string, L.Polygon>();
	private activeRouteLayer: L.Polyline | null = null;
	private activeLocationMarkers: L.Marker[] = [];

	// Subscriptions
	private subs: Subscription[] = [];

	ngOnInit() {
		// Initialize event feed logs and default values
		this.loadInitialZones();
	}

	ngAfterViewInit() {
		this.initMap();
		this.subscribeToRealtimeEvents();
	}

	ngOnDestroy() {
		this.subs.forEach(s => s.unsubscribe());
		if (this.animationFrameId !== null) {
			cancelAnimationFrame(this.animationFrameId);
		}
	}

	private async loadInitialZones() {
		try {
			const zonesMap = await this.rideService.getZones();
			this.availableZones = Object.values(zonesMap);
			
			// Recalculate max surge
			this.updateMaxSurgeSignal();
		} catch (err) {
			console.error('Failed to load initial pricing zones:', err);
		}
	}

	private initMap() {
		// Centered around the Hyderabad Simulation Grid Center (Hitec City / Madhapur)
		this.map = L.map('map', {
			zoomControl: true,
			attributionControl: false
		}).setView([17.4344, 78.3866], 13);

		// Default to premium CartoDB Dark Matter theme
		this.tileLayer = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
			maxZoom: 19,
			attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
		}).addTo(this.map);

		// Render Zone Polygons
		this.drawZones();

		// Start smooth coordinate interpolation loop
		this.startAnimationLoop();
	}

	private drawZones() {
		// Remove existing polygons
		this.zonePolygons.forEach(p => p.remove());
		this.zonePolygons.clear();

		this.availableZones.forEach(zone => {
			const bounds: L.LatLngTuple[] = [
				[zone.min_lat, zone.min_lng],
				[zone.min_lat, zone.max_lng],
				[zone.max_lat, zone.max_lng],
				[zone.max_lat, zone.min_lng]
			];

			const color = this.getSurgeColor(zone.surge);
			const polygon = L.polygon(bounds, {
				color: color,
				weight: 1.5,
				fillColor: color,
				fillOpacity: 0.12,
				dashArray: '3'
			}).addTo(this.map);

			polygon.bindTooltip(`<strong>${zone.name}</strong><br>Surge: ${zone.surge}x`, {
				sticky: true,
				className: 'custom-zone-tooltip'
			});

			this.zonePolygons.set(zone.id, polygon);
		});
	}

	private getSurgeColor(surge: number): string {
		if (surge >= 2.0) return '#ef4444'; // Red
		if (surge >= 1.2) return '#f59e0b'; // Amber
		return '#10b981'; // Green
	}

	private updateMaxSurgeSignal() {
		const max = Math.max(...this.availableZones.map(z => z.surge), 1.0);
		this.maxSurge.set(max);
	}

	private subscribeToRealtimeEvents() {
		// 1. Driver Location Updates
		this.subs.push(
			this.wsService.driverLocation$.subscribe(driver => {
				this.updateDriverMarker(driver);
				this.updateActiveDriversCount();
			})
		);

		// 2. Zone Surge Price Updates
		this.subs.push(
			this.wsService.surgeUpdate$.subscribe(zonesMap => {
				this.availableZones = Object.values(zonesMap);
				this.drawZones();
				this.updateMaxSurgeSignal();
			})
		);

		// 3. Kafka Ride Requests
		this.subs.push(
			this.wsService.rideRequested$.subscribe(ride => {
				this.addEventLog('Ride Requested', `Passenger ${ride.passenger_id} requested a ride in ${this.getZoneName(ride.pickup_lat, ride.pickup_lng)}`, 'requested');
			})
		);

		// 4. Kafka Ride Matches
		this.subs.push(
			this.wsService.rideMatched$.subscribe(payload => {
				const { ride, driver } = payload;
				this.addEventLog('Driver Matched', `Ride #${ride.id} matched with driver ${driver.name} (Surge: ${ride.surge}x)`, 'matched');
				
				// If this is the user's booking, update state
				if (ride.passenger_id === 'user-1') {
					this.bookingStatus.set('Matched');
					this.matchedDriver.set(driver);
					this.matchedRide.set(ride);

					// Render route from pickup to destination
					this.renderActiveRideRoute(
						[ride.pickup_lat, ride.pickup_lng],
						[ride.destination_lat, ride.destination_lng]
					);

					// Reset status back to idle after 8 seconds
					setTimeout(() => {
						this.bookingStatus.set('Idle');
						this.clearActiveRideRoute();
					}, 8000);
				}
			})
		);

		// 5. Kafka Match Failure
		this.subs.push(
			this.wsService.rideFailed$.subscribe(ride => {
				if (ride.passenger_id === 'user-1') {
					this.bookingStatus.set('Idle');
					alert('No drivers available nearby. Please try again in a few seconds.');
				}
				this.addEventLog('Match Failed', `Dispatch failed to find a driver for passenger ${ride.passenger_id}`, 'failed');
			})
		);
	}

	private updateDriverMarker(driver: any) {
		const position: L.LatLngExpression = [driver.latitude, driver.longitude];
		this.driverStatuses.set(driver.id, driver.status);
		
		const shadowColor = driver.status === 'available' ? 'rgba(59, 130, 246, 0.6)' : 'rgba(239, 68, 68, 0.4)';
		const iconHtml = `<div style="
			width: 12px;
			height: 12px;
			border-radius: 50%;
			background-color: ${driver.status === 'available' ? '#3b82f6' : '#ef4444'};
			box-shadow: 0 0 10px ${shadowColor};
			border: 2px solid white;
		"></div>`;

		const customIcon = L.divIcon({
			html: iconHtml,
			className: 'custom-driver-icon',
			iconSize: [12, 12]
		});

		if (this.driverMarkers.has(driver.id)) {
			const marker = this.driverMarkers.get(driver.id)!;
			marker.setIcon(customIcon);
			marker.setTooltipContent(`Driver: ${driver.name}<br>Status: ${driver.status}`);

			// Start smooth gliding coordinate interpolation
			const currentLatLng = marker.getLatLng();
			this.driverAnimations.set(driver.id, {
				startLat: currentLatLng.lat,
				startLng: currentLatLng.lng,
				endLat: driver.latitude,
				endLng: driver.longitude,
				startTime: performance.now(),
				duration: 1500 / this.simulationSpeed
			});
		} else {
			const marker = L.marker(position, { icon: customIcon })
				.addTo(this.map)
				.bindTooltip(`Driver: ${driver.name}<br>Status: ${driver.status}`, {
					direction: 'top'
				});
			this.driverMarkers.set(driver.id, marker);
		}
	}

	private updateActiveDriversCount() {
		this.activeDriversCount.set(this.driverMarkers.size);
		let busy = 0;
		this.driverStatuses.forEach(status => {
			if (status === 'busy') busy++;
		});
		this.busyDriversCount.set(busy);
	}

	private getZoneName(lat: number, lng: number): string {
		const zone = this.availableZones.find(z => lat >= z.min_lat && lat <= z.max_lat && lng >= z.min_lng && lng <= z.max_lng);
		return zone ? zone.name : 'Unknown Sector';
	}

	protected onSearchInput(type: 'pickup' | 'destination') {
		const query = this.searchQueries[type];
		
		if (this.searchDebounces[type]) {
			clearTimeout(this.searchDebounces[type]);
		}

		if (!query || query.trim().length < 3) {
			this.searchResults[type] = [];
			return;
		}

		this.searchDebounces[type] = setTimeout(async () => {
			try {
				const results = await this.rideService.searchLocation(query);
				this.searchResults[type] = results;
			} catch (err) {
				console.error(err);
			}
		}, 400);
	}

	protected selectLocation(type: 'pickup' | 'destination', item: any) {
		this.selectedLocations[type] = item;
		this.searchQueries[type] = item.display_name;
		this.searchResults[type] = [];

		const lat = parseFloat(item.lat);
		const lon = parseFloat(item.lon);

		this.placeLocationMarker(type, [lat, lon], item.display_name.split(',')[0]);

		if (this.selectedLocations.pickup && this.selectedLocations.destination) {
			const pLat = parseFloat(this.selectedLocations.pickup.lat);
			const pLon = parseFloat(this.selectedLocations.pickup.lon);
			const dLat = parseFloat(this.selectedLocations.destination.lat);
			const dLon = parseFloat(this.selectedLocations.destination.lon);
			const bounds = L.latLngBounds([[pLat, pLon], [dLat, dLon]]);
			this.map.fitBounds(bounds, { padding: [50, 50] });
		} else {
			this.map.setView([lat, lon], 14);
		}
	}

	private placeLocationMarker(type: 'pickup' | 'destination', latlng: L.LatLngTuple, title: string) {
		if (this.searchMarkers[type]) {
			this.searchMarkers[type]!.remove();
		}

		const color = type === 'pickup' ? '#f59e0b' : '#ef4444';
		const iconHtml = `<div style="
			width: 14px;
			height: 14px;
			border-radius: 50%;
			background-color: ${color};
			box-shadow: 0 0 10px ${color};
			border: 2.5px solid white;
		"></div>`;

		const customIcon = L.divIcon({
			html: iconHtml,
			iconSize: [14, 14],
			className: 'custom-location-marker'
		});

		this.searchMarkers[type] = L.marker(latlng, { icon: customIcon })
			.addTo(this.map)
			.bindTooltip(title, { permanent: true, direction: 'top' });
	}

	protected bookRide(event: Event) {
		event.preventDefault();
		if (this.bookingStatus() === 'Searching') return;

		const pickup = this.selectedLocations.pickup;
		const dest = this.selectedLocations.destination;

		if (!pickup || !dest) return;

		const payload: RideRequestPayload = {
			passenger_id: 'user-1',
			pickup_lat: parseFloat(pickup.lat),
			pickup_lng: parseFloat(pickup.lon),
			destination_lat: parseFloat(dest.lat),
			destination_lng: parseFloat(dest.lon)
		};

		this.bookingStatus.set('Searching');
		this.matchedDriver.set(null);
		this.matchedRide.set(null);

		this.rideService.requestRide(payload).catch(err => {
			console.error('Ride request error:', err);
			this.bookingStatus.set('Idle');
		});
	}

	private renderActiveRideRoute(pickup: L.LatLngTuple, dest: L.LatLngTuple) {
		this.clearActiveRideRoute();

		// Add markers for pickup (neon circle pulse) and destination
		const pickupIcon = L.divIcon({
			html: `<div style="width: 16px; height: 16px; border-radius: 50%; background: #f59e0b; border: 3px solid white; box-shadow: 0 0 12px #f59e0b; animation: pulse 1.5s infinite;"></div>`,
			iconSize: [16, 16]
		});
		
		const destIcon = L.divIcon({
			html: `<div style="width: 16px; height: 16px; border-radius: 50%; background: #ef4444; border: 3px solid white; box-shadow: 0 0 12px #ef4444;"></div>`,
			iconSize: [16, 16]
		});

		const marker1 = L.marker(pickup, { icon: pickupIcon }).addTo(this.map).bindTooltip('Pickup');
		const marker2 = L.marker(dest, { icon: destIcon }).addTo(this.map).bindTooltip('Destination');
		this.activeLocationMarkers.push(marker1, marker2);

		// Draw path
		this.activeRouteLayer = L.polyline([pickup, dest], {
			color: '#a78bfa',
			weight: 4,
			opacity: 0.8,
			dashArray: '5, 8'
		}).addTo(this.map);

		// Zoom map to fit route bounds
		const bounds = L.latLngBounds([pickup, dest]);
		this.map.fitBounds(bounds, { padding: [50, 50] });
	}

	private clearActiveRideRoute() {
		if (this.activeRouteLayer) {
			this.activeRouteLayer.remove();
			this.activeRouteLayer = null;
		}
		this.activeLocationMarkers.forEach(m => m.remove());
		this.activeLocationMarkers = [];
	}

	private addEventLog(title: string, description: string, type: string) {
		this.eventLogs.unshift({
			title,
			description,
			type,
			time: new Date()
		});

		// Cap feed items
		if (this.eventLogs.length > 30) {
			this.eventLogs.pop();
		}
	}

	protected toggleMapTheme() {
		this.isDarkTheme = !this.isDarkTheme;
		this.map.removeLayer(this.tileLayer);

		const url = this.isDarkTheme
			? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
			: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';

		const attribution = this.isDarkTheme
			? '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
			: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

		this.tileLayer = L.tileLayer(url, {
			maxZoom: 19,
			attribution: attribution
		}).addTo(this.map);
	}

	protected async changeSimulationSpeed() {
		try {
			await this.rideService.setSimulationSpeed(this.simulationSpeed);
			this.addEventLog('Simulator Speed', `Simulation speed set to ${this.simulationSpeed}x`, 'system');
		} catch (err) {
			console.error('Failed to change simulation speed:', err);
		}
	}

	private startAnimationLoop() {
		const loop = (timestamp: number) => {
			this.interpolateDriverPositions(timestamp);
			this.animationFrameId = requestAnimationFrame(loop);
		};
		this.animationFrameId = requestAnimationFrame(loop);
	}

	private interpolateDriverPositions(timestamp: number) {
		this.driverAnimations.forEach((anim, driverId) => {
			const marker = this.driverMarkers.get(driverId);
			if (!marker) return;

			const elapsed = timestamp - anim.startTime;
			const progress = Math.min(elapsed / anim.duration, 1);

			const curLat = anim.startLat + (anim.endLat - anim.startLat) * progress;
			const curLng = anim.startLng + (anim.endLng - anim.startLng) * progress;

			marker.setLatLng([curLat, curLng]);

			if (progress >= 1) {
				this.driverAnimations.delete(driverId);
			}
		});
	}
}
