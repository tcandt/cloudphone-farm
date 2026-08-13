import React from 'react';
import { useTranslation } from 'react-i18next';

interface IsometricIllustrationProps {
  mode: 'login' | 'register';
}

export const IsometricIllustration: React.FC<IsometricIllustrationProps> = ({ mode }) => {
  const { t } = useTranslation();

  const title = mode === 'login' ? t('auth.heroLoginTitle') : t('auth.heroRegisterTitle');
  const subtitle = mode === 'login' ? t('auth.heroLoginSub') : t('auth.heroRegisterSub');

  return (
    <div className="relative w-full h-full min-h-[500px] flex flex-col items-center justify-between p-8 lg:p-12 overflow-hidden bg-[#edf3f9]">
      {/* Isometric Grid Background Pattern */}
      <svg
        className="absolute inset-0 w-full h-full opacity-30 pointer-events-none"
        xmlns="http://www.w3.org/2000/svg"
        width="100%"
        height="100%"
      >
        <defs>
          <pattern id="iso-grid" width="60" height="34.641" patternUnits="userSpaceOnUse">
            <path
              d="M 30 0 L 60 17.32 L 30 34.64 L 0 17.32 Z M 30 0 L 30 34.64 M 0 17.32 L 60 17.32"
              fill="none"
              stroke="#cbd5e1"
              strokeWidth="0.8"
            />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#iso-grid)" />
      </svg>

      {/* Dynamic Text Banner at Top */}
      <div className="relative z-10 text-center max-w-xl space-y-3 mt-4">
        <h2 className="text-xl lg:text-2xl font-extrabold text-slate-800 leading-snug tracking-tight">
          {title}
        </h2>
        <p className="text-xs lg:text-sm font-medium text-slate-500 leading-relaxed">
          {subtitle}
        </p>
      </div>

      {/* Main Isometric 3D Scene */}
      <div className="relative z-10 w-full max-w-md my-auto py-6 flex items-center justify-center">
        {mode === 'login' ? (
          /* Login Illustration: Mobile phone + Shield + Laptop user + Passwords */
          <div className="relative w-72 h-80 flex items-center justify-center">
            {/* Isometric Platform Shadow */}
            <div className="absolute bottom-2 w-64 h-24 bg-gradient-to-r from-blue-300/30 to-amber-300/30 rounded-full blur-xl transform -rotate-12 scale-y-50"></div>

            {/* Smartphone Base Card */}
            <div className="relative w-44 h-72 bg-gradient-to-b from-white to-blue-50/80 rounded-3xl border-4 border-white shadow-2xl p-3 flex flex-col justify-between transform -rotate-6 transition-transform duration-500 hover:rotate-0">
              <div className="w-12 h-1.5 bg-slate-200 rounded-full mx-auto mb-2"></div>

              {/* Simulated UI inside Phone */}
              <div className="space-y-2 flex-1">
                <div className="w-10 h-10 mx-auto rounded-full bg-amber-100 border border-amber-300 flex items-center justify-center text-amber-600 font-bold text-xs">
                  📱
                </div>
                <div className="w-full h-3 bg-amber-500/20 rounded-full"></div>
                <div className="w-3/4 h-2.5 bg-slate-200 rounded-full mx-auto"></div>
                <div className="w-full h-8 bg-amber-50 rounded-xl border border-amber-200 p-1 flex items-center gap-1.5">
                  <div className="w-4 h-4 rounded-full bg-amber-500 text-white text-[10px] flex items-center justify-center font-bold">✓</div>
                  <div className="h-2 bg-amber-400 rounded w-16"></div>
                </div>
              </div>

              <div className="w-full py-2 bg-gradient-to-r from-amber-500 to-amber-600 rounded-xl text-[10px] text-white text-center font-bold">
                MaxCloud System
              </div>
            </div>

            {/* Floating Shield Lock (Front Left) */}
            <div className="absolute -left-4 bottom-12 w-20 h-24 bg-gradient-to-br from-amber-400 via-amber-500 to-amber-600 rounded-2xl shadow-xl border-2 border-white flex flex-col items-center justify-center text-white transform rotate-6 animate-bounce" style={{ animationDuration: '3s' }}>
              <svg className="w-8 h-8 text-white drop-shadow" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
              <span className="text-[9px] font-black uppercase mt-1 tracking-wider">Secure</span>
            </div>

            {/* Asterisk Password Strip (Bottom Front) */}
            <div className="absolute -bottom-2 right-4 bg-white/95 backdrop-blur-md px-4 py-2 rounded-2xl shadow-lg border border-slate-100 flex items-center gap-2 animate-pulse">
              <span className="text-amber-500 font-extrabold text-sm">****</span>
              <div className="w-2 h-2 rounded-full bg-emerald-500"></div>
            </div>

            {/* User with Laptop Icon Card (Top Right) */}
            <div className="absolute -top-3 -right-2 bg-white px-3 py-2 rounded-2xl shadow-lg border border-slate-100 flex items-center gap-2 transform rotate-3">
              <div className="w-8 h-8 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center font-bold text-xs">
                👤
              </div>
              <div className="text-left">
                <div className="text-[10px] font-bold text-slate-800">Support 24/7</div>
                <div className="text-[8px] text-emerald-600 font-semibold">● Online</div>
              </div>
            </div>

            {/* Floating Key Graphic */}
            <div className="absolute bottom-8 right-0 text-2xl transform rotate-45 animate-bounce" style={{ animationDuration: '4s' }}>
              🔑
            </div>
          </div>
        ) : (
          /* Register Illustration: Mobile + Fingerprint + Woman Avatar + Network Badges */
          <div className="relative w-72 h-80 flex items-center justify-center">
            {/* Isometric Base Shadow */}
            <div className="absolute bottom-2 w-64 h-24 bg-gradient-to-r from-amber-300/30 to-blue-300/30 rounded-full blur-xl transform rotate-12 scale-y-50"></div>

            {/* Smartphone Card */}
            <div className="relative w-44 h-72 bg-gradient-to-b from-white to-amber-50/80 rounded-3xl border-4 border-white shadow-2xl p-3 flex flex-col justify-between transform rotate-6 transition-transform duration-500 hover:rotate-0">
              <div className="w-12 h-1.5 bg-slate-200 rounded-full mx-auto mb-2"></div>

              {/* Fingerprint Scanner on Phone */}
              <div className="space-y-3 flex-1 flex flex-col items-center justify-center">
                <div className="w-16 h-16 rounded-2xl bg-amber-100 border-2 border-amber-400 flex items-center justify-center text-amber-600 text-3xl shadow-inner animate-pulse">
                  👆
                </div>
                <div className="w-24 h-2.5 bg-amber-500/20 rounded-full"></div>
                <div className="w-16 h-2 bg-slate-200 rounded-full"></div>
              </div>

              <div className="w-full py-2 bg-gradient-to-r from-amber-500 to-amber-600 rounded-xl text-[10px] text-white text-center font-bold">
                Auto Sync 100%
              </div>
            </div>

            {/* Social Network Floating Bubbles */}
            <div className="absolute -top-4 -left-2 bg-white px-3 py-1.5 rounded-2xl shadow-lg border border-slate-100 flex items-center gap-1.5 animate-bounce" style={{ animationDuration: '3.5s' }}>
              <span className="w-5 h-5 rounded-full bg-blue-600 text-white flex items-center justify-center font-bold text-[10px]">f</span>
              <span className="text-[10px] font-bold text-slate-700">Facebook</span>
            </div>

            <div className="absolute top-12 -right-4 bg-white px-3 py-1.5 rounded-2xl shadow-lg border border-slate-100 flex items-center gap-1.5 animate-bounce" style={{ animationDuration: '4.5s' }}>
              <span className="w-5 h-5 rounded-full bg-slate-900 text-white flex items-center justify-center font-bold text-[10px]">🎵</span>
              <span className="text-[10px] font-bold text-slate-700">TikTok</span>
            </div>

            {/* Woman Avatar Graphic (Right) */}
            <div className="absolute -bottom-2 -right-2 bg-white p-2.5 rounded-2xl shadow-xl border border-slate-100 flex items-center gap-2 transform -rotate-3">
              <div className="w-9 h-9 rounded-full bg-gradient-to-tr from-amber-400 to-rose-400 text-white flex items-center justify-center text-base shadow">
                👩‍💼
              </div>
              <div className="text-left">
                <div className="text-[10px] font-bold text-slate-800">Max Automation</div>
                <div className="text-[8px] text-amber-600 font-semibold">Multi-Platform</div>
              </div>
            </div>

            {/* Floating Security Locks */}
            <div className="absolute bottom-16 -left-4 text-2xl transform -rotate-12 animate-pulse">
              🔐
            </div>
          </div>
        )}
      </div>

      {/* Decorative Footer Badge inside Banner */}
      <div className="relative z-10 flex items-center gap-2 text-[11px] font-semibold text-slate-400">
        <span className="w-2 h-2 rounded-full bg-amber-500 animate-ping"></span>
        Phone Control Platform Security Standard v2.4
      </div>
    </div>
  );
};
