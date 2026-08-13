import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Smartphone,
  Search,
  Filter,
  CheckSquare,
  Square,
  Play,
  RotateCcw,
  SlidersHorizontal,
  Plus,
  ShieldCheck,
  Zap,
  MoreVertical,
  Activity,
  Layers,
} from 'lucide-react';
import { mockDevices, mockGroups } from '../data/mockData';
import { DeviceEntity } from '../types';
import { useUiStore } from '../stores/useUiStore';
import { DeviceControlModal } from '../components/devices/DeviceControlModal';
import { PermissionGuard } from '../components/common/PermissionGuard';

export const DeviceListPage: React.FC = () => {
  const { t } = useTranslation();
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [groupFilter, setGroupFilter] = useState<string>('all');
  const [activeControlDevice, setActiveControlDevice] = useState<DeviceEntity | null>(null);

  const { selectedDeviceIds, toggleSelectDevice, selectAllDevices, clearDeviceSelection } = useUiStore();

  const filteredDevices = mockDevices.filter((dev) => {
    const matchesSearch =
      dev.display_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      dev.model.toLowerCase().includes(searchTerm.toLowerCase()) ||
      dev.serial_number.toLowerCase().includes(searchTerm.toLowerCase());

    const matchesStatus = statusFilter === 'all' || dev.status === statusFilter;
    const matchesGroup = groupFilter === 'all' || dev.groups.includes(groupFilter);

    return matchesSearch && matchesStatus && matchesGroup;
  });

  const isAllSelected =
    filteredDevices.length > 0 && filteredDevices.every((d) => selectedDeviceIds.includes(d.device_id));

  const handleSelectAll = () => {
    if (isAllSelected) {
      clearDeviceSelection();
    } else {
      selectAllDevices(filteredDevices.map((d) => d.device_id));
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
              onClick={clearDeviceSelection}
              className="text-xs font-semibold text-rose-600 hover:underline ml-2"
            >
              Bỏ chọn
            </button>
          </div>
        )}
      </div>

      {/* Filter Toolbar */}
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-2xl p-4 flex flex-col md:flex-row items-center justify-between gap-4">
        {/* Search */}
        <div className="relative w-full md:w-80">
          <Search size={16} className="absolute left-3.5 top-3 text-slate-400" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder={t('devices.searchPlaceholder')}
            className="w-full pl-10 pr-4 py-2.5 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium focus:ring-2 focus:ring-blue-500 outline-none"
          />
        </div>

        {/* Filters */}
        <div className="flex items-center gap-3 w-full md:w-auto flex-wrap">
          {/* Status Filter */}
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="px-3 py-2.5 bg-slate-50 border border-slate-200 rounded-xl text-xs font-semibold text-slate-700 outline-none"
          >
            <option value="all">{t('devices.statusAll')}</option>
            <option value="online">Online</option>
            <option value="degraded">Degraded</option>
            <option value="offline">Offline</option>
          </select>

          {/* Group Filter */}
          <select
            value={groupFilter}
            onChange={(e) => setGroupFilter(e.target.value)}
            className="px-3 py-2.5 bg-slate-50 border border-slate-200 rounded-xl text-xs font-semibold text-slate-700 outline-none"
          >
            <option value="all">{t('devices.groupAll')}</option>
            {mockGroups.map((g) => (
              <option key={g.group_id} value={g.group_id}>
                {g.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Device Data Table */}
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50/70 border-b border-slate-100 text-[11px] font-extrabold uppercase text-slate-400 tracking-wider">
                <th className="p-4 w-10">
                  <button onClick={handleSelectAll} className="text-slate-400 hover:text-slate-700">
                    {isAllSelected ? <CheckSquare size={18} className="text-blue-600" /> : <Square size={18} />}
                  </button>
                </th>
                <th className="p-4">Thiết bị</th>
                <th className="p-4">Trạng thái</th>
                <th className="p-4">Model / Android</th>
                <th className="p-4">Pin / Mạng</th>
                <th className="p-4">Nhóm</th>
                <th className="p-4 text-right">Thao tác</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {filteredDevices.map((dev) => {
                const isSelected = selectedDeviceIds.includes(dev.device_id);

                return (
                  <tr key={dev.device_id} className={`hover:bg-slate-50/80 transition-colors ${isSelected ? 'bg-blue-50/30' : ''}`}>
                    <td className="p-4">
                      <button onClick={() => toggleSelectDevice(dev.device_id)} className="text-slate-400 hover:text-slate-700">
                        {isSelected ? <CheckSquare size={18} className="text-blue-600" /> : <Square size={18} />}
                      </button>
                    </td>

                    <td className="p-4">
                      <div className="flex items-center gap-3">
                        <div className="p-2 bg-slate-100 text-slate-700 rounded-xl font-black text-xs">
                          {dev.display_name.substring(0, 2).toUpperCase()}
                        </div>
                        <div>
                          <p className="font-extrabold text-slate-900">{dev.display_name}</p>
                          <p className="text-[10px] text-slate-400 font-mono">{dev.device_id}</p>
                        </div>
                      </div>
                    </td>

                    <td className="p-4">
                      <span
                        className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-bold ${
                          dev.status === 'online'
                            ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                            : dev.status === 'degraded'
                            ? 'bg-amber-50 text-amber-700 border border-amber-200'
                            : 'bg-rose-50 text-rose-700 border border-rose-200'
                        }`}
                      >
                        <span
                          className={`w-1.5 h-1.5 rounded-full ${
                            dev.status === 'online' ? 'bg-emerald-500' : dev.status === 'degraded' ? 'bg-amber-500' : 'bg-rose-500'
                          }`}
                        />
                        {dev.status.toUpperCase()}
                      </span>
                    </td>

                    <td className="p-4">
                      <p className="font-bold text-slate-800">{dev.model}</p>
                      <p className="text-[11px] text-slate-500">Android {dev.android_version}</p>
                    </td>

                    <td className="p-4">
                      <p className="font-bold text-slate-800">{dev.telemetry.battery}% ⚡</p>
                      <p className="text-[11px] text-slate-500 uppercase">{dev.telemetry.network}</p>
                    </td>

                    <td className="p-4">
                      <div className="flex flex-wrap gap-1">
                        {dev.groups.map((grpId) => {
                          const grp = mockGroups.find((g) => g.group_id === grpId);
                          return (
                            <span key={grpId} className="px-2 py-0.5 rounded-md bg-slate-100 text-slate-700 font-semibold text-[10px]">
                              {grp ? grp.name : grpId}
                            </span>
                          );
                        })}
                      </div>
                    </td>

                    <td className="p-4 text-right">
                      <PermissionGuard permission="device.control.acquire">
                        <button
                          onClick={() => setActiveControlDevice(dev)}
                          className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-sm transition-colors inline-flex items-center gap-1"
                        >
                          <Play size={14} /> Điều khiển
                        </button>
                      </PermissionGuard>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* Control Modal */}
      {activeControlDevice && (
        <DeviceControlModal device={activeControlDevice} onClose={() => setActiveControlDevice(null)} />
      )}
    </div>
  );
};
