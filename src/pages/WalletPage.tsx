import React, { useState } from 'react';
import { Wallet, ArrowUpRight, ArrowDownLeft, Clock, CheckCircle2, ChevronRight, CreditCard, Banknote } from 'lucide-react';
import { useToastStore } from '@ui/toast/Toast';

const MOCK_AMOUNTS = [10, 20, 50, 100, 200, 500];

const MOCK_TRANSACTIONS = [
  { id: 'TX-98234', type: 'deposit', amount: 100, date: '2023-11-15T10:30:00Z', status: 'success' },
  { id: 'TX-98233', type: 'rental', amount: -15, date: '2023-11-12T08:15:00Z', status: 'success', description: 'Thuê Cloud Phone (Gói 3 ngày)' },
  { id: 'TX-98232', type: 'rental', amount: -50, date: '2023-11-05T14:20:00Z', status: 'success', description: 'Thuê Cloud Phone (Gói 10 ngày)' },
  { id: 'TX-98231', type: 'deposit', amount: 50, date: '2023-11-01T09:00:00Z', status: 'success' },
];

export const WalletPage: React.FC = () => {
  const [selectedAmount, setSelectedAmount] = useState<number>(50);
  const addToast = useToastStore((state) => state.addToast);

  const handleDeposit = () => {
    addToast({
      type: 'info',
      title: 'Chức năng nạp tiền',
      message: 'Cổng thanh toán sẽ được tích hợp trong giai đoạn thương mại.',
    });
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('vi-VN', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  return (
    <div className="space-y-6 md:space-y-8 p-4 md:p-8 max-w-5xl mx-auto">
      {/* PAGE HEADER */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-black text-slate-900 tracking-tight">Ví điện tử</h1>
          <p className="text-sm text-slate-500 font-medium mt-1">
            Quản lý số dư và lịch sử giao dịch
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 md:gap-8">
        
        {/* LEFT COLUMN: Balance & Deposit */}
        <div className="lg:col-span-1 space-y-6">
          {/* BALANCE CARD */}
          <div className="bg-slate-900 rounded-[24px] p-6 text-white shadow-xl shadow-slate-900/20 relative overflow-hidden">
            <div className="absolute -right-6 -top-6 w-32 h-32 bg-white/10 rounded-full blur-2xl pointer-events-none" />
            <div className="flex items-center gap-3 mb-8">
              <div className="p-2.5 bg-white/10 rounded-xl">
                <Wallet size={24} className="text-emerald-400" />
              </div>
              <span className="font-semibold text-slate-300">Số dư khả dụng</span>
            </div>
            <div>
              <div className="text-4xl font-black tracking-tight mb-1">$85.00</div>
              <div className="text-sm text-slate-400 font-medium">≈ 2,150,000 VND</div>
            </div>
          </div>

          {/* DEPOSIT ACTION SHELL */}
          <div className="bg-white border border-slate-200/60 rounded-[24px] p-6 shadow-sm space-y-6">
            <div>
              <h2 className="text-lg font-extrabold text-slate-900 flex items-center gap-2">
                <Banknote size={20} className="text-emerald-600" /> Nạp tiền nhanh
              </h2>
              <p className="text-xs text-slate-500 mt-1">Chọn số tiền (USD) để nạp vào ví</p>
            </div>

            <div className="grid grid-cols-3 gap-3">
              {MOCK_AMOUNTS.map((amt) => (
                <button
                  key={amt}
                  onClick={() => setSelectedAmount(amt)}
                  className={`py-3 rounded-xl text-sm font-bold transition-all border ${
                    selectedAmount === amt
                      ? 'bg-emerald-50 text-emerald-700 border-emerald-200 shadow-sm'
                      : 'bg-slate-50 text-slate-600 border-transparent hover:bg-slate-100'
                  }`}
                >
                  ${amt}
                </button>
              ))}
            </div>

            <div className="pt-2">
              <button 
                onClick={handleDeposit}
                className="w-full flex items-center justify-center gap-2 py-3.5 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-bold shadow-sm active:scale-[0.99] transition-all"
              >
                <CreditCard size={18} /> Tiến hành thanh toán ${selectedAmount}
              </button>
            </div>
          </div>
        </div>

        {/* RIGHT COLUMN: Transaction History */}
        <div className="lg:col-span-2">
          <div className="bg-white border border-slate-200/60 rounded-[24px] p-6 shadow-sm h-full">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-extrabold text-slate-900 flex items-center gap-2">
                <Clock size={20} className="text-slate-400" /> Lịch sử giao dịch
              </h2>
              <button className="text-xs font-bold text-emerald-600 hover:text-emerald-700 flex items-center">
                Xem tất cả <ChevronRight size={14} />
              </button>
            </div>

            <div className="space-y-4">
              {MOCK_TRANSACTIONS.map((tx) => (
                <div key={tx.id} className="flex items-center justify-between p-4 rounded-2xl bg-slate-50 hover:bg-slate-100/50 transition-colors border border-slate-100/50">
                  <div className="flex items-center gap-4">
                    <div className={`p-3 rounded-full ${
                      tx.type === 'deposit' ? 'bg-emerald-100 text-emerald-600' : 'bg-slate-200 text-slate-600'
                    }`}>
                      {tx.type === 'deposit' ? <ArrowDownLeft size={20} /> : <ArrowUpRight size={20} />}
                    </div>
                    <div>
                      <div className="font-bold text-slate-900 text-sm">
                        {tx.type === 'deposit' ? 'Nạp tiền vào ví' : tx.description}
                      </div>
                      <div className="flex items-center gap-2 mt-1">
                        <span className="text-[11px] text-slate-500">{formatDate(tx.date)}</span>
                        <span className="text-slate-300">•</span>
                        <span className="text-[11px] text-emerald-600 font-bold flex items-center gap-0.5">
                          <CheckCircle2 size={12} /> Thành công
                        </span>
                      </div>
                    </div>
                  </div>
                  <div className={`text-base font-black ${
                    tx.amount > 0 ? 'text-emerald-600' : 'text-slate-900'
                  }`}>
                    {tx.amount > 0 ? '+' : ''}{tx.amount}.00 USD
                  </div>
                </div>
              ))}
            </div>
            
            <div className="mt-6 pt-6 border-t border-slate-100 text-center">
              <p className="text-xs text-slate-400 font-medium">Hiển thị 4 giao dịch gần nhất</p>
            </div>
          </div>
        </div>

      </div>
    </div>
  );
};
