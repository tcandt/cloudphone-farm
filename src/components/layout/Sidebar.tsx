import React, { useState } from 'react';
import { NavLink } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  LayoutDashboard,
  Smartphone,
  Grid,
  List,
  FolderGit2,
  ShieldCheck,
  Activity,
  History,
  Users,
  ShoppingBag,
  User,
  Globe,
  Radio,
  CreditCard,
  Eye,
  Server,
  ChevronDown,
  ChevronRight,
  X,
} from 'lucide-react';
import { useUiStore } from '../../stores/useUiStore';

interface NavItemProps {
  to: string;
  icon: React.ReactNode;
  label: string;
  badge?: string;
  badgeColor?: 'pink' | 'amber' | 'blue';
}

const NavItem: React.FC<NavItemProps> = ({ to, icon, label, badge, badgeColor = 'pink' }) => {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `flex items-center gap-3 px-4 py-2.5 rounded-xl transition-all duration-200 font-medium text-xs select-none ${
          isActive
            ? 'bg-pcp-activeBg text-pcp-activeText border-l-4 border-pcp-activeBorder shadow-sm font-semibold'
            : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'
        }`
      }
    >
      <span className="text-base flex-shrink-0">{icon}</span>
      <span className="truncate flex-1">{label}</span>
      {badge && (
        <span
          className={`text-[9px] uppercase font-bold px-2 py-0.5 rounded-full ${
            badgeColor === 'pink'
              ? 'bg-rose-500 text-white shadow-sm animate-pulse'
              : badgeColor === 'amber'
              ? 'bg-amber-500 text-white'
              : 'bg-blue-500 text-white'
          }`}
        >
          {badge}
        </span>
      )}
    </NavLink>
  );
};

interface NavGroupProps {
  title: string;
  icon: React.ReactNode;
  isOpenDefault?: boolean;
  children: React.ReactNode;
}

const NavGroup: React.FC<NavGroupProps> = ({ title, icon, isOpenDefault = true, children }) => {
  const [isOpen, setIsOpen] = useState(isOpenDefault);

  return (
    <div className="my-2 space-y-1">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between px-4 py-1 text-[11px] font-bold uppercase tracking-wider text-slate-400 hover:text-slate-600 transition-colors"
      >
        <div className="flex items-center gap-2">
          <span>{icon}</span>
          <span>{title}</span>
        </div>
        <span>{isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</span>
      </button>
      {isOpen && <div className="space-y-1 pl-1">{children}</div>}
    </div>
  );
};

export const Sidebar: React.FC = () => {
  const { t } = useTranslation();
  const { isSidebarCollapsed, toggleSidebar, featureRentalStore } = useUiStore();

  return (
    <aside
      className={`bg-white transition-all duration-300 flex flex-col flex-shrink-0 z-20 ${
        isSidebarCollapsed
          ? 'w-0 opacity-0 overflow-hidden pointer-events-none p-0 border-0'
          : 'w-64 py-2 px-3'
      }`}
    >
      {/* Mobile close button */}
      <div className="md:hidden flex justify-end pb-2">
        <button onClick={toggleSidebar} className="p-1 text-slate-400 hover:text-slate-700">
          <X size={18} />
        </button>
      </div>

      {/* Nav Items Scroll Area */}
      <div className="flex-1 overflow-y-auto space-y-2 custom-scrollbar pr-1 pt-1">
        {/* Overview */}
        <NavItem
          to="/app"
          icon={<LayoutDashboard size={18} />}
          label={t('nav.dashboard')}
        />

        {/* Devices Group */}
        <NavGroup title={t('nav.devices')} icon={<Smartphone size={14} />}>
          <NavItem
            to="/app/devices/grid"
            icon={<Grid size={18} />}
            label={t('nav.deviceGrid')}
            badge="HOT"
            badgeColor="pink"
          />
          <NavItem
            to="/app/devices"
            icon={<List size={18} />}
            label={t('nav.deviceList')}
          />
          <NavItem
            to="/app/groups"
            icon={<FolderGit2 size={18} />}
            label={t('nav.groupsTags')}
          />
          <NavItem
            to="/app/agents"
            icon={<ShieldCheck size={18} />}
            label={t('nav.agents')}
          />
        </NavGroup>

        {/* Operations */}
        <NavGroup title={t('nav.operations')} icon={<Activity size={14} />}>
          <NavItem
            to="/app/live-monitor"
            icon={<Eye size={18} />}
            label="LIVE Monitor"
          />
          <NavItem
            to="/app/sessions"
            icon={<Radio size={18} />}
            label={t('nav.activeSessions')}
          />
          <NavItem
            to="/app/proxy"
            icon={<Globe size={18} />}
            label={t('header.networkProfiles')}
          />
          <NavItem
            to="/app/audit"
            icon={<History size={18} />}
            label={t('nav.auditLogs')}
          />
        </NavGroup>

        {/* Organization */}
        <NavGroup title={t('nav.organization')} icon={<Users size={14} />}>
          <NavItem
            to="/app/team"
            icon={<Users size={18} />}
            label={t('nav.team')}
          />
          <NavItem
            to="/app/billing"
            icon={<CreditCard size={18} />}
            label={t('nav.commerce')}
          />
          <NavItem
            to="/app/diagnostics"
            icon={<Server size={18} />}
            label="Diagnostics & Infra"
          />
        </NavGroup>

        {/* Commerce - Optional under Feature Flag */}
        {featureRentalStore && (
          <NavGroup title="Preview Store" icon={<ShoppingBag size={14} />}>
            <NavItem
              to="/app/rental"
              icon={<ShoppingBag size={18} />}
              label={t('nav.rentalStore')}
              badge="Preview"
              badgeColor="amber"
            />
          </NavGroup>
        )}

        {/* Settings */}
        <NavGroup title={t('nav.settings')} icon={<User size={14} />}>
          <NavItem
            to="/app/settings"
            icon={<User size={18} />}
            label={t('nav.settings')}
          />
        </NavGroup>
      </div>

      {/* Footer System Status */}
      <div className="pt-3 mt-2 border-t border-slate-100 bg-slate-50/80 rounded-xl p-2.5 text-center">
        <div className="flex items-center justify-between text-[11px] text-slate-500">
          <span>Server Status:</span>
          <span className="flex items-center gap-1 font-bold text-emerald-600">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-ping inline-block" /> Ready
          </span>
        </div>
        <div className="text-[10px] text-slate-400 mt-1">Version v1.0.0-prototype</div>
      </div>
    </aside>
  );
};
