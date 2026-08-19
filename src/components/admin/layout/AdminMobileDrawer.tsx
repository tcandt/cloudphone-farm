import React, { useEffect, useRef } from 'react';
import { NavLink } from 'react-router-dom';
import { adminNavGroups } from '../navigation/adminNav';
import { useAdminUiStore } from '../../../stores/useAdminUiStore';
import { BrandLogo } from '@brand/index';
import { X, ChevronRight } from 'lucide-react';

export const AdminMobileDrawer: React.FC = () => {
  const { isMobileDrawerOpen, setMobileDrawerOpen } = useAdminUiStore();
  const drawerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);

  // Store the active element that triggered the drawer
  useEffect(() => {
    if (isMobileDrawerOpen) {
      triggerRef.current = document.activeElement as HTMLElement;
      // Lock body scroll
      document.body.style.overflow = 'hidden';
      // Focus first element inside drawer
      if (drawerRef.current) {
        drawerRef.current.focus();
      }
    } else {
      // Restore body scroll
      document.body.style.overflow = '';
      // Return focus to trigger
      if (triggerRef.current) {
        triggerRef.current.focus();
      }
    }

    return () => {
      document.body.style.overflow = '';
    };
  }, [isMobileDrawerOpen]);

  // Handle Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isMobileDrawerOpen) {
        setMobileDrawerOpen(false);
      }
      
      // Basic focus trap inside the drawer
      if (e.key === 'Tab' && isMobileDrawerOpen && drawerRef.current) {
        const focusableElements = drawerRef.current.querySelectorAll(
          'a[href], button:not([disabled]), textarea:not([disabled]), input[type="text"]:not([disabled]), input[type="radio"]:not([disabled]), input[type="checkbox"]:not([disabled]), select:not([disabled])'
        );
        const firstElement = focusableElements[0] as HTMLElement;
        const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

        if (e.shiftKey) {
          if (document.activeElement === firstElement) {
            lastElement.focus();
            e.preventDefault();
          }
        } else {
          if (document.activeElement === lastElement) {
            firstElement.focus();
            e.preventDefault();
          }
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isMobileDrawerOpen, setMobileDrawerOpen]);

  if (!isMobileDrawerOpen) return null;

  return (
    <div className="fixed inset-0 z-50 lg:hidden flex">
      {/* Overlay */}
      <div 
        className="fixed inset-0 bg-slate-900/40 backdrop-blur-sm transition-opacity" 
        onClick={() => setMobileDrawerOpen(false)}
        aria-hidden="true"
      />
      
      {/* Drawer */}
      <div 
        ref={drawerRef}
        tabIndex={-1}
        className="relative w-4/5 max-w-sm bg-white h-full shadow-2xl flex flex-col focus:outline-none"
        role="dialog"
        aria-modal="true"
        aria-label="Admin Navigation Menu"
      >
        <div className="h-[68px] flex items-center justify-between px-4 border-b border-slate-100 flex-shrink-0">
          <BrandLogo variant="full" size="sm" />
          <button 
            onClick={() => setMobileDrawerOpen(false)}
            className="p-2 text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-xl transition-colors"
            aria-label="Close menu"
          >
            <X size={20} />
          </button>
        </div>

        <nav className="flex-1 overflow-y-auto custom-scrollbar p-4 space-y-6">
          {adminNavGroups.map((group, groupIndex) => (
            <div key={groupIndex} className="flex flex-col space-y-1">
              <div className="px-3 mb-2 text-[10px] font-bold uppercase tracking-wider text-slate-400">
                {group.groupName}
              </div>
              {group.items.map((item, itemIndex) => {
                const Icon = item.icon;
                return (
                  <NavLink
                    key={itemIndex}
                    to={item.href}
                    onClick={() => setMobileDrawerOpen(false)} // Close on navigate
                    className={({ isActive }) =>
                      `group flex items-center gap-3 px-3 py-3 rounded-xl transition-all duration-200 ${
                        isActive
                          ? 'bg-emerald-50 text-emerald-700 font-bold'
                          : 'text-slate-600 font-semibold hover:bg-slate-50 hover:text-slate-900'
                      }`
                    }
                  >
                    {({ isActive }) => (
                      <>
                        <Icon
                          size={20}
                          className={isActive ? 'text-emerald-600' : 'text-slate-400'}
                        />
                        <span className="flex-1 text-sm">{item.title}</span>
                        {isActive && <ChevronRight size={16} className="text-emerald-500/50" />}
                      </>
                    )}
                  </NavLink>
                );
              })}
            </div>
          ))}
        </nav>
      </div>
    </div>
  );
};
