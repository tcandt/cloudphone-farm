import React, { Component, ErrorInfo, ReactNode } from 'react';
import { ShieldAlert, RefreshCcw } from 'lucide-react';

interface Props {
  children?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Uncaught UI error:', error, errorInfo);
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-[400px] bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-8 flex flex-col items-center justify-center text-center space-y-4 my-8 max-w-lg mx-auto">
          <div className="w-16 h-16 rounded-3xl bg-rose-50 text-rose-600 flex items-center justify-center">
            <ShieldAlert size={32} />
          </div>
          <h2 className="text-xl font-extrabold text-slate-900">Đã xảy ra lỗi hiển thị UI</h2>
          <p className="text-xs text-slate-500 max-w-md">
            {this.state.error?.message || 'Một sự cố không mong muốn đã xảy ra trong thành phần này.'}
          </p>
          <button
            onClick={() => window.location.reload()}
            className="px-5 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-md transition-all flex items-center gap-2"
          >
            <RefreshCcw size={15} /> Tải lại trang
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
