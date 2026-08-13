import React from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { useUiStore } from '../../stores/useUiStore';

export const Layout: React.FC = () => {
  const { isSidebarCollapsed } = useUiStore();

  return (
    <div className="min-h-screen bg-white text-pcp-text flex flex-col font-sans antialiased">
      {/* Seamless Top Header Bar */}
      <Header />

      {/* Main Body Layout */}
      <div className="flex flex-1 relative min-h-[calc(100vh-61px)] overflow-hidden bg-white">
        {/* Seamless Left Sidebar */}
        <Sidebar />

        {/* Main Content Area (Right) with Curved Top-Left Inner Corner */}
        <main
          className={`flex-1 p-5 md:p-6 overflow-y-auto min-w-0 bg-[#f3f4f8] transition-all duration-300 animate-fadeIn ${
            !isSidebarCollapsed ? 'rounded-tl-3xl shadow-inner' : ''
          }`}
        >
          <Outlet />
        </main>
      </div>
    </div>
  );
};
