import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Wallet, Download, Award, Layers, LifeBuoy } from 'lucide-react';
import { SupportAccessModal } from '../components/support/SupportAccessModal';

export const BillingPage: React.FC = () => {
  const { t } = useTranslation();
  const [showSupportModal, setShowSupportModal] = useState(false);

  const invoices = [
    { id: 'INV-2026-0801', date: '2026-08-01', plan: 'Enterprise Platform Plan', amount: '$150.00', status: 'PAID' },
    { id: 'INV-2026-0701', date: '2026-07-01', plan: 'Enterprise Platform Plan', amount: '$150.00', status: 'PAID' },
  ];

  return (
    <div className="space-y-6">
      {/* Title & Top Action */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('nav.commerce')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quản lý gói Subscription, giới hạn Quota Entitlement và lịch sử hóa đơn thanh toán
          </p>
        </div>

        <button
          onClick={() => setShowSupportModal(true)}
          className="px-4 py-2.5 bg-amber-500 hover:bg-amber-600 text-white font-bold text-xs rounded-xl shadow-lg shadow-amber-500/20 transition-all flex items-center gap-2 active:scale-95"
        >
          <LifeBuoy size={16} /> Ủy quyền Support Hỗ trợ
        </button>
      </div>

      {/* Quota Entitlement Bars & Plan Info */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Subscription Plan Card */}
        <div className="bg-gradient-to-br from-slate-900 via-indigo-950 to-blue-950 text-white rounded-3xl p-6 space-y-4 shadow-xl">
          <div className="flex items-center justify-between">
            <span className="px-2.5 py-1 rounded-full bg-amber-400 text-slate-950 text-[10px] font-black uppercase">
              Active Tier
            </span>
            <Award size={24} className="text-amber-400" />
          </div>
          <div>
            <h2 className="text-xl font-black">Enterprise Plan</h2>
            <p className="text-xs text-slate-300">Gói hạ tầng doanh nghiệp không giới hạn tính năng</p>
          </div>
          <div className="pt-2">
            <span className="text-3xl font-black">$150.00</span>
            <span className="text-xs text-slate-400"> / tháng</span>
          </div>
        </div>

        {/* Quota Entitlement Usage */}
        <div className="md:col-span-2 bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-4">
          <h2 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
            <Layers size={18} className="text-blue-600" /> Quota Entitlement Hạn Mạch
          </h2>

          <div className="space-y-4">
            {/* Devices Limit */}
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs font-bold">
                <span className="text-slate-700">Thiết bị Quota (Devices Limit)</span>
                <span className="text-slate-900">5 / 10 máy</span>
              </div>
              <div className="w-full h-2.5 bg-slate-100 rounded-full overflow-hidden">
                <div className="h-full bg-blue-600 rounded-full" style={{ width: '50%' }} />
              </div>
            </div>

            {/* Concurrent Streams Limit */}
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs font-bold">
                <span className="text-slate-700">Luồng Stream đồng thời (Concurrent Streams)</span>
                <span className="text-slate-900">2 / 5 luồng</span>
              </div>
              <div className="w-full h-2.5 bg-slate-100 rounded-full overflow-hidden">
                <div className="h-full bg-purple-600 rounded-full" style={{ width: '40%' }} />
              </div>
            </div>

            {/* Control Leases Limit */}
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs font-bold">
                <span className="text-slate-700">Quyền Điều Khởi đồng thời (Exclusive Leases)</span>
                <span className="text-slate-900">1 / 2 máy</span>
              </div>
              <div className="w-full h-2.5 bg-slate-100 rounded-full overflow-hidden">
                <div className="h-full bg-emerald-500 rounded-full" style={{ width: '50%' }} />
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Invoice History Table */}
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="p-4 border-b border-slate-100 bg-slate-50/50">
          <h2 className="text-sm font-extrabold text-slate-900 flex items-center gap-2">
            <Wallet size={18} className="text-blue-600" /> Lịch sử Hóa đơn thanh toán (Invoices)
          </h2>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50/70 border-b border-slate-100 text-[11px] font-extrabold uppercase text-slate-400 tracking-wider">
                <th className="p-4">Mã Hóa Đơn</th>
                <th className="p-4">Ngày Xuất</th>
                <th className="p-4">Gói Dịch Vụ</th>
                <th className="p-4">Số Tiền</th>
                <th className="p-4">Trạng Thái</th>
                <th className="p-4 text-right">Tải Hóa Đơn</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {invoices.map((inv) => (
                <tr key={inv.id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="p-4 font-mono font-bold text-slate-900">{inv.id}</td>
                  <td className="p-4 text-slate-500">{inv.date}</td>
                  <td className="p-4 font-semibold text-slate-800">{inv.plan}</td>
                  <td className="p-4 font-black text-slate-900">{inv.amount}</td>
                  <td className="p-4">
                    <span className="px-2.5 py-1 rounded-full bg-emerald-50 text-emerald-700 font-bold text-[10px] border border-emerald-200 inline-block">
                      {inv.status}
                    </span>
                  </td>
                  <td className="p-4 text-right">
                    <button
                      onClick={() => alert(`Tải xuống file PDF hóa đơn ${inv.id}`)}
                      className="p-2 text-slate-500 hover:text-blue-600 hover:bg-blue-50 rounded-xl transition-colors inline-flex items-center gap-1 font-semibold text-xs"
                    >
                      <Download size={15} /> PDF
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {showSupportModal && <SupportAccessModal onClose={() => setShowSupportModal(false)} />}
    </div>
  );
};
