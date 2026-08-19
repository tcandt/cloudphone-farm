import React from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { BottomNav } from './BottomNav';
import { ErrorBoundary } from '@ui/index';

export const Layout: React.FC = () => {
  return (
    <div className="min-h-[100dvh] h-screen bg-white text-pcp-text flex flex-col font-sans antialiased overflow-hidden">
      {/* Seamless Top Header Bar */}
      <Header />

      {/* Main Body Layout */}
      <div className="flex flex-1 relative overflow-hidden bg-white">
        {/* Seamless Left Sidebar (Desktop Only) */}
        <Sidebar />

        {/* Workspace Geometry: 4-corner rounded light gray body with white breathing space */}
        <div className="flex-1 flex flex-col min-w-0 pr-2 pl-2 pb-2 pt-0 md:pr-4 md:pl-0 md:pb-4 md:pt-0 transition-all duration-300">
          <main className="flex-1 overflow-y-auto bg-[#f3f4f8] rounded-2xl md:rounded-[24px] flex flex-col relative pb-20 md:pb-4 p-4 md:p-6 animate-fadeIn">
            <ErrorBoundary>
              <Outlet />
            </ErrorBoundary>
          </main>
        </div>
      </div>

      {/* Mobile Bottom Navigation */}
      <BottomNav />
    </div>
  );
};
