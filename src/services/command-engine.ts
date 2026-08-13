import { DispatchCommandRequest, DeviceCommand, DeviceEntity, UserSession, ControlLease } from '../types';
import { mockDevices } from '../data/mockData';
import { defaultWsSimulator } from './websocket-simulator';

export class CommandExecutionError extends Error {
  constructor(public code: string, message: string) {
    super(`${code}: ${message}`);
    this.name = 'CommandExecutionError';
  }
}

interface IdempotencyRecord {
  command: DeviceCommand;
  expiresAt: number;
}

export class CommandEngine {
  private activeLeases: Map<string, ControlLease> = new Map();
  private idempotencyStore: Map<string, IdempotencyRecord> = new Map();
  private commandHistory: Map<string, DeviceCommand[]> = new Map();

  constructor() {
    // Seed initial active control lease for dev
    this.activeLeases.set('lease_dev_001', {
      control_lease_id: 'lease_dev_001',
      device_id: 'dev_s7_001',
      organization_id: 'org_pcp_enterprise_01',
      user_id: 'usr_owner_01',
      user_display_name: 'Minh Tuấn (Owner)',
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 3600 * 1000).toISOString(),
      ttl_seconds: 3600,
    });
  }

  public registerLease(lease: ControlLease): void {
    // Check single active lease occupancy per device
    for (const [existingId, existingLease] of this.activeLeases.entries()) {
      if (existingLease.device_id === lease.device_id && new Date(existingLease.expires_at).getTime() > Date.now()) {
        if (existingLease.user_id !== lease.user_id) {
          throw new CommandExecutionError(
            'LEASE_CONFLICT',
            `Device ${lease.device_id} already has an active control lease held by user ${existingLease.user_display_name}.`
          );
        } else {
          // Same user replacing/renewing lease on the device
          this.activeLeases.delete(existingId);
        }
      } else if (new Date(existingLease.expires_at).getTime() <= Date.now()) {
        this.activeLeases.delete(existingId);
      }
    }
    this.activeLeases.set(lease.control_lease_id, lease);
  }

  public revokeLease(leaseId: string): void {
    this.activeLeases.delete(leaseId);
  }

  public getLease(leaseId: string): ControlLease | undefined {
    return this.activeLeases.get(leaseId);
  }

  public dispatch(req: DispatchCommandRequest, session: UserSession): DeviceCommand {
    this.cleanExpiredIdempotency();

    // 1. Validate timestamp & clock skew bounds
    const now = Date.now();
    const issuedAtTime = new Date(req.issuedAt).getTime();
    const expiresAtTime = new Date(req.expiresAt).getTime();

    if (isNaN(issuedAtTime) || issuedAtTime > now + 300 * 1000) {
      throw new CommandExecutionError('INVALID_TIMESTAMP', 'Command issuedAt timestamp is in the far future.');
    }
    if (isNaN(expiresAtTime) || issuedAtTime > expiresAtTime) {
      throw new CommandExecutionError('INVALID_TIMEFRAME', 'Command issuedAt cannot be later than expiresAt.');
    }
    if (expiresAtTime <= now) {
      throw new CommandExecutionError('COMMAND_EXPIRED', 'Command request envelope has expired.');
    }

    // 2. Find device & validate tenant isolation
    const device = mockDevices.find((d) => d.device_id === req.deviceId);
    if (!device) {
      throw new CommandExecutionError('DEVICE_NOT_FOUND', `Device ID ${req.deviceId} not found.`);
    }

    if (device.organization_id !== session.organization_id) {
      throw new CommandExecutionError('TENANT_MISMATCH', 'Target device belongs to a different organization.');
    }

    // 3. Check authorization, leases, device online status, and payload details
    this.validateCommandAuthorization(req, device, session);

    // 4. Check idempotency after authorization (scoped to organization)
    const idempotencyScopedKey = `${session.organization_id}:${req.idempotencyKey}`;
    const existing = this.idempotencyStore.get(idempotencyScopedKey);
    if (existing) {
      return existing.command;
    }

    // 5. Create command record
    const command: DeviceCommand = {
      command_id: `cmd_${Math.random().toString(36).substring(2, 10)}`,
      device_id: req.deviceId,
      organization_id: session.organization_id,
      actor_id: session.user_id,
      actor_name: session.display_name,
      command_type: req.type,
      payload: req.payload,
      status: 'pending',
      created_at: new Date().toISOString(),
    };

    // Store in history
    const history = this.commandHistory.get(req.deviceId) || [];
    history.unshift(command);
    this.commandHistory.set(req.deviceId, history);

    // Cache idempotency with 10 min TTL
    this.idempotencyStore.set(idempotencyScopedKey, {
      command,
      expiresAt: now + 600 * 1000,
    });

    // Simulate async execution flow
    this.emitCommandUpdate(command);
    return command;
  }

  private validateCommandAuthorization(req: DispatchCommandRequest, device: DeviceEntity, session: UserSession): void {
    const isInteractive = [
      'gesture.touch',
      'gesture.swipe',
      'input.text',
      'global.back',
      'global.home',
      'global.recents',
    ].includes(req.type);

    const isAdmin = ['device.reboot', 'device.lock', 'screen.rotate'].includes(req.type);
    const isSoftware = req.type === 'apk.install';
    const isNetwork = req.type === 'network.proxy.apply';
    const isView = req.type === 'screen.capture';

    // Device status check for interactive/admin commands
    if ((isInteractive || isAdmin || isSoftware || isNetwork) && device.status !== 'online') {
      throw new CommandExecutionError('DEVICE_OFFLINE', `Device ${device.device_id} is ${device.status} and cannot receive commands.`);
    }

    // Validate payload details
    if (req.type === 'gesture.touch') {
      const p = req.payload as { x?: number; y?: number };
      if (typeof p.x !== 'number' || typeof p.y !== 'number' || p.x < 0 || p.x > 1 || p.y < 0 || p.y > 1) {
        throw new CommandExecutionError('INVALID_PAYLOAD', 'Touch gesture coordinates (x, y) must be numbers between 0 and 1.');
      }
    } else if (req.type === 'input.text') {
      const p = req.payload as { text?: string };
      if (typeof p.text !== 'string' || !p.text.trim()) {
        throw new CommandExecutionError('INVALID_PAYLOAD', 'Input text command requires a non-empty text string.');
      }
    } else if (req.type === 'gesture.swipe') {
      const p = req.payload as { x1?: number; y1?: number; x2?: number; y2?: number };
      if (
        typeof p.x1 !== 'number' ||
        typeof p.y1 !== 'number' ||
        typeof p.x2 !== 'number' ||
        typeof p.y2 !== 'number' ||
        p.x1 < 0 || p.x1 > 1 ||
        p.y1 < 0 || p.y1 > 1 ||
        p.x2 < 0 || p.x2 > 1 ||
        p.y2 < 0 || p.y2 > 1
      ) {
        throw new CommandExecutionError('INVALID_PAYLOAD', 'Swipe gesture coordinates (x1, y1, x2, y2) must be numbers between 0 and 1.');
      }
    }

    // Interactive commands REQUIRE control lease AND device.control.input permission
    if (isInteractive) {
      if (!session.permissions.includes('device.control.input')) {
        throw new CommandExecutionError('PERMISSION_DENIED', 'Missing device.control.input permission.');
      }
      if (!req.controlLeaseId) {
        throw new CommandExecutionError('CONTROL_LEASE_REQUIRED', 'Interactive touch/input command requires active controlLeaseId.');
      }

      const lease = this.activeLeases.get(req.controlLeaseId);
      if (!lease) {
        throw new CommandExecutionError('INVALID_CONTROL_LEASE', 'Control lease does not exist or has been revoked.');
      }
      if (lease.organization_id !== session.organization_id) {
        throw new CommandExecutionError('TENANT_MISMATCH', 'Control lease belongs to a different organization.');
      }
      if (lease.device_id !== req.deviceId) {
        throw new CommandExecutionError('LEASE_DEVICE_MISMATCH', 'Control lease is bound to a different device.');
      }
      if (lease.user_id !== session.user_id) {
        throw new CommandExecutionError('LEASE_USER_MISMATCH', 'Control lease is held by another user.');
      }
      if (new Date(lease.expires_at).getTime() <= Date.now()) {
        throw new CommandExecutionError('CONTROL_LEASE_EXPIRED', 'Control lease has expired.');
      }
    }

    // View commands
    if (isView) {
      if (!session.permissions.includes('device.stream.view') && !session.permissions.includes('device.read')) {
        throw new CommandExecutionError('PERMISSION_DENIED', 'Missing device.stream.view permission.');
      }
    }

    // Admin commands
    if (isAdmin) {
      if (!session.permissions.includes('device.command.sensitive')) {
        throw new CommandExecutionError('PERMISSION_DENIED', 'Missing device.command.sensitive permission.');
      }
      if (!device.capabilities.control.sensitive_actions) {
        throw new CommandExecutionError('CAPABILITY_UNSUPPORTED', 'Sensitive administrative action disabled on standard non-ADB APK.');
      }
    }

    // Software installation
    if (isSoftware) {
      if (!session.permissions.includes('agent.enroll') && !session.permissions.includes('device.command.sensitive')) {
        throw new CommandExecutionError('PERMISSION_DENIED', 'Missing permission for APK software installation.');
      }
      if (!device.capabilities.control.sensitive_actions) {
        throw new CommandExecutionError('CAPABILITY_UNSUPPORTED', 'APK installation disabled on non-root device.');
      }
    }

    // Network proxy modification
    if (isNetwork) {
      if (!session.permissions.includes('device.update') && !session.permissions.includes('organization.manage')) {
        throw new CommandExecutionError('PERMISSION_DENIED', 'Missing permission for proxy configuration.');
      }
      if (!device.capabilities.control.sensitive_actions) {
        throw new CommandExecutionError('CAPABILITY_UNSUPPORTED', 'Proxy modification disabled on non-ADB APK.');
      }
    }
  }

  private cleanExpiredIdempotency(): void {
    const now = Date.now();
    for (const [key, record] of this.idempotencyStore.entries()) {
      if (record.expiresAt <= now) {
        this.idempotencyStore.delete(key);
      }
    }
  }

  private emitCommandUpdate(command: DeviceCommand): void {
    defaultWsSimulator.publish({
      event_id: `evt_cmd_${Math.random().toString(36).substring(2, 8)}`,
      event_type: 'command.ack',
      device_id: command.device_id,
      timestamp: new Date().toISOString(),
      payload: { command_id: command.command_id, status: 'ack' },
    });

    setTimeout(() => {
      command.status = 'executing';
      defaultWsSimulator.publish({
        event_id: `evt_cmd_${Math.random().toString(36).substring(2, 8)}`,
        event_type: 'command.executing',
        device_id: command.device_id,
        timestamp: new Date().toISOString(),
        payload: { command_id: command.command_id, status: 'executing' },
      });
    }, 200);

    setTimeout(() => {
      command.status = 'succeeded';
      command.executed_at = new Date().toISOString();
      defaultWsSimulator.publish({
        event_id: `evt_cmd_${Math.random().toString(36).substring(2, 8)}`,
        event_type: 'command.succeeded',
        device_id: command.device_id,
        timestamp: new Date().toISOString(),
        payload: { command_id: command.command_id, status: 'succeeded' },
      });
    }, 800);
  }

  public getCommands(deviceId: string): DeviceCommand[] {
    return this.commandHistory.get(deviceId) || [];
  }
}

export const defaultCommandEngine = new CommandEngine();
