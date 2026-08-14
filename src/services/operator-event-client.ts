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
          if (!raw || typeof raw !== 'object' || !raw.type || !raw.data || typeof raw.data !== 'object') {
            console.warn('[OperatorEventClient] Rejected malformed event missing envelope structure:', evt.data);
            return;
          }

          let parsedEvent: OperatorEvent | null = null;

          if (raw.type === 'command.status.changed') {
            const d = raw.data;
            const status = d.execution_status || d.status;
            if (!d.command_id || !d.device_id || typeof status !== 'string' || typeof d.sequence !== 'number' || !d.occurred_at) {
              console.warn('[OperatorEventClient] Rejected malformed command.status.changed event payload:', raw);
              return;
            }
            parsedEvent = {
              type: 'command.status.changed',
              data: {
                command_id: String(d.command_id),
                device_id: String(d.device_id),
                execution_status: String(status),
                sequence: Number(d.sequence),
                error_message: d.error_message ? String(d.error_message) : undefined,
                occurred_at: String(d.occurred_at),
              },
            };
          } else if (raw.type === 'command.delivery.changed') {
            const d = raw.data;
            if (!d.command_id || !d.device_id || typeof d.delivery_status !== 'string' || typeof d.attempt_count !== 'number' || !d.dispatched_at) {
              console.warn('[OperatorEventClient] Rejected malformed command.delivery.changed event payload:', raw);
              return;
            }
            parsedEvent = {
              type: 'command.delivery.changed',
              data: {
                command_id: String(d.command_id),
                device_id: String(d.device_id),
                delivery_status: String(d.delivery_status),
                attempt_count: Number(d.attempt_count),
                dispatched_at: String(d.dispatched_at),
              },
            };
          } else {
            console.warn('[OperatorEventClient] Ignored unrecognized event type:', raw.type);
            return;
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
