import React from 'react';
import { twMerge } from 'tailwind-merge';
import { clsx, type ClassValue } from 'clsx';
import { AlertTriangle } from 'lucide-react';
import { Button } from '../button/Button';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export interface ErrorStateProps {
  title?: string;
  message?: string;
  onRetry?: () => void;
  className?: string;
}

export const ErrorState: React.FC<ErrorStateProps> = ({
  title = 'Something went wrong',
  message = 'An unexpected error occurred. Please try again later.',
  onRetry,
  className,
}) => {
  return (
    <div className={cn('flex flex-col items-center justify-center p-12 text-center bg-rose-50/30 rounded-3xl border border-rose-100', className)}>
      <div className="w-16 h-16 rounded-full bg-rose-100 flex items-center justify-center text-rose-500 mb-6 shadow-sm shadow-rose-200/50">
        <AlertTriangle size={32} />
      </div>
      <h3 className="text-lg font-bold text-slate-900 mb-2">{title}</h3>
      <p className="text-sm text-slate-600 max-w-sm mx-auto mb-6 leading-relaxed">{message}</p>
      {onRetry && (
        <Button variant="danger" onClick={onRetry}>
          Try Again
        </Button>
      )}
    </div>
  );
};
