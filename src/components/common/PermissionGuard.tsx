import React from 'react';
import { PermissionCode } from '../../types';
import { mockCurrentUserSession } from '../../data/mockData';
import { Lock } from 'lucide-react';

interface PermissionGuardProps {
  permission: PermissionCode;
  fallback?: React.ReactNode;
  children: React.ReactNode;
}

export const PermissionGuard: React.FC<PermissionGuardProps> = ({ permission, fallback, children }) => {
  const hasPermission = mockCurrentUserSession.permissions.includes(permission);

  if (!hasPermission) {
    if (fallback !== undefined) {
      return <>{fallback}</>;
    }

    return (
      <div className="relative group inline-block">
        <div className="opacity-40 pointer-events-none">{children}</div>
        <div className="absolute inset-0 flex items-center justify-center bg-slate-900/10 rounded-xl cursor-not-allowed">
          <span className="p-1 bg-slate-800 text-amber-400 rounded-md shadow-md" title={`Cần quyền: ${permission}`}>
            <Lock size={14} />
          </span>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};
