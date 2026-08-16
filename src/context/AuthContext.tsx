import React, { createContext, useContext, useState, useEffect } from 'react';
import { UserSession, UserRole, PermissionCode } from '../types';
import { authService } from '../services/auth-service';

export interface AuthContextType {
  session: UserSession | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, pass: string) => Promise<void>;
  logout: () => Promise<void>;
  // Dev-only role simulation switcher
  switchRole: (role: UserRole) => void;
  hasPermission: (permission: PermissionCode) => boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Dev feature flags
const featureFlags = {
  rbacSimulator: true,
};

const ROLE_PERMISSIONS: Record<UserRole, PermissionCode[]> = {
  owner: [
    'dashboard.read',
    'device.read',
    'device.update',
    'device.assign',
    'device.stream.view',
    'device.control.acquire',
    'device.control.input',
    'device.command.basic',
    'device.command.sensitive',
    'group.read',
    'group.manage',
    'agent.read',
    'agent.enroll',
    'agent.revoke',
    'member.read',
    'member.invite',
    'member.manage',
    'role.manage',
    'audit.read',
    'billing.read',
    'billing.manage',
    'organization.read',
    'organization.manage',
  ],
  admin: [
    'dashboard.read',
    'device.read',
    'device.update',
    'device.stream.view',
    'device.control.acquire',
    'device.control.input',
    'device.command.basic',
    'device.command.sensitive',
    'group.read',
    'group.manage',
    'agent.read',
    'agent.enroll',
    'member.read',
    'audit.read',
    'organization.read',
  ],
  manager: [
    'dashboard.read',
    'device.read',
    'device.stream.view',
    'device.control.acquire',
    'device.control.input',
    'device.command.basic',
    'group.read',
    'agent.read',
    'agent.enroll',
    'member.read',
    'audit.read',
  ],
  operator: [
    'dashboard.read',
    'device.read',
    'device.stream.view',
    'device.control.acquire',
    'device.control.input',
    'device.command.basic',
    'group.read',
  ],
  viewer: [
    'dashboard.read',
    'device.read',
    'device.stream.view',
    'group.read',
  ],
  billing: [
    'dashboard.read',
    'billing.read',
    'billing.manage',
    'audit.read',
  ],
  support_limited: [
    'dashboard.read',
    'device.read',
    'device.stream.view',
  ],
};

const isHttpMode = import.meta.env.VITE_API_MODE === 'http';

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [session, setSession] = useState<UserSession | null>(() => {
    // When connected to real backend, NEVER hydrate stale mock session from localStorage
    if (isHttpMode) return null;
    if (import.meta.env.DEV) {
      const saved = localStorage.getItem('pcp_auth_session');
      if (saved === 'null' || saved === 'none' || !saved) return null;
      try {
        return JSON.parse(saved);
      } catch {
        return null;
      }
    }
    return null;
  });
  const [isLoading, setIsLoading] = useState<boolean>(true);

  useEffect(() => {
    let mounted = true;
    async function loadSession() {
      setIsLoading(true);
      try {
        const current = await authService.fetchSession();
        if (mounted) {
          setSession(current);
          if (!current && isHttpMode) {
            localStorage.removeItem('pcp_auth_session');
          }
        }
      } catch {
        if (mounted) {
          setSession(null);
          if (isHttpMode) {
            localStorage.removeItem('pcp_auth_session');
          }
        }
      } finally {
        if (mounted) {
          setIsLoading(false);
        }
      }
    }
    loadSession();
    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    if (import.meta.env.DEV && !isHttpMode) {
      if (session) {
        localStorage.setItem('pcp_auth_session', JSON.stringify(session));
      } else {
        localStorage.setItem('pcp_auth_session', 'null');
      }
    }
  }, [session]);

  const login = async (email: string, pass: string) => {
    setIsLoading(true);
    try {
      const newSession = await authService.login(email, pass);
      setSession(newSession);
    } finally {
      setIsLoading(false);
    }
  };

  const logout = async () => {
    setIsLoading(true);
    try {
      await authService.logout();
      setSession(null);
      if (isHttpMode) {
        localStorage.removeItem('pcp_auth_session');
      }
    } finally {
      setIsLoading(false);
    }
  };

  // Dev-only role switcher guarded by feature flag & mock mode only
  const switchRole = (newRole: UserRole) => {
    if (isHttpMode || !import.meta.env.DEV || !featureFlags.rbacSimulator) return;
    if (!session) return;

    const updatedSession: UserSession = {
      ...session,
      role: newRole,
      permissions: ROLE_PERMISSIONS[newRole] || [],
    };
    setSession(updatedSession);
  };

  const hasPermission = (permission: PermissionCode): boolean => {
    if (!session) return false;
    return session.permissions.includes(permission);
  };

  return (
    <AuthContext.Provider
      value={{
        session,
        isAuthenticated: !!session,
        isLoading,
        login,
        logout,
        switchRole,
        hasPermission,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = (): AuthContextType => {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return ctx;
};
