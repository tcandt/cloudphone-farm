import React from 'react';
import { Users, Smartphone, Zap, AlertTriangle, ArrowUpRight } from 'lucide-react';

const OVERVIEW_MOCKS = {
  totalCustomers: 124,
  activeDevices: 450,
  offlineDevices: 12,
  currentlyRented: 380,
  alerts: 3,
};

export const OverviewPage: React.FC = () => {
  return (
    <div className="p-4 md:p-8 max-w-7xl mx-auto space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-black text-slate-900 tracking-tight">Tổng quan hệ thống</h1>
        <p className="text-sm text-slate-500 font-medium mt-1">Dữ liệu hiển thị mang tính chất minh họa (Mock Data)</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        {/* Card 1 */}
        <div className="bg-white border border-slate-200/60 rounded-[20px] p-6 shadow-sm">
          <div className="flex items-center gap-4 mb-4">
            <div className="p-3 bg-blue-50 text-blue-600 rounded-2xl">
              <Users size={24} />
            </div>
            <div>
              <p className="text-[11px] font-bold uppercase tracking-wider text-slate-400">Khách hàng</p>
              <h3 className="text-2xl font-black text-slate-900">{OVERVIEW_MOCKS.totalCustomers}</h3>
            </div>
          </div>
          <div className="flex items-center gap-2 text-xs font-semibold text-emerald-600">
            <ArrowUpRight size={14} />
            <span>+12 tuần này</span>
          </div>
        </div>

        {/* Card 2 */}
        <div className="bg-white border border-slate-200/60 rounded-[20px] p-6 shadow-sm">
          <div className="flex items-center gap-4 mb-4">
            <div className="p-3 bg-emerald-50 text-emerald-600 rounded-2xl">
              <Zap size={24} />
            </div>
            <div>
              <p className="text-[11px] font-bold uppercase tracking-wider text-slate-400">Đang hoạt động</p>
              <h3 className="text-2xl font-black text-slate-900">{OVERVIEW_MOCKS.activeDevices}</h3>
            </div>
          </div>
          <div className="flex items-center gap-2 text-xs font-semibold text-slate-500">
            <span>Thiết bị online</span>
          </div>
        </div>

        {/* Card 3 */}
        <div className="bg-white border border-slate-200/60 rounded-[20px] p-6 shadow-sm">
          <div className="flex items-center gap-4 mb-4">
            <div className="p-3 bg-indigo-50 text-indigo-600 rounded-2xl">
              <Smartphone size={24} />
            </div>
            <div>
              <p className="text-[11px] font-bold uppercase tracking-wider text-slate-400">Đang thuê</p>
              <h3 className="text-2xl font-black text-slate-900">{OVERVIEW_MOCKS.currentlyRented}</h3>
            </div>
          </div>
          <div className="flex items-center gap-2 text-xs font-semibold text-slate-500">
            <span>Tỷ lệ lấp đầy ~84%</span>
          </div>
        </div>

        {/* Card 4 */}
        <div className="bg-white border border-rose-100 rounded-[20px] p-6 shadow-sm bg-gradient-to-b from-white to-rose-50/30">
          <div className="flex items-center gap-4 mb-4">
            <div className="p-3 bg-rose-100 text-rose-600 rounded-2xl">
              <AlertTriangle size={24} />
            </div>
            <div>
              <p className="text-[11px] font-bold uppercase tracking-wider text-rose-400">Cần xử lý</p>
              <h3 className="text-2xl font-black text-rose-700">{OVERVIEW_MOCKS.alerts}</h3>
            </div>
          </div>
          <div className="flex items-center gap-2 text-xs font-semibold text-rose-600">
            <span>{OVERVIEW_MOCKS.offlineDevices} thiết bị mất kết nối</span>
          </div>
        </div>
      </div>

      {/* Activity Placeholder */}
      <div className="bg-white border border-slate-200/60 rounded-[24px] p-8 shadow-sm text-center">
        <h3 className="text-lg font-bold text-slate-900 mb-2">Hoạt động gần đây</h3>
        <p className="text-slate-500 text-sm">Biểu đồ và log hoạt động sẽ được tích hợp khi API sẵn sàng.</p>
      </div>
    </div>
  );
};
