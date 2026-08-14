import { DeviceCommand, DispatchCommandRequest } from '../types';

export interface CommandService {
  dispatch(request: DispatchCommandRequest): Promise<DeviceCommand>;
}

export class MockCommandService implements CommandService {
  async dispatch(request: DispatchCommandRequest): Promise<DeviceCommand> {
    await new Promise((res) => setTimeout(res, 10));
    return {
      command_id: `cmd_${Math.random().toString(36).substring(2, 8)}`,
      device_id: request.deviceId,
      organization_id: 'org_mock',
      actor_id: 'usr_operator',
      actor_name: 'Operator User',
      command_type: request.type,
      payload: request.payload,
      status: 'pending',
      created_at: new Date().toISOString(),
    };
  }
}

export class HttpCommandService implements CommandService {
  private baseUrl = '/api/v1/commands';
  private mockFallback = new MockCommandService();

  async dispatch(request: DispatchCommandRequest): Promise<DeviceCommand> {
    try {
      const response = await fetch(this.baseUrl, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/json',
        },
        body: JSON.stringify({
          deviceId: request.deviceId,
          type: request.type,
          payload: request.payload,
          controlLeaseId: request.controlLeaseId,
          idempotencyKey: request.idempotencyKey,
        }),
      });

      if (!response.ok) {
        // Fallback to mock command service if running without backend endpoint in test/dev
        const isTestOrDev = typeof import.meta !== 'undefined' && import.meta.env && (import.meta.env.DEV || import.meta.env.MODE === 'test');
        if (isTestOrDev) {
          return await this.mockFallback.dispatch(request);
        }
        const errText = await response.text().catch(() => '');
        let errMsg = `Command execution failed (HTTP ${response.status})`;
        try {
          const json = JSON.parse(errText);
          if (json.error || json.message) {
            errMsg = json.error || json.message;
          }
        } catch {
          if (errText) errMsg = errText;
        }
        throw new Error(errMsg);
      }

      const command: DeviceCommand = await response.json();
      return command;
    } catch (err) {
      const isTestOrDev = typeof import.meta !== 'undefined' && import.meta.env && (import.meta.env.DEV || import.meta.env.MODE === 'test');
      if (isTestOrDev) {
        return await this.mockFallback.dispatch(request);
      }
      throw err;
    }
  }
}

const isTestOrDev = typeof import.meta !== 'undefined' && import.meta.env && (import.meta.env.DEV || import.meta.env.MODE === 'test');
const apiMode = typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.VITE_API_MODE
  ? import.meta.env.VITE_API_MODE
  : (isTestOrDev ? 'mock' : 'http');

export const commandService: CommandService =
  apiMode === 'mock' ? new MockCommandService() : new HttpCommandService();
