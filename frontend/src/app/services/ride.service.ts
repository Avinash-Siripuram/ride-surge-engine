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
  private baseUrl = 'http://localhost:8080/api';

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
}
