import { OperatorEvent } from './operator-event-client';

export interface OperationalCommand {
  commandId: string;
  deviceId: string;
  acceptedAt: string;

  deliveryStatus?: string;
  attemptCount?: number;
  dispatchedAt?: string;

  executionStatus?: string;
  lastSequence: number;
  occurredAt?: string;

  operationalStatus: 'normal' | 'confirmation_timeout' | 'execution_result_unknown';
  errorMessage?: string;
  isTerminal: boolean;
}

const TERMINAL_STATUSES = new Set(['succeeded', 'failed', 'expired', 'completed']);

export class OperationalCommandStore {
  private commands = new Map<string, OperationalCommand>();
  private timers = new Map<string, ReturnType<typeof setTimeout>>();
  private listeners = new Set<(commands: OperationalCommand[]) => void>();

  constructor(private targetDeviceId: string, private timeoutMs = 15000) {}

  public subscribe(listener: (commands: OperationalCommand[]) => void): () => void {
    this.listeners.add(listener);
    listener(this.getCommands());
    return () => {
      this.listeners.delete(listener);
    };
  }

  public getCommands(): OperationalCommand[] {
    return Array.from(this.commands.values()).sort(
      (a, b) => new Date(b.acceptedAt).getTime() - new Date(a.acceptedAt).getTime()
    );
  }

  public trackAcceptedCommand(commandId: string, deviceId: string, acceptedAt = new Date().toISOString()): void {
    if (deviceId !== this.targetDeviceId) return;

    let cmd = this.commands.get(commandId);
    if (!cmd) {
      cmd = {
        commandId,
        deviceId,
        acceptedAt,
        lastSequence: 0,
        operationalStatus: 'normal',
        isTerminal: false,
      };
      this.commands.set(commandId, cmd);
      this.notify();
    }
  }

  public processEvent(event: OperatorEvent): void {
    if (event.type === 'command.delivery.changed') {
      const data = event.data;
      if (data.device_id !== this.targetDeviceId) return;

      let cmd = this.commands.get(data.command_id);
      if (!cmd) {
        cmd = {
          commandId: data.command_id,
          deviceId: data.device_id,
          acceptedAt: data.dispatched_at,
          lastSequence: 0,
          operationalStatus: 'normal',
          isTerminal: false,
        };
        this.commands.set(data.command_id, cmd);
      }

      cmd.deliveryStatus = data.delivery_status;
      cmd.attemptCount = data.attempt_count;
      cmd.dispatchedAt = data.dispatched_at;
      this.notify();
    } else if (event.type === 'command.status.changed') {
      const data = event.data;
      if (data.device_id !== this.targetDeviceId) return;

      let cmd = this.commands.get(data.command_id);
      if (!cmd) {
        cmd = {
          commandId: data.command_id,
          deviceId: data.device_id,
          acceptedAt: data.occurred_at,
          lastSequence: 0,
          operationalStatus: 'normal',
          isTerminal: false,
        };
        this.commands.set(data.command_id, cmd);
      }

      // Terminal state immutability check
      if (cmd.isTerminal) {
        return;
      }

      // Monotonic sequence protection check
      if (data.sequence > 0 && data.sequence <= cmd.lastSequence) {
        return; // Ignore stale or duplicate lower sequence
      }

      cmd.executionStatus = data.execution_status;
      cmd.lastSequence = data.sequence;
      cmd.occurredAt = data.occurred_at;
      if (data.error_message) {
        cmd.errorMessage = data.error_message;
      }

      const isTerm = TERMINAL_STATUSES.has(data.execution_status.toLowerCase());
      if (isTerm) {
        cmd.isTerminal = true;
        this.clearConfirmationTimer(data.command_id);
        cmd.operationalStatus = 'normal';
      } else if (data.execution_status.toLowerCase() === 'executing') {
        this.startConfirmationTimer(data.command_id);
      }

      this.notify();
    }
  }

  private startConfirmationTimer(commandId: string): void {
    this.clearConfirmationTimer(commandId);
    const timer = setTimeout(() => {
      this.timers.delete(commandId);
      const cmd = this.commands.get(commandId);
      if (cmd && !cmd.isTerminal) {
        cmd.operationalStatus = 'confirmation_timeout';
        this.notify();
      }
    }, this.timeoutMs);
    this.timers.set(commandId, timer);
  }

  private clearConfirmationTimer(commandId: string): void {
    const timer = this.timers.get(commandId);
    if (timer) {
      clearTimeout(timer);
      this.timers.delete(commandId);
    }
  }

  public destroy(): void {
    for (const timer of this.timers.values()) {
      clearTimeout(timer);
    }
    this.timers.clear();
    this.listeners.clear();
    this.commands.clear();
  }

  private notify(): void {
    const cmds = this.getCommands();
    for (const l of this.listeners) {
      try {
        l(cmds);
      } catch (err) {
        console.error('[OperationalCommandStore] Listener error:', err);
      }
    }
  }
}
