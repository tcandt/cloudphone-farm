import { AgentKey, AgentKeyBinding, AgentKeyCreatedResponse } from '../types';

export class AgentKeyApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
    this.name = 'AgentKeyApiError';
  }
}

async function handleResponse(res: Response, expectedStatus?: number) {
  if (expectedStatus !== undefined && res.status !== expectedStatus) {
    // If not matching the exact expected success status (e.g., 201 or 204)
    // We fall through to error handling.
    let errorData;
    try {
      errorData = await res.json();
    } catch {
      // ignore JSON parse error
    }
    throw new AgentKeyApiError(
      res.status,
      errorData?.code || 'UNKNOWN_ERROR',
      errorData?.message || `Unexpected HTTP status ${res.status}`
    );
  }

  if (!res.ok) {
    let errorData;
    try {
      errorData = await res.json();
    } catch {
      // ignore
    }
    throw new AgentKeyApiError(
      res.status,
      errorData?.code || 'UNKNOWN_ERROR',
      errorData?.message || `HTTP error ${res.status}`
    );
  }

  if (res.status !== 204) {
    return res.json();
  }
}

export interface IAgentKeyService {
  createKey(name: string, maxBindings: number | null, expiresAt: string | null): Promise<AgentKeyCreatedResponse>;
  listKeys(): Promise<AgentKey[]>;
  getKey(keyId: string): Promise<AgentKey | null>;
  updateKey(keyId: string, payload: { name?: string; max_bindings?: number | null; expires_at?: string | null }): Promise<AgentKey>;
  revokeKey(keyId: string): Promise<void>;
  getBindings(keyId: string): Promise<AgentKeyBinding[]>;
}

export class HttpAgentKeyService implements IAgentKeyService {
  async createKey(name: string, maxBindings: number | null, expiresAt: string | null): Promise<AgentKeyCreatedResponse> {
    const payload: Record<string, unknown> = {
      name,
      max_bindings: maxBindings,
      expires_at: expiresAt,
    };

    const res = await fetch('/api/v2/agent-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
    });
    
    return handleResponse(res, 201);
  }

  async listKeys(): Promise<AgentKey[]> {
    const res = await fetch('/api/v2/agent-keys', {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });
    return handleResponse(res, 200);
  }

  async getKey(keyId: string): Promise<AgentKey | null> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });
    if (res.status === 404) return null;
    return handleResponse(res, 200);
  }

  async updateKey(keyId: string, payload: { name?: string; max_bindings?: number | null; expires_at?: string | null }): Promise<AgentKey> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
    });
    return handleResponse(res, 200);
  }

  async revokeKey(keyId: string): Promise<void> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}`, {
      method: 'DELETE',
      credentials: 'include',
    });
    await handleResponse(res, 204);
  }

  async getBindings(keyId: string): Promise<AgentKeyBinding[]> {
    const res = await fetch(`/api/v2/agent-keys/${encodeURIComponent(keyId)}/devices`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });
    return handleResponse(res, 200);
  }
}

export const agentKeyService: IAgentKeyService = new HttpAgentKeyService();
