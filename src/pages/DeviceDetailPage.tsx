import React, { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { mockDevices } from '../data/mockData';
import { defaultCommandEngine } from '../services/command-engine';
import { DeviceControlModal } from '../components/devices/DeviceControlModal';
import { PermissionGuard } from '../components/common/PermissionGuard';
import { ArrowLeft, Play, AlertCircle } from 'lucide-react';

export const DeviceDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const [isControlModalOpen, setIsControlModalOpen] = useState(false);

  // Derive device & command history directly during render
  const device = mockDevices.find((d) => d.device_id === id) || null;
  const commandHistory = device ? defaultCommandEngine.getCommands(device.device_id) : [];

  if (!device) {
    return (
      <div className="p-8 flex flex-col items-center justify-center min-h-[60vh] text-center space-y-4">
        <div className="w-16 h-16 rounded-full bg-rose-50 border border-rose-200 text-rose-500 flex items-center justify-center">
          <AlertCircle size={32} />
        </div>
        <h2 className="text-xl font-extrabold text-slate-900 tracking-tight">404 — Device Not Found</h2>
        <p className="text-xs text-slate-500 max-w-sm">
          No device matched ID <code className="bg-slate-100 px-1.5 py-0.5 rounded font-mono text-slate-700">{id}</code>. It may have been unassigned or removed from your organization.
        </p>
        <Link
          to="/app/devices"
          className="inline-flex items-center gap-2 px-4 py-2 bg-slate-900 hover:bg-slate-800 text-white text-xs font-bold rounded-xl transition-all shadow-sm"
        >
          <ArrowLeft size={16} /> Return to Device List
        </Link>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Breadcrumb Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link
            to="/app/devices"
            className="p-2 bg-white border border-slate-200 hover:border-slate-300 text-slate-600 rounded-xl transition-all shadow-sm"
          >
            <ArrowLeft size={18} />
          </Link>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-extrabold text-slate-900 tracking-tight">{device.display_name}</h1>
              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-100 text-slate-700 border border-slate-200">
                {device.model}
              </span>
            </div>
            <p className="text-xs text-slate-500 font-mono">
              ID: {device.device_id} | Serial: {device.serial_number}
            </p>
          </div>
        </div>

        <PermissionGuard permission="device.control.acquire">
          <button
            onClick={() => setIsControlModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 text-white font-extrabold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all active:scale-95"
          >
            <Play size={16} /> {t('devices.acquireControl')}
          </button>
        </PermissionGuard>
      </div>

      {/* Device Specifications Card */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-white p-5 rounded-3xl border border-slate-200/80 shadow-pcp-card space-y-2">
          <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider">Trạng thái kết nối</span>
          <div className="flex items-center gap-2">
            <span className="w-3 h-3 rounded-full bg-emerald-500 animate-ping"></span>
            <span className="text-lg font-black text-slate-900 capitalize">{device.status}</span>
          </div>
          <p className="text-xs text-slate-500 font-mono">Android {device.android_version}</p>
        </div>

        <div className="bg-white p-5 rounded-3xl border border-slate-200/80 shadow-pcp-card space-y-2">
          <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider">Mạng Telemetry</span>
          <div className="text-lg font-black text-slate-900 font-mono">{device.telemetry.network.toUpperCase()}</div>
          <p className="text-xs text-slate-500">
            Orient: <span className="font-semibold text-slate-700">{device.telemetry.orientation}°</span>
          </p>
        </div>

        <div className="bg-white p-5 rounded-3xl border border-slate-200/80 shadow-pcp-card space-y-2">
          <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider">Pin Device</span>
          <div className="flex items-center justify-between">
            <span className="text-lg font-black text-slate-900">{device.telemetry.battery}% ⚡</span>
          </div>
          <div className="w-full bg-slate-100 rounded-full h-2 overflow-hidden">
            <div className="bg-emerald-500 h-2 rounded-full" style={{ width: `${device.telemetry.battery}%` }}></div>
          </div>
        </div>
      </div>

      {/* Command Dispatch History */}
      <div className="bg-white border border-slate-200/80 shadow-pcp-card rounded-3xl overflow-hidden p-6 space-y-4">
        <h3 className="font-extrabold text-slate-900 text-sm">Lịch sử lệnh điều khiển ({commandHistory.length})</h3>
        {commandHistory.length === 0 ? (
          <p className="text-xs text-slate-400 italic">Chưa có lệnh nào được thực thi cho thiết bị này.</p>
        ) : (
          <div className="space-y-2">
            {commandHistory.map((cmd) => (
              <div key={cmd.command_id} className="p-3 bg-slate-50 rounded-2xl border border-slate-100 flex items-center justify-between text-xs">
                <div className="flex items-center gap-3">
                  <span className="font-mono font-bold text-slate-700">{cmd.command_id}</span>
                  <span className="px-2 py-0.5 rounded-full bg-blue-50 text-blue-700 font-bold font-mono text-[10px]">
                    {cmd.command_type}
                  </span>
                  <span className="text-slate-500 text-[11px]">{cmd.actor_name}</span>
                </div>
                <div className="flex items-center gap-3 font-mono">
                  <span className="text-[10px] text-slate-400">{new Date(cmd.created_at).toLocaleTimeString('vi-VN')}</span>
                  <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 font-bold text-[10px]">
                    {cmd.status}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Device Control Modal */}
      <DeviceControlModal
        device={device}
        isOpen={isControlModalOpen}
        onClose={() => setIsControlModalOpen(false)}
      />
    </div>
  );
};
