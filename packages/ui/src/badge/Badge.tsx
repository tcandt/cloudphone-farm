import React, { HTMLAttributes } from 'react';
import { twMerge } from 'tailwind-merge';
import { clsx, type ClassValue } from 'clsx';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: 'neutral' | 'success' | 'warning' | 'danger' | 'primary';
  size?: 'sm' | 'md';
}

export const Badge: React.FC<BadgeProps> = ({ 
  className, 
  variant = 'neutral', 
  size = 'md',
  children, 
  ...props 
}) => {
  const baseStyles = 'inline-flex items-center font-bold tracking-wide uppercase';
  
  const variants = {
    neutral: 'bg-slate-100 text-slate-600',
    primary: 'bg-emerald-50 text-emerald-700',
    success: 'bg-emerald-100 text-emerald-800',
    warning: 'bg-amber-100 text-amber-800',
    danger: 'bg-rose-100 text-rose-800',
  };

  const sizes = {
    sm: 'px-1.5 py-0.5 text-[9px] rounded-md',
    md: 'px-2 py-1 text-[10px] rounded-lg',
  };

  return (
    <span
      className={cn(baseStyles, variants[variant], sizes[size], className)}
      {...props}
    >
      {children}
    </span>
  );
};
