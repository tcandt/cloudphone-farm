import React from 'react';
import { create } from 'zustand';
import { twMerge } from 'tailwind-merge';
import { clsx, type ClassValue } from 'clsx';
import { CheckCircle2, AlertCircle, Info, XCircle, X } from 'lucide-react';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export interface ToastMessage {
  id: string;
  type: 'success' | 'error' | 'info' | 'warning';
  title: string;
  message?: string;
}

interface ToastStore {
  toasts: ToastMessage[];
  addToast: (toast: Omit<ToastMessage, 'id'>) => void;
  removeToast: (id: string) => void;
}

export const useToastStore = create<ToastStore>((set) => ({
  toasts: [],
  addToast: (toast) => set((state) => {
    const id = Math.random().toString(36).substr(2, 9);
    // Auto remove after 5s
    setTimeout(() => {
      set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
    }, 5000);
    return { toasts: [...state.toasts, { ...toast, id }] };
  }),
  removeToast: (id) => set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) })),
}));

export const ToastProvider: React.FC = () => {
  const { toasts, removeToast } = useToastStore();

  return (
    <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-3 pointer-events-none">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={cn(
            'pointer-events-auto flex w-80 items-start gap-3 rounded-2xl p-4 shadow-lg shadow-slate-200/50 bg-white border transform transition-all duration-300 animate-fadeIn',
            toast.type === 'success' && 'border-emerald-100',
            toast.type === 'error' && 'border-rose-100',
            toast.type === 'warning' && 'border-amber-100',
            toast.type === 'info' && 'border-blue-100'
          )}
          role={toast.type === 'error' || toast.type === 'warning' ? 'alert' : 'status'}
          aria-live={toast.type === 'error' || toast.type === 'warning' ? 'assertive' : 'polite'}
        >
          <div className="shrink-0 mt-0.5" aria-hidden="true">
            {toast.type === 'success' && <CheckCircle2 size={20} className="text-emerald-500" />}
            {toast.type === 'error' && <XCircle size={20} className="text-rose-500" />}
            {toast.type === 'warning' && <AlertCircle size={20} className="text-amber-500" />}
            {toast.type === 'info' && <Info size={20} className="text-blue-500" />}
          </div>
          <div className="flex-1">
            <p className="text-sm font-bold text-slate-800">{toast.title}</p>
            {toast.message && <p className="text-xs text-slate-500 mt-1">{toast.message}</p>}
          </div>
          <button
            onClick={() => removeToast(toast.id)}
            className="shrink-0 text-slate-400 hover:text-slate-600 transition-colors p-1"
            aria-label="Close notification"
          >
            <X size={16} aria-hidden="true" />
          </button>
        </div>
      ))}
    </div>
  );
};
