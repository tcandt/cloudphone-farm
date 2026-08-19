import React from 'react';
import { NavLink } from 'react-router-dom';
import { adminNavGroups } from '../navigation/adminNav';
import { useAdminUiStore } from '../../../stores/useAdminUiStore';
import { BrandLogo } from '@brand/index';
import { ChevronRight } from 'lucide-react';

export const AdminSidebar: React.FC = () => {
  const { isSidebarCollapsed } = useAdminUiStore();

  return (
    <aside
      className={`hidden lg:flex flex-col bg-white border-r border-slate-200 h-screen sticky top-0 transition-all duration-300 z-40 ${
        isSidebarCollapsed ? 'w-[72px]' : 'w-[260px]'
      }`}
    >
      {/* BRANDING */}
      <div className="h-[68px] flex items-center px-4 flex-shrink-0">
        <BrandLogo variant={isSidebarCollapsed ? 'mark' : 'full'} size="sm" />
      </div>

      {/* NAVIGATION */}
      <nav className="flex-1 overflow-y-auto custom-scrollbar py-4 px-3 space-y-6">
        {adminNavGroups.map((group, groupIndex) => (
          <div key={groupIndex} className="flex flex-col space-y-1">
            {!isSidebarCollapsed && (
              <div className="px-3 mb-2 text-[10px] font-bold uppercase tracking-wider text-slate-400">
                {group.groupName}
              </div>
            )}
            {group.items.map((item, itemIndex) => {
              const Icon = item.icon;
              return (
                <NavLink
                  key={itemIndex}
                  to={item.href}
                  className={({ isActive }) =>
                    `group relative flex items-center gap-3 rounded-xl transition-all duration-200 ${
                      isSidebarCollapsed ? 'p-3 justify-center' : 'px-3 py-2.5'
                    } ${
                      isActive
                        ? 'bg-emerald-50 text-emerald-700 font-bold'
                        : 'text-slate-600 font-semibold hover:bg-slate-50 hover:text-slate-900'
                    }`
                  }
                  title={isSidebarCollapsed ? item.title : undefined}
                  aria-label={item.title}
                >
                  {({ isActive }) => (
                    <>
                      <Icon
                        size={20}
                        className={isActive ? 'text-emerald-600' : 'text-slate-400 group-hover:text-slate-600'}
                      />
                      {!isSidebarCollapsed && (
                        <span className="flex-1 truncate text-sm">{item.title}</span>
                      )}
                      {!isSidebarCollapsed && isActive && (
                        <ChevronRight size={16} className="text-emerald-500/50" />
                      )}
                    </>
                  )}
                </NavLink>
              );
            })}
          </div>
        ))}
      </nav>
    </aside>
  );
};
