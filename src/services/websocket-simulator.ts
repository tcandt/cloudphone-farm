export type WsStatus = 'connected' | 'reconnecting' | 'disconnected';

export type WsEventType =
  | 'device.presence.changed'
  | 'device.telemetry.updated'
  | 'stream.state.changed'
  | 'control.lease.changed'
  | 'command.updated'
  | 'command.ack'
  | 'command.executing'
  | 'command.succeeded'
  | 'command.failed';

export interface WsEventEnvelope {
  event_id: string;
  event_type: WsEventType;
  device_id?: string;
  organization_id?: string;
  timestamp: string;
  payload?: Record<string, unknown>;
  data?: Record<string, unknown>;
}

type WsListener = (event: WsEventEnvelope) => void;

class WebSocketSimulator {
  private listeners: Map<WsEventType, Set<WsListener>> = new Map();
  private statusListeners: Set<(status: WsStatus) => void> = new Set();
  private currentStatus: WsStatus = 'connected';
  private telemetryIntervalId: number | null = null;

  constructor() {
    this.startPeriodicTelemetry();
  }

  getStatus(): WsStatus {
    return this.currentStatus;
  }

  onStatusChange(callback: (status: WsStatus) => void): () => void {
    this.statusListeners.add(callback);
    callback(this.currentStatus);
    return () => this.statusListeners.delete(callback);
  }

  subscribe(eventType: WsEventType, listener: WsListener): () => void {
    if (!this.listeners.has(eventType)) {
      this.listeners.set(eventType, new Set());
    }
    this.listeners.get(eventType)!.add(listener);

    return () => {
      this.listeners.get(eventType)?.delete(listener);
    };
  }

  publish(event: WsEventEnvelope): void {
    const topicListeners = this.listeners.get(event.event_type);
    if (topicListeners) {
      topicListeners.forEach((fn) => fn(event));
    }
  }

  simulateReconnect(): void {
    this.setStatus('reconnecting');
    setTimeout(() => {
      this.setStatus('connected');
    }, 1500);
  }

  private setStatus(status: WsStatus): void {
    this.currentStatus = status;
    this.statusListeners.forEach((fn) => fn(status));
  }

  private startPeriodicTelemetry(): void {
    if (this.telemetryIntervalId) return;

    this.telemetryIntervalId = window.setInterval(() => {
      if (this.currentStatus !== 'connected') return;

      // Emit simulated periodic telemetry event
      const event: WsEventEnvelope = {
        event_id: `evt_${Math.random().toString(36).substring(2, 9)}`,
        event_type: 'device.telemetry.updated',
        organization_id: 'org_pcp_enterprise_01',
        timestamp: new Date().toISOString(),
        data: {
          device_id: 'dev_s7_001',
          battery: Math.min(100, Math.max(10, 85 + Math.floor(Math.random() * 5 - 2))),
          cpu_usage: Math.floor(12 + Math.random() * 20),
          ram_usage: Math.floor(40 + Math.random() * 10),
          latency_ms: Math.floor(14 + Math.random() * 8),
        },
      };

      this.publish(event);
    }, 4000);
  }
}

export const defaultWsSimulator = new WebSocketSimulator();
