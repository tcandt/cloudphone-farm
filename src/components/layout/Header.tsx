import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Wallet, ChevronDown, User, LogOut, Menu } from 'lucide-react';
import { mockCurrentUserSession } from '../../data/mockData';
import { useNavigate } from 'react-router-dom';
import { useUiStore } from '../../stores/useUiStore';
import { BrandLogo } from '@brand/index';

export const Header: React.FC = () => {
  const { i18n } = useTranslation();
  const navigate = useNavigate();
  const { toggleSidebar } = useUiStore();

  const [showProfileMenu, setShowProfileMenu] = useState(false);
  const [showLangMenu, setShowLangMenu] = useState(false);

  const toggleLanguage = (lang: string) => {
    i18n.changeLanguage(lang);
    setShowLangMenu(false);
  };

  return (
    <header className="w-full bg-white h-[68px] px-4 md:px-6 flex items-center justify-between gap-4 z-30 flex-shrink-0">
      {/* LEFT: Logo (Mobile) + Hamburger */}
      <div className="flex items-center gap-3">
        {/* Mobile Brand Logo */}
        <div className="md:hidden cursor-pointer mr-1" onClick={() => navigate('/app/store')}>
          <BrandLogo variant="mark" size="sm" />
        </div>

        {/* Collapse / Expand Toggle */}
        <button
          onClick={toggleSidebar}
          className="p-2 text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-xl transition-all active:scale-95"
          title="Toggle Sidebar"
        >
          <Menu size={20} />
        </button>
      </div>

      {/* RIGHT: Lang + Wallet + Profile */}
      <div className="flex items-center gap-2 md:gap-4">
        {/* Language Switcher */}
        <div className="relative hidden sm:block">
          <button
            onClick={() => setShowLangMenu(!showLangMenu)}
            className="flex items-center gap-1.5 px-3 py-2 bg-transparent hover:bg-slate-50 rounded-xl text-xs font-semibold text-slate-700 transition-colors"
          >
            <span>{i18n.language === 'vi' ? 'VN' : 'EN'}</span>
            <ChevronDown size={14} />
          </button>

          {showLangMenu && (
            <div className="absolute right-0 mt-2 w-32 bg-white border border-slate-100 shadow-xl rounded-xl py-1 z-50">
              <button
                onClick={() => toggleLanguage('vi')}
                className="w-full text-left px-4 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50"
              >
                🇻🇳 Tiếng Việt
              </button>
              <button
                onClick={() => toggleLanguage('en')}
                className="w-full text-left px-4 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50"
              >
                🇬🇧 English
              </button>
            </div>
          )}
        </div>

        {/* Wallet Indicator */}
        <div className="flex items-center gap-2 bg-slate-50 border border-slate-100 rounded-xl px-3 py-1.5 cursor-pointer hover:bg-slate-100 transition-colors" onClick={() => navigate('/app/wallet')}>
          <Wallet size={16} className="text-emerald-600" />
          <div className="flex flex-col">
            <span className="text-[10px] text-slate-500 font-bold uppercase leading-none">Số dư</span>
            <span className="text-xs font-extrabold text-slate-900 leading-tight">
              ${mockCurrentUserSession.balance_usd.toFixed(2)}
            </span>
          </div>
        </div>

        {/* Profile Menu */}
        <div className="relative">
          <button
            onClick={() => setShowProfileMenu(!showProfileMenu)}
            className="flex items-center gap-2.5 p-1 rounded-xl hover:bg-slate-50 transition-colors"
          >
            <img
              src={mockCurrentUserSession.avatar_url}
              alt="Avatar"
              className="w-8 h-8 rounded-xl object-cover ring-1 ring-slate-200"
            />
            <div className="hidden md:flex flex-col text-left mr-1">
              <span className="text-xs font-bold text-slate-900 leading-tight truncate max-w-[110px]">
                {mockCurrentUserSession.display_name}
              </span>
            </div>
            <ChevronDown size={14} className="text-slate-400 hidden md:block" />
          </button>

          {showProfileMenu && (
            <div className="absolute right-0 mt-2 w-48 bg-white border border-slate-100 shadow-2xl rounded-2xl py-2 z-50">
              <div className="px-4 py-2 border-b border-slate-50 mb-1">
                <p className="text-xs font-bold text-slate-900">{mockCurrentUserSession.display_name}</p>
                <p className="text-[11px] text-slate-500 truncate">{mockCurrentUserSession.email}</p>
              </div>
              <button
                onClick={() => {
                  setShowProfileMenu(false);
                  navigate('/app/settings');
                }}
                className="w-full text-left px-4 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50 flex items-center gap-2"
              >
                <User size={15} />
                <span>Hồ sơ</span>
              </button>
              <button
                onClick={() => {
                  setShowProfileMenu(false);
                  navigate('/login');
                }}
                className="w-full text-left px-4 py-2 text-xs font-semibold text-rose-600 hover:bg-rose-50 flex items-center gap-2"
              >
                <LogOut size={15} />
                <span>Đăng xuất</span>
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
};
