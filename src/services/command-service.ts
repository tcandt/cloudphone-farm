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

  async dispatch(request: DispatchCommandRequest): Promise<DeviceCommand> {
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
  }
}

export const getApiMode = (): 'mock' | 'http' => {
  if (typeof window !== 'undefined') {
    const storageMode = localStorage.getItem('pcp_api_mode');
    if (storageMode === 'mock' || storageMode === 'http') return storageMode;
    // @ts-expect-error window injected api mode override
    if (window.__PCP_VITE_API_MODE__ === 'mock') return 'mock';
  }
  if (typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.VITE_API_MODE) {
    return import.meta.env.VITE_API_MODE === 'mock' ? 'mock' : 'http';
  }
  return 'http';
};

class DynamicCommandService implements CommandService {
  private httpService = new HttpCommandService();
  private mockService = new MockCommandService();

  async dispatch(request: DispatchCommandRequest): Promise<DeviceCommand> {
    const service = getApiMode() === 'mock' ? this.mockService : this.httpService;
    return await service.dispatch(request);
  }
}

export const commandService: CommandService = new DynamicCommandService();
