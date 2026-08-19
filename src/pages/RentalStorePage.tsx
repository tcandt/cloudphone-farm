import React, { useState } from 'react';
import { Search, SlidersHorizontal, Package, Check, Smartphone, Cpu } from 'lucide-react';
import { mockRentalPackages } from '../data/mockData';
import { useToastStore } from '@ui/toast/Toast';

// 1. PAGE-LOCAL VIEW MODEL (No backend change)
interface ClientRentalPackage {
  id: string;
  title: string;
  description: string;
  model: string;
  android_version: string;
  ram_storage: string;
  badge?: string;
  availabilityStatus: 'Sẵn sàng' | 'Sắp hết' | 'Hết máy';
  dailyPrice: number;
  weeklyPrice: number;
  monthlyPrice: number;
}

// Map from existing mock DTO to our presentation shell model
const clientPackages: ClientRentalPackage[] = mockRentalPackages.map((pkg, index) => ({
  id: pkg.package_id,
  title: pkg.title,
  description: pkg.description,
  model: pkg.model,
  android_version: pkg.android_version,
  ram_storage: pkg.ram_storage,
  badge: pkg.badge,
  // Mock presentation data for availability
  availabilityStatus: index === 1 ? 'Sắp hết' : index === 2 ? 'Hết máy' : 'Sẵn sàng',
  dailyPrice: pkg.daily_price_usd,
  weeklyPrice: pkg.weekly_price_usd,
  monthlyPrice: pkg.monthly_price_usd,
}));

export const RentalStorePage: React.FC = () => {
  const addToast = useToastStore((state) => state.addToast);
  const [selectedDuration, setSelectedDuration] = useState<Record<string, 'daily' | 'weekly' | 'monthly'>>({});

  const handleRent = () => {
    addToast({
      type: 'info',
      title: 'Tính năng giới hạn',
      message: 'Tính năng thuê máy sẽ được kích hoạt ở giai đoạn thương mại.',
    });
  };

  return (
    <div className="space-y-8 p-4 md:p-8 max-w-7xl mx-auto">
      {/* PAGE HEADER */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-black text-slate-900 tracking-tight">Cửa hàng cho thuê</h1>
          <p className="text-sm text-slate-500 font-medium mt-1">
            Lựa chọn thiết bị Android Cloud phù hợp với nhu cầu của bạn
          </p>
        </div>
      </div>

      {/* PRIMARY TOOLBAR */}
      <div className="bg-white p-2 md:p-3 rounded-2xl border border-slate-100 shadow-sm flex flex-col md:flex-row gap-3">
        <div className="relative flex-1">
          <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none">
            <Search size={18} className="text-slate-400" />
          </div>
          <input
            type="text"
            placeholder="Tìm kiếm thiết bị, model..."
            className="w-full pl-10 pr-4 py-2.5 bg-slate-50 border-transparent rounded-xl text-sm focus:bg-white focus:border-emerald-500 focus:ring-2 focus:ring-emerald-200 transition-all outline-none"
          />
        </div>
        <div className="flex gap-3">
          <button className="flex items-center justify-center gap-2 px-4 py-2.5 bg-slate-50 hover:bg-slate-100 text-slate-700 rounded-xl text-sm font-semibold transition-colors border border-transparent">
            <Package size={18} /> Loại thiết bị
          </button>
          <button className="flex items-center justify-center gap-2 px-4 py-2.5 bg-slate-50 hover:bg-slate-100 text-slate-700 rounded-xl text-sm font-semibold transition-colors border border-transparent">
            <SlidersHorizontal size={18} /> Lọc trạng thái
          </button>
        </div>
      </div>

      {/* RENTAL PRODUCT CARDS GRID */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 md:gap-8">
        {clientPackages.map((pkg) => {
          const duration = selectedDuration[pkg.id] || 'daily';

          const getPrice = () => {
            if (duration === 'weekly') return pkg.weeklyPrice;
            if (duration === 'monthly') return pkg.monthlyPrice;
            return pkg.dailyPrice;
          };

          return (
            <div
              key={pkg.id}
              className="bg-white border border-slate-200/60 shadow-sm hover:shadow-xl hover:-translate-y-1 transition-all duration-300 rounded-[24px] p-6 flex flex-col justify-between group"
            >
              <div className="space-y-6">
                {/* Header & Badges */}
                <div className="flex items-start justify-between">
                  <div className="p-3 bg-emerald-50 text-emerald-600 rounded-2xl group-hover:bg-emerald-100 group-hover:text-emerald-700 transition-colors">
                    <Smartphone size={24} />
                  </div>
                  <div className="flex flex-col items-end gap-2">
                    {pkg.badge && (
                      <span className="px-3 py-1 rounded-full bg-slate-900 text-white text-[11px] font-bold uppercase tracking-wider shadow-sm">
                        {pkg.badge}
                      </span>
                    )}
                    <span className={`px-2.5 py-1 rounded-full text-[11px] font-bold flex items-center gap-1 border ${
                      pkg.availabilityStatus === 'Sẵn sàng' ? 'bg-emerald-50 text-emerald-700 border-emerald-200/50' : 
                      pkg.availabilityStatus === 'Sắp hết' ? 'bg-amber-50 text-amber-700 border-amber-200/50' : 
                      'bg-rose-50 text-rose-700 border-rose-200/50'
                    }`}>
                      {pkg.availabilityStatus === 'Sẵn sàng' && <Check size={12} />}
                      {pkg.availabilityStatus}
                    </span>
                  </div>
                </div>

                {/* Title */}
                <div>
                  <h2 className="text-xl font-extrabold text-slate-900">{pkg.title}</h2>
                  <p className="text-sm text-slate-500 mt-2 leading-relaxed">{pkg.description}</p>
                </div>

                {/* Specs */}
                <div className="grid grid-cols-2 gap-3">
                  <div className="bg-slate-50 rounded-xl p-3 border border-slate-100/50">
                    <div className="text-[10px] uppercase font-bold text-slate-400 mb-1 flex items-center gap-1">
                      <Smartphone size={12} /> Model
                    </div>
                    <div className="text-sm font-semibold text-slate-700">{pkg.model}</div>
                  </div>
                  <div className="bg-slate-50 rounded-xl p-3 border border-slate-100/50">
                    <div className="text-[10px] uppercase font-bold text-slate-400 mb-1 flex items-center gap-1">
                      <Cpu size={12} /> Cấu hình
                    </div>
                    <div className="text-sm font-semibold text-slate-700">{pkg.ram_storage}</div>
                  </div>
                </div>

                {/* Duration Picker */}
                <div className="space-y-2">
                  <span className="text-[11px] font-bold text-slate-500 uppercase tracking-wider">
                    Thời hạn thuê
                  </span>
                  <div className="grid grid-cols-3 gap-2 bg-slate-50 p-1.5 rounded-2xl border border-slate-100/50">
                    {(['daily', 'weekly', 'monthly'] as const).map((d) => (
                      <button
                        key={d}
                        onClick={() => setSelectedDuration({ ...selectedDuration, [pkg.id]: d })}
                        className={`py-2 rounded-xl text-xs font-bold transition-all ${
                          duration === d 
                            ? 'bg-white text-emerald-700 shadow-sm border border-emerald-100/50' 
                            : 'text-slate-500 hover:bg-slate-200/50 hover:text-slate-700 border border-transparent'
                        }`}
                      >
                        {d === 'daily' ? 'Ngày' : d === 'weekly' ? 'Tuần' : 'Tháng'}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Action Area */}
              <div className="mt-8 pt-6 border-t border-slate-100">
                <div className="flex items-end justify-between mb-4">
                  <div className="text-[11px] font-bold text-slate-400 uppercase tracking-wider">Chi phí</div>
                  <div className="text-3xl font-black text-emerald-600 tracking-tight">
                    ${getPrice().toFixed(2)}
                    <span className="text-sm font-semibold text-slate-400 tracking-normal ml-1">
                      /{duration === 'daily' ? 'ngày' : duration === 'weekly' ? 'tuần' : 'tháng'}
                    </span>
                  </div>
                </div>

                <button
                  onClick={handleRent}
                  disabled={pkg.availabilityStatus === 'Hết máy'}
                  className="w-full py-3.5 bg-slate-900 text-white font-bold text-sm rounded-xl hover:bg-slate-800 disabled:bg-slate-200 disabled:text-slate-400 transition-colors shadow-sm active:scale-[0.99]"
                >
                  {pkg.availabilityStatus === 'Hết máy' ? 'Tạm hết' : 'Thuê ngay'}
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
