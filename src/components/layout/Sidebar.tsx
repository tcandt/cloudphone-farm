import React from 'react';
import { NavLink } from 'react-router-dom';
import { ShoppingBag, Smartphone, Wallet, BookOpen } from 'lucide-react';
import { useUiStore } from '../../stores/useUiStore';
import { BrandLogo } from '@brand/index';

interface NavItemProps {
  to: string;
  icon: React.ReactNode;
  label: string;
  isCollapsed: boolean;
}

const NavItem: React.FC<NavItemProps> = ({ to, icon, label, isCollapsed }) => {
  return (
    <NavLink
      to={to}
      title={isCollapsed ? label : undefined}
      className={({ isActive }) =>
        `flex items-center rounded-xl transition-all duration-200 select-none group ${
          isCollapsed ? 'justify-center w-12 h-12 mx-auto' : 'px-4 py-3 mx-4 gap-3'
        } ${
          isActive
            ? 'bg-emerald-50 text-emerald-700 font-bold shadow-sm border border-emerald-100/50'
            : 'text-slate-500 hover:bg-slate-50 hover:text-slate-800 font-medium border border-transparent'
        }`
      }
    >
      {({ isActive }) => (
        <>
          <span className={`flex-shrink-0 transition-transform duration-200 ${isActive ? 'scale-110' : 'group-hover:scale-110'}`}>
            {React.cloneElement(icon as React.ReactElement, {
              className: isActive ? 'stroke-[2.5px]' : 'stroke-2',
              size: 22
            })}
          </span>
          {!isCollapsed && <span className="truncate flex-1 text-sm">{label}</span>}
        </>
      )}
    </NavLink>
  );
};

export const Sidebar: React.FC = () => {
  const { isSidebarCollapsed } = useUiStore();

  return (
    <aside
      className={`hidden md:flex flex-col bg-white transition-all duration-300 flex-shrink-0 z-20 ${
        isSidebarCollapsed ? 'w-[72px]' : 'w-[250px]'
      }`}
    >
      <div className={`h-[68px] flex items-center flex-shrink-0 ${isSidebarCollapsed ? 'justify-center' : 'px-6'}`}>
        <BrandLogo variant={isSidebarCollapsed ? 'mark' : 'full'} size="sm" />
      </div>

      <div className="flex-1 overflow-y-auto space-y-2 py-4 custom-scrollbar">
        <NavItem to="/app/store" icon={<ShoppingBag />} label="Cửa hàng cho thuê" isCollapsed={isSidebarCollapsed} />
        <NavItem to="/app/devices" icon={<Smartphone />} label="Quản lý thiết bị" isCollapsed={isSidebarCollapsed} />
        <NavItem to="/app/wallet" icon={<Wallet />} label="Nạp tiền" isCollapsed={isSidebarCollapsed} />
        <NavItem to="/app/docs" icon={<BookOpen />} label="Document" isCollapsed={isSidebarCollapsed} />
      </div>
    </aside>
  );
};
