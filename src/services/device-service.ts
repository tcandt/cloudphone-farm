import { DeviceEntity, ControlLease } from '../types';
import { mockDevices } from '../data/mockData';

export interface DeviceListParams {
  page?: number;
  limit?: number;
  status?: string;
  group_id?: string;
  search?: string;
}

export interface DeviceListResponse {
  items: DeviceEntity[];
  page: number;
  limit: number;
  total: number;
}

export interface DeviceService {
  list(params?: DeviceListParams): Promise<DeviceListResponse>;
  getById(id: string): Promise<DeviceEntity | null>;
  acquireLease(deviceId: string): Promise<ControlLease>;
  renewLease(deviceId: string, leaseId: string): Promise<ControlLease>;
  releaseLease(deviceId: string, leaseId: string): Promise<void>;
}

export class MockDeviceService implements DeviceService {
  async list(params: DeviceListParams = {}): Promise<DeviceListResponse> {
    await new Promise((res) => setTimeout(res, 10));
    let result = [...mockDevices];

    if (params.status && params.status !== 'all') {
      result = result.filter((d) => d.status === params.status);
    }
    if (params.group_id && params.group_id !== 'all') {
      result = result.filter((d) => d.group_id === params.group_id);
    }
    if (params.search) {
      const q = params.search.toLowerCase();
      result = result.filter(
        (d) =>
          (d.display_name || d.name || '').toLowerCase().includes(q) ||
          d.serial_number.toLowerCase().includes(q) ||
          d.model.toLowerCase().includes(q)
      );
    }

    const page = params.page ?? 1;
    const limit = params.limit ?? 50;

    return {
      items: result,
      page,
      limit,
      total: result.length,
    };
  }

  async getById(id: string): Promise<DeviceEntity | null> {
    await new Promise((res) => setTimeout(res, 10));
    return mockDevices.find((d) => d.device_id === id) ?? null;
  }

  async acquireLease(deviceId: string): Promise<ControlLease> {
    await new Promise((res) => setTimeout(res, 10));
    const now = new Date();
    return {
      control_lease_id: `lease_${Math.random().toString(36).substring(2, 10)}`,
      device_id: deviceId,
      organization_id: 'org_mock',
      user_id: 'usr_operator',
      user_display_name: 'Operator User',
      fencing_token: 1,
      acquired_at: now.toISOString(),
      expires_at: new Date(now.getTime() + 30000).toISOString(),
      ttl_seconds: 30,
    };
  }

  async renewLease(deviceId: string, leaseId: string): Promise<ControlLease> {
    await new Promise((res) => setTimeout(res, 10));
    const now = new Date();
    return {
      control_lease_id: leaseId,
      device_id: deviceId,
      organization_id: 'org_mock',
      user_id: 'usr_operator',
      user_display_name: 'Operator User',
      fencing_token: 1,
      acquired_at: now.toISOString(),
      expires_at: new Date(now.getTime() + 30000).toISOString(),
      ttl_seconds: 30,
    };
  }

  async releaseLease(): Promise<void> {
    await new Promise((res) => setTimeout(res, 10));
  }
}

export class HttpDeviceService implements DeviceService {
  private baseUrl = '/api/v1/devices';

  async list(params: DeviceListParams = {}): Promise<DeviceListResponse> {
    const qs = new URLSearchParams();
    if (params.page) qs.set('page', String(params.page));
    if (params.limit) qs.set('limit', String(params.limit));
    if (params.status && params.status !== 'all') qs.set('status', params.status);
    if (params.group_id && params.group_id !== 'all') qs.set('group_id', params.group_id);
    if (params.search) qs.set('search', params.search);

    const queryStr = qs.toString() ? `?${qs.toString()}` : '';
    const res = await fetch(`${this.baseUrl}${queryStr}`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });

    if (!res.ok) {
      throw new Error(`Device API error: HTTP ${res.status}`);
    }

    return await res.json();
  }

  async getById(id: string): Promise<DeviceEntity | null> {
    const res = await fetch(`${this.baseUrl}/${encodeURIComponent(id)}`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });

    if (res.status === 404) {
      return null;
    }

    if (!res.ok) {
      throw new Error(`Device API error: HTTP ${res.status}`);
    }

    return await res.json();
  }

  async acquireLease(deviceId: string): Promise<ControlLease> {
    const res = await fetch(`${this.baseUrl}/${encodeURIComponent(deviceId)}/control-leases`, {
      method: 'POST',
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });

    if (!res.ok) {
      throw new Error(`Acquire lease failed: HTTP ${res.status}`);
    }

    return await res.json();
  }

  async renewLease(deviceId: string, leaseId: string): Promise<ControlLease> {
    const res = await fetch(`${this.baseUrl}/${encodeURIComponent(deviceId)}/control-leases/${encodeURIComponent(leaseId)}/renew`, {
      method: 'POST',
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });

    if (!res.ok) {
      throw new Error(`Renew lease failed: HTTP ${res.status}`);
    }

    return await res.json();
  }

  async releaseLease(deviceId: string, leaseId: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/${encodeURIComponent(deviceId)}/control-leases/${encodeURIComponent(leaseId)}`, {
      method: 'DELETE',
      credentials: 'include',
    });

    if (!res.ok && res.status !== 204) {
      throw new Error(`Release lease failed: HTTP ${res.status}`);
    }
  }
}

const isTestEnv = typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.MODE === 'test';
const apiMode = typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.VITE_API_MODE
  ? import.meta.env.VITE_API_MODE
  : (isTestEnv || (typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.DEV) ? 'mock' : 'http');

export const deviceService: DeviceService =
  apiMode === 'mock' ? new MockDeviceService() : new HttpDeviceService();
