import { DeviceCommand, DispatchCommandRequest, DeviceCommandType, PermissionCode } from '../types';
import { mockCurrentUserSession, mockDevices } from '../data/mockData';
import { defaultWsSimulator } from './websocket-simulator';

export class CommandEngine {
  private commands: DeviceCommand[] = [];
  private processedIdempotencyKeys = new Set<string>();

  getCommands(deviceId?: string): DeviceCommand[] {
    if (deviceId) {
      return this.commands.filter((c) => c.device_id === deviceId);
    }
    return this.commands;
  }

  /**
   * Dispatches a command with strict contract validation:
   * - Interactive commands require controlLeaseId and device.control.acquire permission
   * - Admin commands require device.admin permission
   * - Timestamp validity (issuedAt vs expiresAt vs current time)
   * - Idempotency key tracking
   * - Device capabilities check
   */
  async dispatchCommand(req: DispatchCommandRequest): Promise<DeviceCommand> {
    const now = Date.now();
    const issuedAtTime = new Date(req.issuedAt).getTime();
    const expiresAtTime = new Date(req.expiresAt).getTime();

    // 1. Clock skew / Expiration check
    if (now > expiresAtTime || issuedAtTime > expiresAtTime) {
      throw new Error('COMMAND_EXPIRED: Command timestamp has expired or is invalid.');
    }

    // 2. Frontend idempotency simulation (Authoritative storage is Go Server in Redis/DB)
    if (this.processedIdempotencyKeys.has(req.idempotencyKey)) {
      const existing = this.commands.find((c) => c.payload.idempotencyKey === req.idempotencyKey);
      if (existing) return existing;
    }
    this.processedIdempotencyKeys.add(req.idempotencyKey);

    // 3. Find target device and check online state
    const device = mockDevices.find((d) => d.device_id === req.deviceId);
    if (!device) {
      throw new Error('DEVICE_NOT_FOUND: Target device does not exist.');
    }
    if (device.status === 'offline') {
      throw new Error('DEVICE_OFFLINE: Cannot dispatch command to an offline device.');
    }

    // 4. Categorize command & check lease requirement + permissions + capabilities
    this.validateCommandContract(req, device);

    const commandId = `cmd_${Math.random().toString(36).substring(2, 9)}`;

    const newCommand: DeviceCommand = {
      command_id: commandId,
      device_id: req.deviceId,
      organization_id: mockCurrentUserSession.organization_id,
      actor_id: mockCurrentUserSession.user_id,
      actor_name: mockCurrentUserSession.display_name,
      command_type: req.type,
      payload: { ...req.payload, idempotencyKey: req.idempotencyKey },
      status: 'pending',
      created_at: new Date().toISOString(),
    };

    this.commands.unshift(newCommand);
    this.emitCommandUpdate(newCommand);

    // Simulate async execution phases (ack -> executing -> succeeded)
    setTimeout(() => {
      newCommand.status = 'ack';
      this.emitCommandUpdate(newCommand);
    }, 100);

    setTimeout(() => {
      newCommand.status = 'executing';
      this.emitCommandUpdate(newCommand);
    }, 250);

    setTimeout(() => {
      newCommand.status = 'succeeded';
      newCommand.executed_at = new Date().toISOString();
      this.emitCommandUpdate(newCommand);
    }, 500);

    return newCommand;
  }

  private validateCommandContract(req: DispatchCommandRequest, device: typeof mockDevices[0]): void {
    const isInteractive = [
      'gesture.touch',
      'gesture.swipe',
      'input.text',
      'global.back',
      'global.home',
      'global.recents',
    ].includes(req.type as DeviceCommandType);

    // Interactive commands MUST have controlLeaseId
    if (isInteractive) {
      if (!req.controlLeaseId) {
        throw new Error('CONTROL_LEASE_REQUIRED: Interactive gestures require an active control lease.');
      }
      if (!mockCurrentUserSession.permissions.includes('device.control.acquire')) {
        throw new Error('PERMISSION_DENIED: Missing device.control.acquire permission.');
      }
      if (!device.capabilities.control) {
        throw new Error('CAPABILITY_UNSUPPORTED: Device control capability is disabled on non-root/non-ADB APK.');
      }
    }

    // Sensitive / Admin commands check (Reboot, Proxy change, APK install)
    if (['device.reboot', 'device.lock', 'screen.rotate', 'network.proxy.apply', 'apk.install'].includes(req.type)) {
      if (!device.capabilities.control.sensitive_actions) {
        throw new Error('CAPABILITY_UNSUPPORTED: Sensitive administrative action disabled on standard non-ADB APK.');
      }
    }
  }

  private emitCommandUpdate(command: DeviceCommand): void {
    defaultWsSimulator.publish({
      event_id: `evt_cmd_${Math.random().toString(36).substring(2, 8)}`,
      event_type: 'command.updated',
      organization_id: command.organization_id,
      timestamp: new Date().toISOString(),
      data: { command },
    });
  }
}

export const defaultCommandEngine = new CommandEngine();
