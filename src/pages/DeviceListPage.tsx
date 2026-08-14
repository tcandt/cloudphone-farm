import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Search, CheckSquare, Square, Play, Loader2 } from 'lucide-react';
import { mockGroups } from '../data/mockData';
import { DeviceEntity } from '../types';
import { useUiStore } from '../stores/useUiStore';
import { DeviceControlModal } from '../components/devices/DeviceControlModal';
import { PermissionGuard } from '../components/common/PermissionGuard';
import { deviceService } from '../services/device-service';

export const DeviceListPage: React.FC = () => {
  const { t } = useTranslation();
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [groupFilter, setGroupFilter] = useState<string>('all');
  const [activeControlDevice, setActiveControlDevice] = useState<DeviceEntity | null>(null);
  const [devices, setDevices] = useState<DeviceEntity[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const { selectedDeviceIds, toggleSelectDevice, selectAllDevices, clearDeviceSelection } = useUiStore();

  useEffect(() => {
    let isMounted = true;

    deviceService
      .list({
        status: statusFilter,
        group_id: groupFilter,
        search: searchTerm,
      })
      .then((res) => {
        if (isMounted) {
          setDevices(res.items);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (isMounted) {
          setError(err.message || 'Failed to load devices');
          setLoading(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, [searchTerm, statusFilter, groupFilter]);

  const isAllSelected =
    devices.length > 0 && devices.every((d) => selectedDeviceIds.includes(d.device_id));

  const handleSelectAll = () => {
    if (isAllSelected) {
      clearDeviceSelection();
    } else {
      selectAllDevices(devices.map((d) => d.device_id));
    }
  };

  return (
    <div className="space-y-6">
      {/* Title Bar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('devices.title')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quản lý danh sách, gán nhóm và theo dõi trạng thái thiết bị Android
          </p>
        </div>

        {/* Selected Batch Actions Bar */}
        {selectedDeviceIds.length > 0 && (
          <div className="flex items-center gap-2 bg-blue-50 border border-blue-200 px-4 py-2 rounded-2xl shadow-sm animate-fadeIn">
            <span className="text-xs font-bold text-blue-900">Đã chọn: {selectedDeviceIds.length}</span>
            <span className="text-blue-300">|</span>
            <button
              onClick={() => alert(`Lệnh restart đã gửi tới ${selectedDeviceIds.length} thiết bị.`)}
              className="px-3 py-1 bg-white hover:bg-slate-50 text-slate-800 rounded-xl text-xs font-bold shadow-sm"
            >
              Restart
            </button>
            <button
              onClick={() => alert(`Đang áp dụng Proxy cho ${selectedDeviceIds.length} thiết bị.`)}
              className="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded-xl text-xs font-bold shadow-sm"
            >
              Gán Proxy
            </button>
          </div>
        )}
      </div>

      {/* Filter and Control Bar */}
      <div className="bg-white p-4 rounded-2xl border border-slate-200 shadow-sm flex flex-col md:flex-row gap-4 justify-between items-center">
        <div className="relative w-full md:w-80">
          <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder={t('devices.searchPlaceholder')}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all"
          />
        </div>

        <div className="flex flex-wrap items-center gap-3 w-full md:w-auto">
          {/* Status Filter */}
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-semibold text-slate-700 focus:outline-none"
          >
            <option value="all">{t('common.allStatus')}</option>
            <option value="online">Online</option>
            <option value="offline">Offline</option>
            <option value="degraded">Degraded</option>
            <option value="busy">Busy</option>
          </select>

          {/* Group Filter */}
          <select
            value={groupFilter}
            onChange={(e) => setGroupFilter(e.target.value)}
            className="px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-semibold text-slate-700 focus:outline-none"
          >
            <option value="all">{t('common.allGroups')}</option>
            {mockGroups.map((grp) => (
              <option key={grp.group_id} value={grp.group_id}>
                {grp.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Table Container */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-12 text-center text-slate-400 font-medium flex items-center justify-center gap-2">
            <Loader2 className="w-5 h-5 animate-spin text-blue-600" />
            <span>Đang tải danh sách thiết bị...</span>
          </div>
        ) : error ? (
          <div className="p-12 text-center text-rose-500 font-medium">
            Lỗi tải dữ liệu: {error}
          </div>
        ) : devices.length === 0 ? (
          <div className="p-12 text-center text-slate-400 font-medium">
            Không tìm thấy thiết bị phù hợp.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-slate-50/80 border-b border-slate-200 text-[11px] font-bold text-slate-500 uppercase tracking-wider">
                  <th className="py-3 px-4 w-10">
                    <button onClick={handleSelectAll} className="text-slate-400 hover:text-slate-600">
                      {isAllSelected ? (
                        <CheckSquare className="w-4 h-4 text-blue-600" />
                      ) : (
                        <Square className="w-4 h-4" />
                      )}
                    </button>
                  </th>
                  <th className="py-3 px-4">{t('devices.name')}</th>
                  <th className="py-3 px-4">Model & Serial</th>
                  <th className="py-3 px-4">Trạng thái</th>
                  <th className="py-3 px-4">Pin / Mạng</th>
                  <th className="py-3 px-4">Nhóm</th>
                  <th className="py-3 px-4 text-right">Thao tác</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-xs">
                {devices.map((dev) => {
                  const isSelected = selectedDeviceIds.includes(dev.device_id);
                  return (
                    <tr
                      key={dev.device_id}
                      className={`hover:bg-slate-50/80 transition-colors ${
                        isSelected ? 'bg-blue-50/40' : ''
                      }`}
                    >
                      <td className="py-3.5 px-4">
                        <button
                          onClick={() => toggleSelectDevice(dev.device_id)}
                          className="text-slate-400 hover:text-slate-600"
                        >
                          {isSelected ? (
                            <CheckSquare className="w-4 h-4 text-blue-600" />
                          ) : (
                            <Square className="w-4 h-4" />
                          )}
                        </button>
                      </td>
                      <td className="py-3.5 px-4 font-bold text-slate-900">
                        {dev.display_name || dev.name}
                      </td>
                      <td className="py-3.5 px-4 text-slate-600 font-mono text-[11px]">
                        <div>{dev.model}</div>
                        <div className="text-slate-400">{dev.serial_number}</div>
                      </td>
                      <td className="py-3.5 px-4">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase ${
                            dev.status === 'online'
                              ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                              : dev.status === 'degraded'
                              ? 'bg-amber-50 text-amber-700 border border-amber-200'
                              : 'bg-slate-100 text-slate-600 border border-slate-200'
                          }`}
                        >
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              dev.status === 'online'
                                ? 'bg-emerald-500'
                                : dev.status === 'degraded'
                                ? 'bg-amber-500'
                                : 'bg-slate-400'
                            }`}
                          />
                          {dev.status}
                        </span>
                      </td>
                      <td className="py-3.5 px-4 text-slate-600">
                        {dev.telemetry ? (
                          <div className="flex items-center gap-2">
                            <span className="font-semibold text-slate-700">{dev.telemetry.battery}%</span>
                            <span className="text-slate-300">|</span>
                            <span className="uppercase text-[10px] font-bold text-slate-500">{dev.telemetry.network}</span>
                          </div>
                        ) : (
                          <span className="text-slate-400 italic">No telemetry</span>
                        )}
                      </td>
                      <td className="py-3.5 px-4 text-slate-600 font-medium">
                        {dev.group_id || (dev.groups && dev.groups[0]) || 'Mặc định'}
                      </td>
                      <td className="py-3.5 px-4 text-right">
                        <PermissionGuard permission="device.control.input">
                          <button
                            onClick={() => setActiveControlDevice(dev)}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-bold shadow-sm transition-all text-xs"
                          >
                            <Play className="w-3.5 h-3.5" />
                            <span>Điều khiển</span>
                          </button>
                        </PermissionGuard>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Control Modal */}
      {activeControlDevice && (
        <DeviceControlModal
          device={activeControlDevice}
          onClose={() => setActiveControlDevice(null)}
        />
      )}
    </div>
  );
};
