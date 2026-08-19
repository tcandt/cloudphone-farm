import React, { useState, useEffect, useMemo } from 'react';
import { Search, MonitorPlay, Smartphone, Clock, Filter, AlertCircle, RefreshCw } from 'lucide-react';
import { DeviceEntity } from '../types';
import { deviceService } from '../services/device-service';
import { useToastStore } from '@ui/toast/Toast';

// 1. PAGE-LOCAL VIEW MODEL
interface ClientDeviceViewModel {
  id: string;
  displayName: string;
  model: string;
  androidVersion: string;
  group: string;
  presentationStatus: 'Online' | 'Offline';
  remainingTime: string;
  rawDevice: DeviceEntity;
}

const mapToClientViewModel = (devices: DeviceEntity[]): ClientDeviceViewModel[] => {
  return devices.map((dev) => {
    // Only map real device statuses. Do not invent missing_permission or expiring.
    const presentationStatus = dev.status === 'online' ? 'Online' : 'Offline';
    
    // Use neutral placeholder since rental backend is not present
    const remainingTime = '—';

    return {
      id: dev.device_id,
      displayName: dev.display_name || dev.name || 'Cloud Phone',
      model: dev.model,
      androidVersion: `Android ${dev.android_version}`,
      group: dev.group_id || 'Mặc định',
      presentationStatus,
      remainingTime,
      rawDevice: dev,
    };
  });
};

export const DeviceListPage: React.FC = () => {
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('Tất cả');
  const [rawDevices, setRawDevices] = useState<DeviceEntity[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryVersion, setRetryVersion] = useState(0);
  
  const addToast = useToastStore((state) => state.addToast);

  useEffect(() => {
    let isMounted = true;
    
    // We only fetch based on search term. We DO NOT send fake presentation statuses to the API.
    deviceService
      .list({ search: searchTerm })
      .then((res) => {
        if (isMounted) {
          setRawDevices(res.items);
          setLoading(false);
        }
      })
      .catch(() => {
        if (isMounted) {
          setError('Không thể tải danh sách thiết bị lúc này.');
          setLoading(false);
        }
      });

    return () => { isMounted = false; };
  }, [searchTerm, retryVersion]);

  const viewModels = useMemo(() => {
    const allModels = mapToClientViewModel(rawDevices);
    // Note: statusFilter might contain values like 'Thiếu quyền' that won't match any real devices yet
    if (statusFilter === 'Tất cả') return allModels;
    return allModels.filter(m => m.presentationStatus === statusFilter);
  }, [rawDevices, statusFilter]);

  const handleControlClick = () => {
    addToast({
      type: 'info',
      title: 'Tính năng giới hạn',
      message: 'Tính năng điều khiển sẽ được kích hoạt ở giai đoạn thương mại.',
    });
  };

  return (
    <div className="space-y-6 md:space-y-8 p-4 md:p-8 max-w-7xl mx-auto">
      {/* PAGE HEADER */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-black text-slate-900 tracking-tight">Quản lý thiết bị</h1>
          <p className="text-sm text-slate-500 font-medium mt-1">
            Tổng cộng {rawDevices.length} thiết bị đang thuê
          </p>
        </div>
        
        <div className="flex items-center gap-3">
          <button className="hidden md:flex items-center gap-2 px-4 py-2.5 bg-white border border-slate-200 text-slate-600 rounded-xl text-sm font-semibold hover:bg-slate-50 transition-colors shadow-sm cursor-not-allowed opacity-70" disabled>
            Wall View
          </button>
          <button className="flex items-center gap-2 px-4 py-2.5 bg-emerald-50 text-emerald-700 rounded-xl text-sm font-semibold hover:bg-emerald-100 transition-colors cursor-not-allowed opacity-70 border border-emerald-200/50" disabled>
            Điều khiển đồng bộ
          </button>
        </div>
      </div>

      {/* FILTER TOOLBAR */}
      <div className="bg-white p-2 md:p-3 rounded-2xl border border-slate-100 shadow-sm flex flex-col md:flex-row gap-3">
        <div className="relative flex-1">
          <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none">
            <Search size={18} className="text-slate-400" />
          </div>
          <input
            type="text"
            placeholder="Tìm kiếm tên thiết bị..."
            value={searchTerm}
            onChange={(e) => {
              setSearchTerm(e.target.value);
              setLoading(true);
              setError(null);
            }}
            className="w-full pl-10 pr-4 py-2.5 bg-slate-50 border-transparent rounded-xl text-sm focus:bg-white focus:border-emerald-500 focus:ring-2 focus:ring-emerald-200 transition-all outline-none"
          />
        </div>
        
        <div className="flex overflow-x-auto custom-scrollbar gap-2 pb-1 md:pb-0">
          {['Tất cả', 'Online', 'Offline', 'Thiếu quyền', 'Sắp hết hạn'].map((status) => (
            <button
              key={status}
              onClick={() => setStatusFilter(status)}
              className={`flex-shrink-0 px-4 py-2.5 rounded-xl text-sm font-semibold transition-all ${
                statusFilter === status
                  ? 'bg-slate-900 text-white shadow-sm'
                  : 'bg-slate-50 text-slate-600 hover:bg-slate-100 border border-transparent'
              }`}
            >
              {status}
            </button>
          ))}
          <button className="flex-shrink-0 flex items-center gap-2 px-4 py-2.5 bg-slate-50 text-slate-600 rounded-xl text-sm font-semibold hover:bg-slate-100 transition-colors border border-transparent">
            <Filter size={16} /> Nhóm
          </button>
        </div>
      </div>

      {/* DEVICE LIST */}
      {loading ? (
        <div className="flex flex-col items-center justify-center p-16 text-slate-400 space-y-4">
          <RefreshCw className="w-8 h-8 animate-spin text-emerald-500" />
          <p className="font-medium text-sm">Đang tải danh sách thiết bị...</p>
        </div>
      ) : error ? (
        <div className="bg-white border border-rose-100 rounded-3xl p-16 text-center shadow-sm">
          <div className="w-16 h-16 bg-rose-50 rounded-full flex items-center justify-center mx-auto mb-4 text-rose-400">
            <AlertCircle size={32} />
          </div>
          <h3 className="text-lg font-bold text-slate-900">Không thể tải dữ liệu</h3>
          <p className="text-sm text-slate-500 mt-1">{error}</p>
          <button 
            onClick={() => {
              setLoading(true);
              setError(null);
              setRetryVersion(v => v + 1);
            }}
            className="mt-6 px-6 py-2 bg-white border border-slate-200 text-slate-700 font-bold rounded-xl shadow-sm hover:bg-slate-50 transition-colors"
          >
            Thử lại
          </button>
        </div>
      ) : viewModels.length === 0 ? (
        <div className="bg-white border border-slate-100 rounded-3xl p-16 text-center shadow-sm">
          <div className="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mx-auto mb-4 text-slate-400">
            <Smartphone size={32} />
          </div>
          <h3 className="text-lg font-bold text-slate-900">Không tìm thấy thiết bị</h3>
          <p className="text-sm text-slate-500 mt-1">Không có thiết bị nào phù hợp với bộ lọc hiện tại.</p>
        </div>
      ) : (
        <>
          {/* DESKTOP GRID VIEW (Hidden on Mobile) */}
          <div className="hidden md:grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-6">
            {viewModels.map((vm) => (
              <div key={vm.id} className="bg-white border border-slate-200/60 rounded-[20px] p-5 shadow-sm hover:shadow-md transition-shadow flex flex-col justify-between group">
                <div className="flex items-start justify-between mb-4">
                  <div className="flex items-center gap-3">
                    <div className={`p-2.5 rounded-xl ${
                      vm.presentationStatus === 'Online' ? 'bg-emerald-50 text-emerald-600' : 'bg-slate-100 text-slate-400'
                    }`}>
                      <Smartphone size={20} />
                    </div>
                    <div>
                      <h3 className="font-extrabold text-slate-900 text-base">{vm.displayName}</h3>
                      <p className="text-xs text-slate-500 font-medium">{vm.model}</p>
                    </div>
                  </div>
                  
                  <span className={`px-2.5 py-1 rounded-full flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wide border ${
                    vm.presentationStatus === 'Online' ? 'bg-emerald-50 text-emerald-700 border-emerald-200/50' : 'bg-slate-50 text-slate-500 border-slate-200'
                  }`}>
                    <span className={`w-1.5 h-1.5 rounded-full ${
                      vm.presentationStatus === 'Online' ? 'bg-emerald-500' : 'bg-slate-400'
                    }`} />
                    {vm.presentationStatus}
                  </span>
                </div>
                
                <div className="grid grid-cols-2 gap-2 mb-5">
                  <div className="bg-slate-50 p-2.5 rounded-xl border border-slate-100/50">
                    <div className="text-[10px] uppercase font-bold text-slate-400 mb-0.5">Hệ điều hành</div>
                    <div className="text-xs font-semibold text-slate-700">{vm.androidVersion}</div>
                  </div>
                  <div className="bg-slate-50 p-2.5 rounded-xl border border-slate-100/50">
                    <div className="text-[10px] uppercase font-bold text-slate-400 mb-0.5 flex items-center gap-1">
                      <Clock size={10} /> Hết hạn
                    </div>
                    <div className="text-xs font-semibold text-slate-700">{vm.remainingTime}</div>
                  </div>
                </div>
                
                <button 
                  onClick={handleControlClick}
                  className="w-full flex items-center justify-center gap-2 py-3 bg-slate-900 hover:bg-slate-800 text-white rounded-xl text-sm font-bold transition-all shadow-sm active:scale-[0.99]"
                >
                  <MonitorPlay size={16} /> Mở điều khiển
                </button>
              </div>
            ))}
          </div>

          {/* MOBILE STACKED LIST VIEW (Hidden on Desktop) */}
          <div className="md:hidden space-y-4">
            {viewModels.map((vm) => (
              <div key={vm.id} className="bg-white border border-slate-200/60 rounded-[20px] p-4 shadow-sm space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`p-2 rounded-xl ${
                      vm.presentationStatus === 'Online' ? 'bg-emerald-50 text-emerald-600' : 'bg-slate-100 text-slate-500'
                    }`}>
                      <Smartphone size={20} />
                    </div>
                    <div>
                      <h3 className="font-extrabold text-slate-900 text-sm">{vm.displayName}</h3>
                      <p className="text-[11px] text-slate-500">{vm.model}</p>
                    </div>
                  </div>
                  <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold flex items-center gap-1 ${
                    vm.presentationStatus === 'Online' ? 'text-emerald-700 bg-emerald-50' : 'text-slate-500 bg-slate-50'
                  }`}>
                    {vm.presentationStatus}
                  </span>
                </div>
                
                <div className="flex items-center justify-between text-[11px] text-slate-500 font-medium px-1">
                  <span>{vm.androidVersion}</span>
                  <div className="flex items-center gap-1">
                    <Clock size={12} /> 
                    <span>
                      {vm.remainingTime}
                    </span>
                  </div>
                </div>

                <button 
                  onClick={handleControlClick}
                  className="w-full flex items-center justify-center gap-2 py-2.5 bg-slate-900 text-white rounded-xl text-sm font-bold shadow-sm"
                >
                  <MonitorPlay size={16} /> Điều khiển
                </button>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
};
