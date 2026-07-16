import { Injectable } from '@angular/core';

export interface RideRequestPayload {
  passenger_id: string;
  pickup_lat: number;
  pickup_lng: number;
  destination_lat: number;
  destination_lng: number;
}

@Injectable({
  providedIn: 'root'
})
export class RideService {
  private baseUrl = '';

  constructor() {
    const hostname = window.location.hostname;
    const protocol = window.location.protocol;
    
    // Connect to live Back4app backend in production
    this.baseUrl = hostname === 'localhost' || hostname === '127.0.0.1'
      ? 'http://localhost:8080/api'
      : `${protocol}//ride-surge-engine.onrender.com/api`;
  }

  async requestRide(payload: RideRequestPayload): Promise<any> {
    const res = await fetch(`${this.baseUrl}/rides/request`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      const err = await res.json();
      throw new Error(err.message || 'Failed to match ride');
    }

    return res.json();
  }

  async getZones(): Promise<any> {
    const res = await fetch(`${this.baseUrl}/zones`);
    return res.json();
  }

  async searchLocation(query: string): Promise<any[]> {
    if (!query || query.trim().length < 3) return [];
    
    const url = `https://nominatim.openstreetmap.org/search?format=json&q=${encodeURIComponent(query)}&limit=5&countrycodes=in&viewbox=78.2,17.2,78.7,17.6&bounded=1`;
    
    try {
      const res = await fetch(url, {
        headers: {
          'Accept-Language': 'en',
          'User-Agent': 'ride-surge-engine-app'
        }
      });
      if (!res.ok) return [];
      return res.json();
    } catch (err) {
      console.error('Geocoding search failed:', err);
      return [];
    }
  }
}
