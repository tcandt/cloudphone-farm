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
    <header className="w-full bg-white px-6 py-3 flex items-center justify-between gap-4 z-30 flex-shrink-0">
      {/* Left: Brand Logo + Hamburger Button + Action Pills */}
      <div className="flex items-center gap-4 flex-wrap">
        {/* Brand Logo (ALWAYS VISIBLE - NEVER HIDDEN WHEN SIDEBAR COLLAPSES) */}
        <div className="flex items-center gap-2.5 cursor-pointer" onClick={() => navigate('/app')}>
          <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-blue-600 via-indigo-600 to-amber-500 flex items-center justify-center text-white font-black text-lg shadow-md shadow-blue-500/20 flex-shrink-0">
            P
          </div>
          <div className="flex flex-col">
            <span className="font-extrabold text-slate-900 leading-tight text-base tracking-tight">
              Phone Control
            </span>
            <span className="text-[10px] font-bold text-amber-600 tracking-wider uppercase flex items-center gap-1">
              <Sparkles size={10} /> Platform Pro
            </span>
          </div>
        </div>

        {/* 3-line Hamburger Toggle Button */}
        <button
          onClick={toggleSidebar}
          className="p-2.5 bg-amber-50 hover:bg-amber-100 text-amber-800 rounded-xl transition-all active:scale-95 border border-amber-200/60"
          title="Đóng / Mở Menu Sidebar"
        >
          <Menu size={18} />
        </button>

        {/* Action Pills */}
        <div className="hidden sm:flex items-center gap-2">
          <button
            onClick={() => navigate('/app/settings')}
            className="flex items-center gap-2 px-3.5 py-2 rounded-xl bg-gradient-to-r from-blue-600 to-cyan-500 text-white font-bold text-xs shadow-md shadow-blue-500/15 hover:opacity-95 transition-all active:scale-95"
          >
            <Globe size={15} />
            <span>{t('header.networkProfiles')}</span>
          </button>

          <button
            onClick={() => navigate('/app/agents')}
            className="flex items-center gap-2 px-3.5 py-2 rounded-xl bg-gradient-to-r from-purple-600 to-pink-500 text-white font-bold text-xs shadow-md shadow-purple-500/15 hover:opacity-95 transition-all active:scale-95"
          >
            <PlusCircle size={15} />
            <span>{t('header.enrollDevice')}</span>
          </button>
        </div>
      </div>

      {/* Right Metrics + Lang + Bell + Balance + Profile */}
      <div className="flex items-center gap-3">
        {/* Live Metrics Pill */}
        <div className="hidden xl:flex items-center gap-3 px-3 py-1.5 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium text-slate-600">
          <span className="flex items-center gap-1.5">
            <Smartphone size={14} className="text-blue-500" />
            <span className="font-bold text-slate-800">3/5</span> Online
          </span>
          <span className="text-slate-300">|</span>
          <span className="flex items-center gap-1 text-emerald-600 font-semibold">
            <Wifi size={14} /> 18ms
          </span>
        </div>

        {/* Language Switcher Dropdown */}
        <div className="relative">
          <button
            onClick={() => setShowLangMenu(!showLangMenu)}
            className="flex items-center gap-1.5 px-3 py-2 bg-slate-50 hover:bg-slate-100 border border-slate-200 rounded-xl text-xs font-semibold text-slate-700 transition-colors"
          >
            <span>{i18n.language === 'vi' ? '🇻🇳 Tiếng Việt' : '🇬🇧 English'}</span>
            <ChevronDown size={14} />
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
          className="relative p-2 text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-xl transition-colors"
          title="Thông báo hệ thống"
        >
          <Bell size={18} />
          <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-rose-500 rounded-full ring-2 ring-white animate-ping" />
          <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-rose-500 rounded-full" />
        </button>

        {/* Balance Pill */}
        <div className="hidden sm:flex items-center gap-2 bg-amber-50 border border-amber-200/80 rounded-xl px-3 py-1.5">
          <Wallet size={16} className="text-amber-600" />
          <div className="flex flex-col">
            <span className="text-[10px] text-amber-700 font-bold uppercase leading-none">{t('header.balance')}</span>
            <span className="text-xs font-extrabold text-amber-900 leading-tight">
              ${mockCurrentUserSession.balance_usd.toFixed(2)}
            </span>
          </div>
        </div>

        {/* Profile Dropdown */}
        <div className="relative">
          <button
            onClick={() => setShowProfileMenu(!showProfileMenu)}
            className="flex items-center gap-2.5 p-1 rounded-xl hover:bg-slate-100 transition-colors"
          >
            <img
              src={mockCurrentUserSession.avatar_url}
              alt="Avatar"
              className="w-8 h-8 rounded-xl object-cover ring-2 ring-blue-500/20"
            />
            <div className="hidden md:flex flex-col text-left">
              <span className="text-xs font-bold text-slate-900 leading-tight truncate max-w-[110px]">
                {mockCurrentUserSession.display_name}
              </span>
              <span className="text-[10px] text-blue-600 font-bold uppercase tracking-wider">
                {mockCurrentUserSession.role}
              </span>
            </div>
            <ChevronDown size={14} className="text-slate-400" />
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
