import React, { useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { AdminHeader } from './AdminHeader';
import { AdminSidebar } from './AdminSidebar';
import { AdminMobileDrawer } from './AdminMobileDrawer';

export const AdminLayout: React.FC = () => {
  const { pathname } = useLocation();

  // Scroll to top on navigation
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);

  return (
    <div className="flex h-screen w-full bg-[#F6F8FA] overflow-hidden font-sans">
      <AdminSidebar />
      <AdminMobileDrawer />
      
      <div className="flex flex-col flex-1 min-w-0 h-full relative z-10">
        <AdminHeader />
        
        <main className="flex-1 overflow-y-auto custom-scrollbar relative z-0">
          <Outlet />
        </main>
      </div>
    </div>
  );
};
