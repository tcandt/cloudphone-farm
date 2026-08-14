import { mockAgents } from '../data/mockData';
import { DeviceEntity, AgentEntity } from '../types';

export interface AgentItem {
  agent_id: string;
  organization_id: string;
  device_id: string;
  public_key_fingerprint?: string;
  apk_version: string;
  status: 'active' | 'inactive' | 'revoked';
  last_authenticated_at?: string;
}

export interface EnrollmentTokenIssued {
  token_id: string;
  organization_id: string;
  token_code: string;
  created_by: string;
  expires_at: string;
  bound_group_id?: string;
}

export interface EnrollmentTokenMetadata {
  token_id: string;
  organization_id: string;
  created_by: string;
  created_at: string;
  expires_at: string;
  status: 'active' | 'consumed' | 'revoked' | 'expired';
  bound_group_id?: string;
}

export interface AgentService {
  listAgents(): Promise<AgentItem[]>;
  createEnrollmentToken(ttlMinutes?: number): Promise<EnrollmentTokenIssued>;
  listEnrollmentTokens(): Promise<EnrollmentTokenMetadata[]>;
  revokeEnrollmentToken(id: string): Promise<void>;
}

export class MockAgentService implements AgentService {
  async listAgents(): Promise<AgentItem[]> {
    await new Promise((res) => setTimeout(res, 50));
    const agents: AgentEntity[] = mockAgents;
    return agents.map((a) => ({
      agent_id: a.agent_id,
      organization_id: a.organization_id,
      device_id: a.device_id,
      apk_version: a.app_version,
      status: a.status === 'active' ? 'active' : 'inactive',
      last_authenticated_at: a.connected_at,
    }));
  }

  async createEnrollmentToken(ttlMinutes = 10): Promise<EnrollmentTokenIssued> {
    await new Promise((res) => setTimeout(res, 50));
    const randomSuffix = Math.random().toString(36).substring(2, 6).toUpperCase();
    return {
      token_id: `ent_${Date.now()}`,
      organization_id: 'org_pcp_enterprise_01',
      token_code: `PCP-MOCK-${randomSuffix}`,
      created_by: 'usr_owner_01',
      expires_at: new Date(Date.now() + ttlMinutes * 60000).toISOString(),
    };
  }

  async listEnrollmentTokens(): Promise<EnrollmentTokenMetadata[]> {
    await new Promise((res) => setTimeout(res, 50));
    return [];
  }

  async revokeEnrollmentToken(_id: string): Promise<void> {
    await new Promise((res) => setTimeout(res, 50));
  }
}

export class HttpAgentService implements AgentService {
  private baseUrl = '/api/v1';

  async listAgents(): Promise<AgentItem[]> {
    // Agents list derived from devices list endpoint
    const res = await fetch(`${this.baseUrl}/devices`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });

    if (!res.ok) {
      throw new Error(`Agent list error: HTTP ${res.status}`);
    }

    const data = await res.json();
    const items: DeviceEntity[] = data.items || [];
    return items.map((d) => ({
      agent_id: `agt_${d.device_id}`,
      organization_id: d.organization_id,
      device_id: d.device_id,
      apk_version: '1.0.0',
      status: d.status === 'online' ? 'active' : 'inactive',
      last_authenticated_at: d.last_seen_at,
    }));
  }

  async createEnrollmentToken(ttlMinutes = 10): Promise<EnrollmentTokenIssued> {
    const res = await fetch(`${this.baseUrl}/enrollment-tokens`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({ ttl_minutes: ttlMinutes }),
    });

    if (!res.ok) {
      throw new Error(`Token creation error: HTTP ${res.status}`);
    }

    return await res.json();
  }

  async listEnrollmentTokens(): Promise<EnrollmentTokenMetadata[]> {
    const res = await fetch(`${this.baseUrl}/enrollment-tokens`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });

    if (!res.ok) {
      throw new Error(`List tokens error: HTTP ${res.status}`);
    }

    return await res.json();
  }

  async revokeEnrollmentToken(id: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/enrollment-tokens/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      credentials: 'include',
    });

    if (!res.ok && res.status !== 404) {
      throw new Error(`Revoke token error: HTTP ${res.status}`);
    }
  }
}

const apiMode = import.meta.env.VITE_API_MODE ?? (import.meta.env.DEV ? 'mock' : 'http');

export const agentService: AgentService =
  apiMode === 'mock'
    ? new MockAgentService()
    : new HttpAgentService();
