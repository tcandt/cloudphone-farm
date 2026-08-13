import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { User, Shield, Key, CheckCircle2, ToggleLeft, ToggleRight } from 'lucide-react';
import { mockCurrentUserSession } from '../data/mockData';
import { useUiStore } from '../stores/useUiStore';

export const SettingsPage: React.FC = () => {
  const { t } = useTranslation();
  const [totpEnabled, setTotpEnabled] = useState(true);
  const { featureRentalStore, setFeatureRentalStore } = useUiStore();

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('nav.settings')}</h1>
        <p className="text-xs text-slate-500 font-medium">Quản lý hồ sơ cá nhân, cấu hình xác thực TOTP MFA và feature flags</p>
      </div>

      {/* User Profile Card */}
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-4">
        <h2 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
          <User size={18} className="text-blue-600" /> Hồ sơ tài khoản
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
          <div>
            <label className="block font-semibold text-slate-500 mb-1">Tên hiển thị</label>
            <input
              type="text"
              readOnly
              value={mockCurrentUserSession.display_name}
              className="w-full px-3.5 py-2.5 bg-slate-50 border border-slate-200 rounded-xl font-bold text-slate-900 outline-none"
            />
          </div>
          <div>
            <label className="block font-semibold text-slate-500 mb-1">Địa chỉ Email</label>
            <input
              type="email"
              readOnly
              value={mockCurrentUserSession.email}
              className="w-full px-3.5 py-2.5 bg-slate-50 border border-slate-200 rounded-xl font-bold text-slate-900 outline-none"
            />
          </div>
        </div>
      </div>

      {/* Security & MFA */}
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-4">
        <h2 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
          <Shield size={18} className="text-purple-600" /> Bảo mật & Xác thực hai yếu tố (TOTP MFA)
        </h2>

        <div className="flex items-center justify-between p-4 bg-slate-50 border border-slate-100 rounded-2xl">
          <div>
            <p className="text-xs font-extrabold text-slate-900">Xác thực ứng dụng TOTP (Google Authenticator)</p>
            <p className="text-[11px] text-slate-500">Bắt buộc yêu cầu mã TOTP khi đăng nhập tài khoản Privileged Role</p>
          </div>

          <button
            onClick={() => setTotpEnabled(!totpEnabled)}
            className={`px-3 py-1.5 rounded-xl font-bold text-xs flex items-center gap-1.5 transition-colors ${
              totpEnabled ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-200 text-slate-600'
            }`}
          >
            {totpEnabled ? <CheckCircle2 size={16} /> : null}
            <span>{totpEnabled ? 'Đã bật' : 'Chưa bật'}</span>
          </button>
        </div>
      </div>

      {/* Feature Flags Configuration */}
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-4">
        <h2 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
          <Key size={18} className="text-amber-500" /> Feature Flags UI Prototype
        </h2>

        <div className="flex items-center justify-between p-4 bg-slate-50 border border-slate-100 rounded-2xl">
          <div>
            <p className="text-xs font-extrabold text-slate-900">VITE_FEATURE_RENTAL_STORE (Rental Store Preview)</p>
            <p className="text-[11px] text-slate-500">Bật/Tắt xem trước danh mục thuê phần cứng Cloud Node</p>
          </div>

          <button
            onClick={() => setFeatureRentalStore(!featureRentalStore)}
            className={`p-2 rounded-xl text-2xl transition-colors ${
              featureRentalStore ? 'text-blue-600' : 'text-slate-400'
            }`}
          >
            {featureRentalStore ? <ToggleRight size={32} /> : <ToggleLeft size={32} />}
          </button>
        </div>
      </div>
    </div>
  );
};
