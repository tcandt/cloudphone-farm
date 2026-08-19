import React from 'react';
import { twMerge } from 'tailwind-merge';
import { clsx, type ClassValue } from 'clsx';
import { FolderOpen } from 'lucide-react';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  icon,
  title,
  description,
  action,
  className,
}) => {
  return (
    <div className={cn('flex flex-col items-center justify-center p-12 text-center', className)}>
      <div className="w-16 h-16 rounded-full bg-slate-50 flex items-center justify-center text-slate-400 mb-6 border border-slate-100 shadow-sm">
        {icon || <FolderOpen size={32} />}
      </div>
      <h3 className="text-lg font-bold text-slate-900 mb-2">{title}</h3>
      {description && <p className="text-sm text-slate-500 max-w-sm mx-auto mb-6 leading-relaxed">{description}</p>}
      {action && <div>{action}</div>}
    </div>
  );
};
