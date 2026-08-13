import React from 'react';
import { Link } from 'react-router-dom';
import { MailCheck } from 'lucide-react';

export const VerifyEmailPage: React.FC = () => {
  return (
    <div className="min-h-screen bg-pcp-bg flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white border border-slate-100 shadow-pcp-floating rounded-3xl p-8 text-center space-y-5">
        <div className="w-16 h-16 mx-auto rounded-3xl bg-blue-50 text-blue-600 flex items-center justify-center">
          <MailCheck size={32} />
        </div>
        <h1 className="text-xl font-extrabold text-slate-900">Kiểm tra Email xác thực</h1>
        <p className="text-xs text-slate-600 leading-relaxed">
          Chúng tôi đã gửi một liên kết xác minh tới email của bạn. Vui lòng kiểm tra hộp thư (hoặc thư mục Spam) và nhấp vào liên kết để kích hoạt tài khoản Organization.
        </p>
        <div className="pt-2 space-y-2">
          <Link
            to="/login"
            className="block w-full py-2.5 bg-blue-600 text-white font-bold text-xs rounded-xl shadow-md hover:bg-blue-700 transition-colors"
          >
            Trở về Đăng nhập
          </Link>
        </div>
      </div>
    </div>
  );
};
