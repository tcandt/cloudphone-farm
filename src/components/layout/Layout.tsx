import React from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';

export const Layout: React.FC = () => {
  return (
    <div className="min-h-screen bg-slate-100 text-pcp-text flex flex-col font-sans antialiased">
      {/* Single-row Top Header Bar */}
      <Header />

      {/* Main Body Layout */}
      <div className="flex flex-1 relative min-h-[calc(100vh-56px)] overflow-hidden bg-slate-100 p-2 md:p-3.5 gap-3">
        {/* Left Sidebar */}
        <Sidebar />

        {/* Main Content Area (Right) with Rounded Corners on ALL sides (Both Top-Left AND Top-Right) matching Image 2 */}
        <main className="flex-1 p-4 md:p-6 overflow-y-auto min-w-0 bg-[#f8fafc] rounded-2xl md:rounded-3xl border border-slate-200/80 shadow-sm transition-all duration-300 animate-fadeIn">
          <Outlet />
        </main>
      </div>
    </div>
  );
};
