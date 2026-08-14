import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Globe,
  PlusCircle,
  Bell,
  Wallet,
  ChevronDown,
  User,
  Shield,
  LogOut,
  Wifi,
  Smartphone,
  Menu,
  Sparkles,
} from 'lucide-react';
import { mockCurrentUserSession } from '../../data/mockData';
import { useNavigate } from 'react-router-dom';
import { useUiStore } from '../../stores/useUiStore';

export const Header: React.FC = () => {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { toggleSidebar } = useUiStore();

  const [showProfileMenu, setShowProfileMenu] = useState(false);
  const [showLangMenu, setShowLangMenu] = useState(false);

  const toggleLanguage = (lang: string) => {
    i18n.changeLanguage(lang);
    setShowLangMenu(false);
  };

  return (
    <header className="w-full bg-white px-4 md:px-6 h-14 min-h-[56px] flex items-center justify-between gap-3 z-30 flex-shrink-0 border-b border-slate-200/80 flex-nowrap overflow-x-auto scrollbar-none">
      {/* Left: Brand Logo + Hamburger Button + Action Pills (INLINE SINGLE ROW, NEVER WRAPPING) */}
      <div className="flex items-center gap-3 flex-shrink-0 flex-nowrap">
        {/* Brand Logo */}
        <div className="flex items-center gap-2 cursor-pointer flex-shrink-0" onClick={() => navigate('/app')}>
          <div className="w-8 h-8 rounded-xl bg-gradient-to-tr from-blue-600 via-indigo-600 to-amber-500 flex items-center justify-center text-white font-black text-base shadow-md shadow-blue-500/20 flex-shrink-0">
            P
          </div>
          <div className="flex flex-col flex-shrink-0">
            <span className="font-extrabold text-slate-900 leading-none text-sm tracking-tight whitespace-nowrap">
              Phone Control
            </span>
            <span className="text-[9px] font-bold text-amber-600 tracking-wider uppercase flex items-center gap-0.5 whitespace-nowrap">
              <Sparkles size={9} /> Platform Pro
            </span>
          </div>
        </div>

        {/* 3-line Hamburger Toggle Button */}
        <button
          onClick={toggleSidebar}
          className="p-2 bg-amber-50 hover:bg-amber-100 text-amber-800 rounded-xl transition-all active:scale-95 border border-amber-200/60 flex-shrink-0"
          title="Đóng / Mở Menu Sidebar"
        >
          <Menu size={16} />
        </button>

        {/* Inline Action Pills (Single line next to hamburger, styled like Image 2) */}
        <div className="flex items-center gap-2 flex-shrink-0 flex-nowrap">
          <button
            onClick={() => navigate('/app/settings')}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-gradient-to-r from-sky-500 via-cyan-500 to-blue-600 text-white font-bold text-xs shadow-sm shadow-cyan-500/20 hover:opacity-95 transition-all active:scale-95 whitespace-nowrap flex-shrink-0"
          >
            <Globe size={14} />
            <span>{t('header.networkProfiles')}</span>
          </button>

          <button
            onClick={() => navigate('/app/agents')}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-gradient-to-r from-purple-500 via-pink-500 to-indigo-600 text-white font-bold text-xs shadow-sm shadow-purple-500/20 hover:opacity-95 transition-all active:scale-95 whitespace-nowrap flex-shrink-0"
          >
            <PlusCircle size={14} />
            <span>{t('header.enrollDevice')}</span>
          </button>
        </div>
      </div>

      {/* Right Metrics + Lang + Bell + Balance + Profile */}
      <div className="flex items-center gap-2.5 flex-shrink-0 flex-nowrap">
        {/* Live Metrics Pill */}
        <div className="hidden lg:flex items-center gap-2.5 px-2.5 py-1 bg-slate-50 border border-slate-200/80 rounded-xl text-xs font-medium text-slate-600 flex-shrink-0 whitespace-nowrap">
          <span className="flex items-center gap-1">
            <Smartphone size={13} className="text-blue-500" />
            <span className="font-bold text-slate-800">3/5</span> Online
          </span>
          <span className="text-slate-300">|</span>
          <span className="flex items-center gap-1 text-emerald-600 font-semibold">
            <Wifi size={13} /> 18ms
          </span>
        </div>

        {/* Language Switcher Dropdown */}
        <div className="relative flex-shrink-0">
          <button
            onClick={() => setShowLangMenu(!showLangMenu)}
            className="flex items-center gap-1.5 px-2.5 py-1.5 bg-slate-50 hover:bg-slate-100 border border-slate-200/80 rounded-xl text-xs font-semibold text-slate-700 transition-colors whitespace-nowrap"
          >
            <span>{i18n.language === 'vi' ? '🇻🇳 Tiếng Việt' : '🇬🇧 English'}</span>
            <ChevronDown size={13} />
          </button>

          {showLangMenu && (
            <div className="absolute right-0 mt-2 w-36 bg-white border border-slate-100 shadow-xl rounded-xl py-1 z-50">
              <button
                onClick={() => toggleLanguage('vi')}
                className="w-full text-left px-3 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50 flex items-center gap-2"
              >
                <span>🇻🇳 Tiếng Việt</span>
              </button>
              <button
                onClick={() => toggleLanguage('en')}
                className="w-full text-left px-3 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50 flex items-center gap-2"
              >
                <span>🇬🇧 English</span>
              </button>
            </div>
          )}
        </div>

        {/* Notification Bell */}
        <button
          onClick={() => navigate('/app/audit')}
          className="relative p-1.5 text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-xl transition-colors flex-shrink-0"
          title="Thông báo hệ thống"
        >
          <Bell size={17} />
          <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-rose-500 rounded-full ring-2 ring-white animate-ping" />
          <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-rose-500 rounded-full" />
        </button>

        {/* Balance Pill */}
        <div className="hidden sm:flex items-center gap-2 bg-amber-50 border border-amber-200/80 rounded-xl px-2.5 py-1 flex-shrink-0 whitespace-nowrap">
          <Wallet size={15} className="text-amber-600" />
          <div className="flex flex-col">
            <span className="text-[9px] text-amber-700 font-bold uppercase leading-none">{t('header.balance')}</span>
            <span className="text-xs font-extrabold text-amber-900 leading-tight">
              ${mockCurrentUserSession.balance_usd.toFixed(2)}
            </span>
          </div>
        </div>

        {/* Profile Dropdown */}
        <div className="relative flex-shrink-0">
          <button
            onClick={() => setShowProfileMenu(!showProfileMenu)}
            className="flex items-center gap-2 p-1 rounded-xl hover:bg-slate-100 transition-colors"
          >
            <img
              src={mockCurrentUserSession.avatar_url}
              alt="Avatar"
              className="w-7 h-7 rounded-xl object-cover ring-2 ring-blue-500/20"
            />
            <div className="hidden md:flex flex-col text-left">
              <span className="text-xs font-bold text-slate-900 leading-tight truncate max-w-[100px]">
                {mockCurrentUserSession.display_name}
              </span>
              <span className="text-[9px] text-blue-600 font-bold uppercase tracking-wider">
                {mockCurrentUserSession.role}
              </span>
            </div>
            <ChevronDown size={13} className="text-slate-400" />
          </button>

          {showProfileMenu && (
            <div className="absolute right-0 mt-2 w-56 bg-white border border-slate-100 shadow-2xl rounded-2xl py-2 z-50 divide-y divide-slate-100">
              <div className="px-4 py-2">
                <p className="text-xs font-bold text-slate-900">{mockCurrentUserSession.display_name}</p>
                <p className="text-[11px] text-slate-500 truncate">{mockCurrentUserSession.email}</p>
                <span className="inline-block mt-1 text-[10px] font-extrabold px-2 py-0.5 rounded-md bg-blue-50 text-blue-700 border border-blue-200">
                  {mockCurrentUserSession.organization_name}
                </span>
              </div>
              <div className="py-1">
                <button
                  onClick={() => {
                    setShowProfileMenu(false);
                    navigate('/app/settings');
                  }}
                  className="w-full text-left px-4 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50 flex items-center gap-2"
                >
                  <User size={15} />
                  <span>{t('header.profile')}</span>
                </button>
                <button
                  onClick={() => {
                    setShowProfileMenu(false);
                    navigate('/app/team');
                  }}
                  className="w-full text-left px-4 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50 flex items-center gap-2"
                >
                  <Shield size={15} />
                  <span>{t('nav.team')}</span>
                </button>
              </div>
              <div className="py-1">
                <button
                  onClick={() => {
                    setShowProfileMenu(false);
                    navigate('/login');
                  }}
                  className="w-full text-left px-4 py-2 text-xs font-semibold text-rose-600 hover:bg-rose-50 flex items-center gap-2"
                >
                  <LogOut size={15} />
                  <span>{t('header.logout')}</span>
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  );
};
