import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HttpAgentKeyService } from '../services/agent-key-service';

global.fetch = vi.fn();

describe('HttpAgentKeyService', () => {
  let service: HttpAgentKeyService;

  beforeEach(() => {
    service = new HttpAgentKeyService();
    vi.resetAllMocks();
  });

  it('createKey sends explicit nulls for unbounded fields', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: async () => ({ key: { key_id: '123' }, raw_secret: 'secret' })
    } as unknown as Response);

    await service.createKey('Test Key', null, null);

    expect(fetch).toHaveBeenCalledWith('/api/v2/agent-keys', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'Test Key', max_bindings: null, expires_at: null })
    }));
  });

  it('createKey throws structured AgentKeyApiError on 400', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: async () => ({ code: 'INVALID_INPUT', message: 'Name too long' })
    } as unknown as Response);

    await expect(service.createKey('Test', 5, null)).rejects.toMatchObject({
      name: 'AgentKeyApiError',
      status: 400,
      code: 'INVALID_INPUT',
      message: 'Name too long'
    });
  });

  it('revokeKey throws on 404', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: async () => ({ code: 'NOT_FOUND', message: 'Key not found' })
    } as unknown as Response);

    await expect(service.revokeKey('123')).rejects.toThrow();
  });

  it('getBindings throws on 404', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: async () => ({ code: 'NOT_FOUND', message: 'Key not found' })
    } as unknown as Response);

    await expect(service.getBindings('123')).rejects.toThrow();
  });
});
