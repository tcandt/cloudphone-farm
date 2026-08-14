import { UserSession } from '../types';
import { mockCurrentUserSession } from '../data/mockData';

export interface AuthService {
  fetchSession(): Promise<UserSession | null>;
  login(email: string, pass: string): Promise<UserSession>;
  logout(): Promise<void>;
}

export class MockAuthService implements AuthService {
  async fetchSession(): Promise<UserSession | null> {
    const saved = localStorage.getItem('pcp_auth_session');
    if (saved === 'null' || saved === 'none' || !saved) return null;
    try {
      return JSON.parse(saved);
    } catch {
      return null;
    }
  }

  async login(_email: string, _pass: string): Promise<UserSession> {
    await new Promise((res) => setTimeout(res, 200));
    return mockCurrentUserSession;
  }

  async logout(): Promise<void> {
    localStorage.setItem('pcp_auth_session', 'null');
  }
}

export class HttpAuthService implements AuthService {
  private baseUrl = '/api/v1/auth';

  async fetchSession(): Promise<UserSession | null> {
    try {
      const res = await fetch(`${this.baseUrl}/session`, {
        headers: { Accept: 'application/json' },
        credentials: 'include',
      });
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async login(email: string, pass: string): Promise<UserSession> {
    const res = await fetch(`${this.baseUrl}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ email, password: pass }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ message: 'Login failed' }));
      throw new Error(err.message || 'Authentication failed');
    }
    return await res.json();
  }

  async logout(): Promise<void> {
    await fetch(`${this.baseUrl}/logout`, {
      method: 'POST',
      credentials: 'include',
    }).catch(() => {});
  }
}

const apiMode = import.meta.env.VITE_API_MODE ?? (import.meta.env.DEV ? 'mock' : 'http');

export const authService: AuthService =
  apiMode === 'mock'
    ? new MockAuthService()
    : new HttpAuthService();
