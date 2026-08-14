import React from 'react';
import { PermissionCode } from '../../types';
import { useAuth } from '../../context/AuthContext';
import { Lock } from 'lucide-react';

interface PermissionGuardProps {
  permission?: PermissionCode;
  requiredPermission?: PermissionCode;
  fallback?: React.ReactNode;
  children: React.ReactNode;
}

export const PermissionGuard: React.FC<PermissionGuardProps> = ({ permission, requiredPermission, fallback, children }) => {
  const { hasPermission } = useAuth();
  const targetPermission = (permission || requiredPermission)!;
  const allowed = hasPermission(targetPermission);

  if (!allowed) {
    if (fallback !== undefined) {
      return <>{fallback}</>;
    }

    return (
      <div className="relative group inline-block">
        <div className="opacity-40 pointer-events-none">{children}</div>
        <div className="absolute inset-0 flex items-center justify-center bg-slate-900/10 rounded-xl cursor-not-allowed">
          <span className="p-1 bg-slate-800 text-amber-400 rounded-md shadow-md" title={`Required Permission: ${permission}`}>
            <Lock size={14} />
          </span>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};
