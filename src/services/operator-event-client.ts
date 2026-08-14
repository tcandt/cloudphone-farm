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

const VALID_EXEC_STATUSES = new Set(['ack', 'executing', 'succeeded', 'failed', 'expired']);
const VALID_DELIVERY_STATUSES = new Set(['prepared', 'dispatched', 'failed']);

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
            const statusStr = typeof d.execution_status === 'string' ? d.execution_status.toLowerCase() : '';
            const seq = Number(d.sequence);
            const ts = String(d.occurred_at || '');

            if (
              !d.command_id ||
              typeof d.command_id !== 'string' ||
              !d.device_id ||
              typeof d.device_id !== 'string' ||
              !VALID_EXEC_STATUSES.has(statusStr) ||
              !Number.isInteger(seq) ||
              seq <= 0 ||
              !ts ||
              isNaN(Date.parse(ts))
            ) {
              console.warn('[OperatorEventClient] Rejected malformed command.status.changed event payload:', raw);
              return;
            }

            parsedEvent = {
              type: 'command.status.changed',
              data: {
                command_id: d.command_id,
                device_id: d.device_id,
                execution_status: statusStr,
                sequence: seq,
                error_message: d.error_message ? String(d.error_message) : undefined,
                occurred_at: ts,
              },
            };
          } else if (raw.type === 'command.delivery.changed') {
            const d = raw.data;
            const delivStr = typeof d.delivery_status === 'string' ? d.delivery_status.toLowerCase() : '';
            const attempts = Number(d.attempt_count);
            const ts = String(d.dispatched_at || '');

            if (
              !d.command_id ||
              typeof d.command_id !== 'string' ||
              !d.device_id ||
              typeof d.device_id !== 'string' ||
              !VALID_DELIVERY_STATUSES.has(delivStr) ||
              !Number.isInteger(attempts) ||
              attempts <= 0 ||
              !ts ||
              isNaN(Date.parse(ts))
            ) {
              console.warn('[OperatorEventClient] Rejected malformed command.delivery.changed event payload:', raw);
              return;
            }

            parsedEvent = {
              type: 'command.delivery.changed',
              data: {
                command_id: d.command_id,
                device_id: d.device_id,
                delivery_status: delivStr,
                attempt_count: attempts,
                dispatched_at: ts,
              },
            };
          } else {
            console.warn('[OperatorEventClient] Ignored unrecognized event type:', raw.type);
            return;
          }

          if (parsedEvent) {
            for (const listener of this.listeners) {
              try {
                listener(parsedEvent);
              } catch (err) {
                console.error('[OperatorEventClient] Listener error:', err);
              }
            }
          }
        } catch (err) {
          console.error('[OperatorEventClient] Error parsing event data:', err);
        }
      };

      this.ws.onerror = (err) => {
        console.warn('[OperatorEventClient] WebSocket error:', err);
      };

      this.ws.onclose = () => {
        this.ws = null;
        if (!this.isClosed) {
          this.reconnectTimer = setTimeout(() => this.connect(), 3000);
        }
      };
    } catch (err) {
      console.error('[OperatorEventClient] Connection error:', err);
    }
  }

  public close(): void {
    this.isClosed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onerror = null;
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    this.listeners.clear();
  }
}
