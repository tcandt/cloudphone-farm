import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, User, LogOut, Menu, BellRing } from 'lucide-react';
import { mockCurrentUserSession } from '../../../data/mockData';
import { useNavigate } from 'react-router-dom';
import { useAdminUiStore } from '../../../stores/useAdminUiStore';

export const AdminHeader: React.FC = () => {
  const { i18n } = useTranslation();
  const navigate = useNavigate();
  const { toggleSidebar, toggleMobileDrawer } = useAdminUiStore();

  const [showProfileMenu, setShowProfileMenu] = useState(false);
  const [showLangMenu, setShowLangMenu] = useState(false);

  const toggleLanguage = (lang: string) => {
    i18n.changeLanguage(lang);
    setShowLangMenu(false);
  };

  return (
    <header className="w-full bg-white h-[68px] px-4 md:px-6 flex items-center justify-between gap-4 z-30 flex-shrink-0 border-b border-slate-200">
      {/* LEFT: Hamburger + Identity */}
      <div className="flex items-center gap-3 md:gap-4">
        {/* Mobile/Tablet Drawer Toggle (< lg) */}
        <button
          onClick={toggleMobileDrawer}
          className="lg:hidden p-2 text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-xl transition-all active:scale-95"
          aria-label="Open Admin Menu"
          aria-expanded={false}
        >
          <Menu size={20} />
        </button>

        {/* Desktop Sidebar Toggle (>= lg) */}
        <button
          onClick={toggleSidebar}
          className="hidden lg:block p-2 text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-xl transition-all active:scale-95"
          title="Toggle Sidebar"
        >
          <Menu size={20} />
        </button>

        <div className="flex items-center">
          <span className="font-extrabold text-slate-800 tracking-tight hidden sm:block">
            Platform<span className="text-emerald-600">Admin</span>
          </span>
        </div>
      </div>

      {/* RIGHT: Global Controls */}
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

        {/* Notification Indicator */}
        <button className="p-2 text-slate-400 hover:text-slate-700 hover:bg-slate-50 rounded-xl transition-colors relative">
          <BellRing size={18} />
          <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-rose-500 rounded-full border-2 border-white"></span>
        </button>

        {/* Profile Menu */}
        <div className="relative">
          <button
            onClick={() => setShowProfileMenu(!showProfileMenu)}
            className="flex items-center gap-2.5 p-1 rounded-xl hover:bg-slate-50 transition-colors"
          >
            <img
              src={mockCurrentUserSession.avatar_url}
              alt="Admin Avatar"
              className="w-8 h-8 rounded-xl object-cover ring-1 ring-slate-200"
            />
            <div className="hidden md:flex flex-col text-left mr-1">
              <span className="text-xs font-bold text-slate-900 leading-tight truncate max-w-[110px]">
                Administrator
              </span>
            </div>
            <ChevronDown size={14} className="text-slate-400 hidden md:block" />
          </button>

          {showProfileMenu && (
            <div className="absolute right-0 mt-2 w-48 bg-white border border-slate-100 shadow-2xl rounded-2xl py-2 z-50">
              <div className="px-4 py-2 border-b border-slate-50 mb-1">
                <p className="text-xs font-bold text-slate-900">Administrator</p>
                <p className="text-[11px] text-slate-500 truncate">{mockCurrentUserSession.email}</p>
              </div>
              <button
                onClick={() => {
                  setShowProfileMenu(false);
                  navigate('/admin/settings');
                }}
                className="w-full text-left px-4 py-2 text-xs font-medium text-slate-700 hover:bg-slate-50 flex items-center gap-2"
              >
                <User size={15} />
                <span>Cài đặt hệ thống</span>
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
