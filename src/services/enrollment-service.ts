import { EnrollmentToken } from '../types';
import { mockCurrentUserSession } from '../data/mockData';

export interface IEnrollmentService {
  createToken(groupId?: string): Promise<EnrollmentToken>;
  getToken(tokenId: string): Promise<EnrollmentToken | null>;
  revokeToken(tokenId: string): Promise<void>;
  listTokens(): Promise<EnrollmentToken[]>;
}

export class MockEnrollmentService implements IEnrollmentService {
  private tokens: EnrollmentToken[] = [];

  async createToken(groupId?: string): Promise<EnrollmentToken> {
    const array = new Uint8Array(16);
    crypto.getRandomValues(array);
    const tokenCode = Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');

    const newToken: EnrollmentToken = {
      token_id: `token_${Math.random().toString(36).substring(2, 9)}`,
      organization_id: mockCurrentUserSession.organization_id,
      token_code: `ENROLL-${tokenCode.substring(0, 12).toUpperCase()}`,
      created_by: mockCurrentUserSession.user_id,
      expires_at: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
      used: false,
      bound_group_id: groupId,
    };

    this.tokens.unshift(newToken);
    return newToken;
  }

  async getToken(tokenId: string): Promise<EnrollmentToken | null> {
    return this.tokens.find((t) => t.token_id === tokenId) || null;
  }

  async revokeToken(tokenId: string): Promise<void> {
    this.tokens = this.tokens.filter((t) => t.token_id !== tokenId);
  }

  async listTokens(): Promise<EnrollmentToken[]> {
    return this.tokens;
  }
}

export class HttpEnrollmentService implements IEnrollmentService {
  async createToken(groupId?: string): Promise<EnrollmentToken> {
    const res = await fetch('/api/v1/enrollment-tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bound_group_id: groupId }),
    });
    if (!res.ok) throw new Error('Failed to create enrollment token');
    return res.json();
  }

  async getToken(tokenId: string): Promise<EnrollmentToken | null> {
    const res = await fetch(`/api/v1/enrollment-tokens/${tokenId}`);
    if (res.status === 404) return null;
    if (!res.ok) throw new Error('Failed to fetch enrollment token');
    return res.json();
  }

  async revokeToken(tokenId: string): Promise<void> {
    const res = await fetch(`/api/v1/enrollment-tokens/${tokenId}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Failed to revoke enrollment token');
  }

  async listTokens(): Promise<EnrollmentToken[]> {
    const res = await fetch('/api/v1/enrollment-tokens');
    if (!res.ok) throw new Error('Failed to list enrollment tokens');
    return res.json();
  }
}

export const enrollmentService: IEnrollmentService = import.meta.env.DEV
  ? new MockEnrollmentService()
  : new HttpEnrollmentService();
