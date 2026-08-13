import React from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Lock } from 'lucide-react';

export const ResetPasswordPage: React.FC = () => {
  const navigate = useNavigate();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-pcp-bg flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white border border-slate-100 shadow-pcp-floating rounded-3xl p-8 space-y-6">
        <div className="text-center space-y-2">
          <h1 className="text-xl font-extrabold text-slate-900">Đặt lại mật khẩu mới</h1>
          <p className="text-xs text-slate-500">Tạo mật khẩu mới an toàn cho tài khoản của bạn.</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">Mật khẩu mới</label>
            <div className="relative">
              <Lock size={16} className="absolute left-3.5 top-3 text-slate-400" />
              <input
                type="password"
                required
                minLength={8}
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 border border-slate-200 rounded-xl text-sm font-medium focus:ring-2 focus:ring-blue-500 focus:bg-white outline-none"
                placeholder="••••••••"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">Xác nhận mật khẩu mới</label>
            <div className="relative">
              <Lock size={16} className="absolute left-3.5 top-3 text-slate-400" />
              <input
                type="password"
                required
                minLength={8}
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 border border-slate-200 rounded-xl text-sm font-medium focus:ring-2 focus:ring-blue-500 focus:bg-white outline-none"
                placeholder="••••••••"
              />
            </div>
          </div>

          <button
            type="submit"
            className="w-full py-3 bg-blue-600 text-white font-bold text-sm rounded-xl shadow-lg shadow-blue-500/20 hover:bg-blue-700 transition-colors"
          >
            Cập nhật mật khẩu
          </button>
        </form>

        <div className="text-center pt-2">
          <Link to="/login" className="text-xs font-semibold text-slate-600 hover:text-blue-600">
            Quay lại Đăng nhập
          </Link>
        </div>
      </div>
    </div>
  );
};
