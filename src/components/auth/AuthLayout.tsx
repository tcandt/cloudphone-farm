import React from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Headphones } from 'lucide-react';
import { LanguageSelector } from './LanguageSelector';
import { IsometricIllustration } from './IsometricIllustration';

interface AuthLayoutProps {
  children: React.ReactNode;
  mode: 'login' | 'register';
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children, mode }) => {
  const { t } = useTranslation();

  return (
    <div className="min-h-screen w-full bg-white flex flex-col lg:flex-row text-slate-900 font-sans selection:bg-amber-100 selection:text-amber-900">
      {/* Left Panel: Form & Brand Header (~42% - 45% width on desktop) */}
      <div className="w-full lg:w-[45%] flex flex-col justify-between p-6 sm:p-10 lg:p-12 min-h-screen z-10 bg-white">
        {/* Top Header: Logo + Language Switcher */}
        <div className="flex items-center justify-between w-full mb-8">
          <Link to="/" className="flex items-center gap-2 group">
            {/* Logo Icon matching MaxCloudPhone design */}
            <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-orange-500 via-amber-500 to-red-500 p-0.5 shadow-md group-hover:scale-105 transition-transform flex items-center justify-center">
              <div className="w-full h-full bg-gradient-to-br from-orange-500 to-amber-600 rounded-[10px] flex items-center justify-center text-white font-black text-xs tracking-tighter">
                MAX
              </div>
            </div>
            {/* Brand Text */}
            <div className="flex items-center text-lg font-extrabold tracking-tight">
              <span className="text-orange-600">Max</span>
              <span className="text-slate-900">CloudPhone</span>
            </div>
          </Link>

          {/* Language Switcher Dropdown */}
          <LanguageSelector />
        </div>

        {/* Form Container */}
        <div className="w-full max-w-md mx-auto my-auto py-4">
          {children}
        </div>

        {/* Left Panel Footer / Legal copyright */}
        <div className="text-center lg:text-left text-xs font-medium text-slate-400 mt-8">
          © {new Date().getFullYear()} MaxCloudPhone System. All rights reserved.
        </div>
      </div>

      {/* Right Panel: Hero Banner & Isometric Illustration (~55% width on desktop) */}
      <div className="hidden lg:block lg:w-[55%] min-h-screen relative">
        <IsometricIllustration mode={mode} />
      </div>

      {/* Floating Bottom-Right Support Badge (Hỗ trợ 24/7) */}
      <div className="fixed bottom-4 right-4 z-50">
        <button
          type="button"
          onClick={() => alert('Đội ngũ hỗ trợ MaxCloudPhone 24/7 luôn sẵn sàng! Hotline: 1900 xxxx')}
          className="flex items-center gap-2 bg-blue-600 hover:bg-blue-700 text-white px-3.5 py-2 rounded-xl shadow-lg shadow-blue-500/25 transition-all duration-200 hover:scale-105 active:scale-95 group text-xs font-bold"
        >
          <div className="relative">
            <Headphones size={16} />
            <span className="absolute -top-1 -right-1 w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
          </div>
          <span>{t('auth.supportBtn')}</span>
          <span className="ml-1 px-1.5 py-0.2 bg-white/20 rounded-full text-[10px] font-extrabold">0</span>
        </button>
      </div>
    </div>
  );
};
