import React, { createContext, useContext, useState, useEffect } from 'react';
import { UserSession, UserRole, PermissionCode } from '../types';
import { mockCurrentUserSession } from '../data/mockData';

export interface AuthContextType {
  session: UserSession | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, pass: string) => Promise<void>;
  logout: () => void;
  // Dev-only role simulation switcher
  switchRole: (role: UserRole) => void;
  hasPermission: (permission: PermissionCode) => boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

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

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [session, setSession] = useState<UserSession | null>(() => {
    const saved = localStorage.getItem('pcp_auth_session');
    if (saved === 'null' || saved === 'none') return null;
    return saved ? JSON.parse(saved) : mockCurrentUserSession;
  });
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (session) {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    } else {
      localStorage.removeItem('pcp_auth_session');
    }
  }, [session]);

  const login = async (_email: string, _pass: string) => {
    setIsLoading(true);
    await new Promise((res) => setTimeout(res, 300));
    setSession(mockCurrentUserSession);
    setIsLoading(false);
  };

  const logout = () => {
    setSession(null);
  };

  // Dev-only role switcher
  const switchRole = (newRole: UserRole) => {
    if (!import.meta.env.DEV) return;
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
