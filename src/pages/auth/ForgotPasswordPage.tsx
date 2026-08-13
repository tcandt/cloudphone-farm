import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { Mail, ArrowLeft } from 'lucide-react';
import { AuthLayout } from '../../components/auth/AuthLayout';

export const ForgotPasswordPage: React.FC = () => {
  const [sent, setSent] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSent(true);
  };

  return (
    <AuthLayout mode="login">
      <div className="space-y-6">
        <div className="text-center space-y-1">
          <h1 className="text-2xl sm:text-3xl font-extrabold text-amber-600 tracking-tight">
            Khôi phục mật khẩu
          </h1>
          <p className="text-xs sm:text-sm font-medium text-slate-400">
            Nhập email của bạn để nhận liên kết đặt lại mật khẩu an toàn.
          </p>
        </div>

        {sent ? (
          <div className="bg-emerald-50 border border-emerald-200 rounded-2xl p-4 text-center space-y-3">
            <p className="text-xs font-semibold text-emerald-800">
              Liên kết khôi phục đã được gửi! Vui lòng kiểm tra email của bạn.
            </p>
            <Link to="/login" className="inline-block text-xs font-bold text-amber-600 hover:underline">
              Trở về Đăng nhập
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-bold text-slate-700 mb-1">
                <span className="text-red-500 mr-0.5">*</span> Email tài khoản
              </label>
              <div className="relative">
                <Mail size={16} className="absolute left-3.5 top-3.5 text-slate-400" />
                <input
                  type="email"
                  required
                  className="w-full pl-10 pr-4 py-2.5 bg-slate-50/50 border border-slate-200 rounded-xl text-sm font-medium focus:ring-2 focus:ring-amber-100 focus:border-amber-500 focus:bg-white outline-none"
                  placeholder="admin@organization.com"
                />
              </div>
            </div>
            <button
              type="submit"
              className="w-full py-3 bg-[#df7f00] hover:bg-[#c97200] text-white font-extrabold text-sm rounded-xl shadow-md transition-all"
            >
              Gửi liên kết khôi phục
            </button>
          </form>
        )}

        <div className="text-center pt-2">
          <Link to="/login" className="text-xs font-semibold text-slate-600 hover:text-amber-600 flex items-center justify-center gap-1">
            <ArrowLeft size={14} /> Quay lại đăng nhập
          </Link>
        </div>
      </div>
    </AuthLayout>
  );
};
