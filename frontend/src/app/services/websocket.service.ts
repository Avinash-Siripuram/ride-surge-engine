import { Injectable, signal } from '@angular/core';
import { Subject } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class WebSocketService {
  private ws!: WebSocket;
  
  // Observables for components to subscribe to
  public driverLocation$ = new Subject<any>();
  public surgeUpdate$ = new Subject<any>();
  public rideRequested$ = new Subject<any>();
  public rideMatched$ = new Subject<any>();
  public rideFailed$ = new Subject<any>();
  
  public isConnected = signal(false);

  constructor() {
    this.connect();
  }

  private connect() {
    const hostname = window.location.hostname;
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    
    // Connect to live Back4app backend in production
    const wsUrl = hostname === 'localhost' || hostname === '127.0.0.1'
      ? 'ws://localhost:8080/ws'
      : `${protocol}//ridesurgebackend-nyo1.b4a.run/ws`;

    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log('Connected to WebSocket server');
      this.isConnected.set(true);
    };

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessage(data);
      } catch (err) {
        console.error('Error parsing WebSocket message:', err);
      }
    };

    this.ws.onclose = () => {
      console.log('Disconnected from WebSocket server. Reconnecting in 3s...');
      this.isConnected.set(false);
      setTimeout(() => this.connect(), 3000);
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      this.ws.close();
    };
  }

  private handleMessage(message: { type: string; payload: any }) {
    switch (message.type) {
      case 'DRIVER_LOCATION':
        this.driverLocation$.next(message.payload);
        break;
      case 'SURGE_UPDATE':
        this.surgeUpdate$.next(message.payload);
        break;
      case 'RIDE_REQUESTED':
        this.rideRequested$.next(message.payload);
        break;
      case 'RIDE_MATCHED':
        this.rideMatched$.next(message.payload);
        break;
      case 'RIDE_FAILED':
        this.rideFailed$.next(message.payload);
        break;
      default:
        console.warn('Unknown message type:', message.type);
    }
  }
}
