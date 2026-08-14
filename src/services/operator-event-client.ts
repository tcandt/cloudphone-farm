export interface CommandStatusChangeEvent {
  type: 'command.status.changed';
  command_id: string;
  device_id: string;
  execution_status: string;
  sequence: number;
  error_message?: string;
  occurred_at: string;
}

export type OperatorEvent = CommandStatusChangeEvent;

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
          const payload = JSON.parse(evt.data) as OperatorEvent;
          for (const l of this.listeners) {
            try {
              l(payload);
            } catch (err) {
              console.error('[OperatorEventClient] Listener error:', err);
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
