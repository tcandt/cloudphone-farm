export interface CommandStatusEventData {
  command_id: string;
  device_id: string;
  execution_status: string;
  sequence: number;
  error_message?: string;
  occurred_at: string;
}

export interface CommandStatusChangeEvent {
  type: 'command.status.changed';
  data: CommandStatusEventData;
}

export interface CommandDeliveryEventData {
  command_id: string;
  device_id: string;
  delivery_status: string;
  attempt_count: number;
  dispatched_at: string;
}

export interface CommandDeliveryChangeEvent {
  type: 'command.delivery.changed';
  data: CommandDeliveryEventData;
}

export type OperatorEvent = CommandStatusChangeEvent | CommandDeliveryChangeEvent;

export class OperatorEventClient {
  private ws: WebSocket | null = null;
  private isClosed = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private listeners = new Set<(event: OperatorEvent) => void>();

  constructor(private deviceId: string) {}

  public subscribe(listener: (event: OperatorEvent) => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  public connect(): void {
    if (this.isClosed || this.ws) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/devices/${this.deviceId}/events/ws`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onmessage = (evt) => {
        try {
          const raw = JSON.parse(evt.data);
          if (!raw || typeof raw !== 'object' || !raw.type) return;

          let parsedEvent: OperatorEvent | null = null;

          if (raw.type === 'command.status.changed' && raw.data) {
            parsedEvent = {
              type: 'command.status.changed',
              data: {
                command_id: String(raw.data.command_id || ''),
                device_id: String(raw.data.device_id || ''),
                execution_status: String(raw.data.execution_status || raw.data.status || 'ack'),
                sequence: Number(raw.data.sequence || 0),
                error_message: raw.data.error_message ? String(raw.data.error_message) : undefined,
                occurred_at: String(raw.data.occurred_at || new Date().toISOString()),
              },
            };
          } else if (raw.type === 'command.delivery.changed' && raw.data) {
            parsedEvent = {
              type: 'command.delivery.changed',
              data: {
                command_id: String(raw.data.command_id || ''),
                device_id: String(raw.data.device_id || ''),
                delivery_status: String(raw.data.delivery_status || 'dispatched'),
                attempt_count: Number(raw.data.attempt_count || 1),
                dispatched_at: String(raw.data.dispatched_at || new Date().toISOString()),
              },
            };
          }

          if (parsedEvent) {
            for (const l of this.listeners) {
              try {
                l(parsedEvent);
              } catch (err) {
                console.error('[OperatorEventClient] Listener error:', err);
              }
            }
          }
        } catch (err) {
          console.warn('[OperatorEventClient] Failed to parse operator event:', err);
        }
      };

      this.ws.onclose = () => {
        this.ws = null;
        if (!this.isClosed) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = () => {
        if (this.ws) {
          this.ws.close();
        }
      };
    } catch (err) {
      console.warn('[OperatorEventClient] Connection initialization error:', err);
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer || this.isClosed) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, 3000);
  }

  public close(): void {
    this.isClosed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}
