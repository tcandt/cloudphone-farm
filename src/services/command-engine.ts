import { DeviceCommand, CommandStatus } from '../types';
import { mockCurrentUserSession } from '../data/mockData';
import { defaultWsSimulator } from './websocket-simulator';

export class CommandEngine {
  private commands: DeviceCommand[] = [];

  getCommands(deviceId?: string): DeviceCommand[] {
    if (deviceId) {
      return this.commands.filter((c) => c.device_id === deviceId);
    }
    return this.commands;
  }

  async dispatchCommand(
    deviceId: string,
    commandType: DeviceCommand['command_type'],
    payload: Record<string, any> = {}
  ): Promise<DeviceCommand> {
    const commandId = `cmd_${Math.random().toString(36).substring(2, 9)}`;

    const newCommand: DeviceCommand = {
      command_id: commandId,
      device_id: deviceId,
      organization_id: mockCurrentUserSession.organization_id,
      actor_id: mockCurrentUserSession.user_id,
      actor_name: mockCurrentUserSession.display_name,
      command_type: commandType,
      payload,
      status: 'pending',
      created_at: new Date().toISOString(),
    };

    this.commands.unshift(newCommand);
    this.emitCommandUpdate(newCommand);

    // Simulate async execution phases (ack -> executing -> succeeded)
    setTimeout(() => {
      newCommand.status = 'ack';
      this.emitCommandUpdate(newCommand);
    }, 120);

    setTimeout(() => {
      newCommand.status = 'executing';
      this.emitCommandUpdate(newCommand);
    }, 300);

    setTimeout(() => {
      newCommand.status = 'succeeded';
      newCommand.executed_at = new Date().toISOString();
      this.emitCommandUpdate(newCommand);
    }, 600);

    return newCommand;
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
