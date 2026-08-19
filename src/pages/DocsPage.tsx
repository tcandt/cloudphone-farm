import React, { useState } from 'react';
import { Search, Book, Smartphone, MonitorPlay, Zap, Code, Download, ArrowRight } from 'lucide-react';
import { useToastStore } from '@ui/toast/Toast';

const DOC_CATEGORIES = [
  {
    id: 'getting-started',
    title: 'Getting Started',
    description: 'Hướng dẫn cơ bản, cách tạo tài khoản và thuê thiết bị đầu tiên.',
    icon: Book,
    color: 'emerald'
  },
  {
    id: 'device-management',
    title: 'Device Management',
    description: 'Quản lý thiết bị, gán nhóm, kiểm tra trạng thái và theo dõi tài nguyên.',
    icon: Smartphone,
    color: 'blue'
  },
  {
    id: 'remote-control',
    title: 'Remote Control',
    description: 'Sử dụng giao diện điều khiển thiết bị (touch, swipe, gõ phím).',
    icon: MonitorPlay,
    color: 'indigo'
  },
  {
    id: 'automation',
    title: 'Automation',
    description: 'Tự động hóa thao tác (macros, kịch bản) trên nhiều thiết bị cùng lúc.',
    icon: Zap,
    color: 'amber'
  },
  {
    id: 'api',
    title: 'API / Integration',
    description: 'Tài liệu tích hợp hệ thống qua RESTful API dành cho Developers.',
    icon: Code,
    color: 'slate'
  },
];

export const DocsPage: React.FC = () => {
  const [searchTerm, setSearchTerm] = useState('');
  const addToast = useToastStore((state) => state.addToast);

  const handleDocClick = () => {
    addToast({
      type: 'info',
      title: 'Tài liệu chi tiết',
      message: 'Chi tiết tài liệu sẽ được cập nhật trong phiên bản tiếp theo.',
    });
  };

  return (
    <div className="space-y-8 p-4 md:p-8 max-w-5xl mx-auto">
      {/* PAGE HERO */}
      <div className="bg-slate-900 rounded-[32px] p-8 md:p-12 text-center relative overflow-hidden shadow-xl shadow-slate-900/10">
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[400px] bg-emerald-500/20 blur-[100px] pointer-events-none rounded-full" />
        <div className="relative z-10 max-w-2xl mx-auto space-y-6">
          <h1 className="text-3xl md:text-5xl font-black text-white tracking-tight">Tài liệu hướng dẫn</h1>
          <p className="text-slate-400 font-medium text-sm md:text-base">
            Tất cả những gì bạn cần biết để sử dụng, quản lý và tích hợp Cloud Phone.
          </p>
          
          <div className="relative max-w-lg mx-auto mt-8">
            <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
              <Search size={20} className="text-slate-400" />
            </div>
            <input
              type="text"
              placeholder="Tìm kiếm tài liệu, API, hướng dẫn..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-12 pr-4 py-4 bg-slate-800 border border-slate-700 rounded-2xl text-white placeholder-slate-400 focus:bg-slate-800 focus:outline-none focus:border-emerald-500 transition-all shadow-inner"
            />
          </div>
        </div>
      </div>

      {/* CATEGORIES GRID */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {DOC_CATEGORIES.map((cat) => {
          const Icon = cat.icon;
          return (
            <div 
              key={cat.id} 
              onClick={handleDocClick}
              className="bg-white border border-slate-200/60 rounded-[24px] p-6 shadow-sm hover:shadow-xl hover:-translate-y-1 transition-all duration-300 group cursor-pointer"
            >
              <div className={`w-12 h-12 rounded-2xl flex items-center justify-center mb-5 ${
                cat.color === 'emerald' ? 'bg-emerald-50 text-emerald-600' :
                cat.color === 'blue' ? 'bg-blue-50 text-blue-600' :
                cat.color === 'indigo' ? 'bg-indigo-50 text-indigo-600' :
                cat.color === 'amber' ? 'bg-amber-50 text-amber-600' :
                'bg-slate-100 text-slate-700'
              }`}>
                <Icon size={24} />
              </div>
              <h3 className="text-lg font-bold text-slate-900 group-hover:text-emerald-600 transition-colors">{cat.title}</h3>
              <p className="text-sm text-slate-500 mt-2 leading-relaxed h-16">{cat.description}</p>
              
              <div className="mt-4 pt-4 border-t border-slate-100 flex items-center text-sm font-bold text-slate-400 group-hover:text-emerald-600 transition-colors">
                Xem chi tiết <ArrowRight size={16} className="ml-1" />
              </div>
            </div>
          );
        })}

        {/* ANDROID AGENT SPECIAL CARD */}
        <div className="bg-slate-50 border border-slate-200/60 rounded-[24px] p-6 shadow-sm flex flex-col justify-between">
          <div>
            <div className="w-12 h-12 rounded-2xl flex items-center justify-center mb-5 bg-slate-200 text-slate-700">
              <Download size={24} />
            </div>
            <h3 className="text-lg font-bold text-slate-900">Android Agent</h3>
            <p className="text-sm text-slate-500 mt-2 leading-relaxed">
              Tải và cài đặt Agent client lên thiết bị Android vật lý để kết nối vào farm.
            </p>
          </div>
          
          <div className="mt-6 pt-4 border-t border-slate-200/60">
            <div className="w-full py-2.5 bg-slate-100 border border-slate-200 text-slate-500 font-bold text-sm rounded-xl text-center flex flex-col items-center justify-center gap-0.5">
              <span>Sắp có</span>
              <span className="text-[10px] font-medium text-slate-400">Sẽ khả dụng trong giai đoạn Agent</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
