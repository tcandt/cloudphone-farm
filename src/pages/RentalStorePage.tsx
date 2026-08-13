import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ShoppingBag, Server, Check, ShieldAlert, Sparkles, Cpu } from 'lucide-react';
import { mockRentalPackages, mockCurrentUserSession } from '../data/mockData';
import { useUiStore } from '../stores/useUiStore';

export const RentalStorePage: React.FC = () => {
  const { t } = useTranslation();
  const { featureRentalStore } = useUiStore();
  const [selectedDuration, setSelectedDuration] = useState<Record<string, 'daily' | 'weekly' | 'monthly'>>({});

  if (!featureRentalStore) {
    return (
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-8 text-center space-y-4 max-w-lg mx-auto my-12">
        <div className="w-16 h-16 mx-auto rounded-3xl bg-amber-50 text-amber-600 flex items-center justify-center">
          <ShieldAlert size={32} />
        </div>
        <h1 className="text-lg font-extrabold text-slate-900">Rental Store Chưa Được Bật</h1>
        <p className="text-xs text-slate-500 leading-relaxed">
          Tính năng Rental Store hiện đang được ẩn dưới Feature Flag <code>VITE_FEATURE_RENTAL_STORE=false</code>. Bạn có thể bật tính năng này trong phần Cài đặt (Settings).
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Title & Preview Note */}
      <div>
        <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-amber-50 border border-amber-200 text-amber-800 text-[11px] font-bold mb-2">
          <Sparkles size={12} /> {t('rental.featureFlagNote')}
        </div>
        <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('rental.title')}</h1>
        <p className="text-xs text-slate-500 font-medium">
          Thuê và mở rộng cụm nút phần cứng Android Cloud Node tiêu chuẩn cho doanh nghiệp
        </p>
      </div>

      {/* Rental Package Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {mockRentalPackages.map((pkg) => {
          const duration = selectedDuration[pkg.package_id] || 'daily';

          const getPrice = () => {
            if (duration === 'weekly') return pkg.weekly_price_usd;
            if (duration === 'monthly') return pkg.monthly_price_usd;
            return pkg.daily_price_usd;
          };

          return (
            <div
              key={pkg.package_id}
              className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-5 hover:shadow-xl transition-all flex flex-col justify-between"
            >
              <div className="space-y-4">
                {/* Header Badge */}
                <div className="flex items-center justify-between">
                  <span className="p-2.5 bg-blue-50 text-blue-600 rounded-2xl">
                    <Server size={20} />
                  </span>
                  {pkg.badge && (
                    <span className="px-2.5 py-1 rounded-full bg-amber-100 text-amber-800 text-[10px] font-extrabold uppercase">
                      {pkg.badge}
                    </span>
                  )}
                </div>

                <div>
                  <h2 className="text-base font-extrabold text-slate-900">{pkg.title}</h2>
                  <p className="text-xs text-slate-500 mt-1 leading-relaxed">{pkg.description}</p>
                </div>

                {/* Specs */}
                <div className="p-3 bg-slate-50 rounded-2xl space-y-1 text-xs font-semibold text-slate-700">
                  <div className="flex justify-between">
                    <span className="text-slate-400">Model:</span> <span>{pkg.model}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-400">OS:</span> <span>{pkg.android_version}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-400">RAM / ROM:</span> <span>{pkg.ram_storage}</span>
                  </div>
                </div>

                {/* Duration Pills */}
                <div className="space-y-1.5">
                  <span className="text-[10px] font-extrabold text-slate-400 uppercase tracking-wider block">
                    Thời hạn thuê:
                  </span>
                  <div className="grid grid-cols-3 gap-1.5 bg-slate-100 p-1 rounded-xl">
                    {(['daily', 'weekly', 'monthly'] as const).map((d) => (
                      <button
                        key={d}
                        onClick={() => setSelectedDuration({ ...selectedDuration, [pkg.package_id]: d })}
                        className={`py-1.5 rounded-lg text-xs font-bold transition-all ${
                          duration === d ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600 hover:bg-slate-200/60'
                        }`}
                      >
                        {d === 'daily' ? 'Ngày' : d === 'weekly' ? 'Tuần' : 'Tháng'}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Price & Action */}
              <div className="pt-4 border-t border-slate-100 space-y-3">
                <div className="flex items-baseline justify-between">
                  <span className="text-xs font-bold text-slate-400">Chi phí:</span>
                  <span className="text-2xl font-black text-slate-900">
                    ${getPrice().toFixed(2)}{' '}
                    <span className="text-xs font-normal text-slate-400">
                      /{duration === 'daily' ? 'ngày' : duration === 'weekly' ? 'tuần' : 'tháng'}
                    </span>
                  </span>
                </div>

                <button
                  onClick={() =>
                    alert(
                      `Xác nhận thuê ${pkg.title} (${duration}) với giá $${getPrice().toFixed(
                        2
                      )}? Số dư khả dụng: $${mockCurrentUserSession.balance_usd.toFixed(2)}`
                    )
                  }
                  className="w-full py-3 bg-gradient-to-r from-blue-600 to-indigo-600 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 hover:opacity-95 transition-all active:scale-[0.99]"
                >
                  {t('rental.rentNow')}
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
