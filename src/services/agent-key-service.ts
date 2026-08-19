import { AgentKey, AgentKeyBinding, AgentKeyCreatedResponse } from '../types';

export interface IAgentKeyService {
  createKey(name: string, maxBindings?: number, expiresAt?: string): Promise<AgentKeyCreatedResponse>;
  listKeys(): Promise<AgentKey[]>;
  getKey(keyId: string): Promise<AgentKey | null>;
  updateKey(keyId: string, payload: { name?: string; max_bindings?: number | null; expires_at?: string | null }): Promise<AgentKey>;
  revokeKey(keyId: string): Promise<void>;
  getBindings(keyId: string): Promise<AgentKeyBinding[]>;
}

export class HttpAgentKeyService implements IAgentKeyService {
  async createKey(name: string, maxBindings?: number, expiresAt?: string): Promise<AgentKeyCreatedResponse> {
    const payload: Record<string, any> = { name };
    if (maxBindings !== undefined) payload.max_bindings = maxBindings;
    if (expiresAt !== undefined) payload.expires_at = expiresAt;

    const res = await fetch('/api/v2/agent-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error('Failed to create agent key');
    return res.json();
  }

  async listKeys(): Promise<AgentKey[]> {
    const res = await fetch('/api/v2/agent-keys', {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });
    if (!res.ok) throw new Error('Failed to list agent keys');
    return res.json();
  }

  async getKey(keyId: string): Promise<AgentKey | null> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });
    if (res.status === 404) return null;
    if (!res.ok) throw new Error('Failed to fetch agent key');
    return res.json();
  }

  async updateKey(keyId: string, payload: { name?: string; max_bindings?: number | null; expires_at?: string | null }): Promise<AgentKey> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error('Failed to update agent key');
    return res.json();
  }

  async revokeKey(keyId: string): Promise<void> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}`, {
      method: 'DELETE',
      credentials: 'include',
    });
    if (!res.ok && res.status !== 404 && res.status !== 204) throw new Error('Failed to revoke agent key');
  }

  async getBindings(keyId: string): Promise<AgentKeyBinding[]> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}/devices`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });
    if (res.status === 404) return [];
    if (!res.ok) throw new Error('Failed to fetch bindings for agent key');
    return res.json();
  }
}

export const agentKeyService: IAgentKeyService = new HttpAgentKeyService();
