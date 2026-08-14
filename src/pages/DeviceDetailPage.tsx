import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { defaultCommandEngine } from '../services/command-engine';
import { DeviceControlModal } from '../components/devices/DeviceControlModal';
import { PermissionGuard } from '../components/common/PermissionGuard';
import { ArrowLeft, Play, AlertCircle, Loader2 } from 'lucide-react';
import { DeviceEntity } from '../types';
import { deviceService } from '../services/device-service';

export const DeviceDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [isControlModalOpen, setIsControlModalOpen] = useState(false);
  const [device, setDevice] = useState<DeviceEntity | null>(null);
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    if (!id) return;
    let isMounted = true;

    deviceService
      .getById(id)
      .then((data) => {
        if (isMounted) {
          setDevice(data);
          setLoading(false);
        }
      })
      .catch(() => {
        if (isMounted) {
          setDevice(null);
          setLoading(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, [id]);

  const commandHistory = device ? defaultCommandEngine.getCommands(device.device_id) : [];

  if (loading) {
    return (
      <div className="p-16 flex items-center justify-center min-h-[60vh]">
        <div className="flex items-center gap-2 text-slate-500 font-medium">
          <Loader2 className="w-6 h-6 animate-spin text-blue-600" />
          <span>Đang tải thông tin chi tiết thiết bị...</span>
        </div>
      </div>
    );
  }

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
              <h1 className="text-xl font-extrabold text-slate-900 tracking-tight">{device.display_name || device.name}</h1>
              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-100 text-slate-700 border border-slate-200">
                {device.model}
              </span>
            </div>
            <p className="text-xs text-slate-500 font-mono">
              ID: {device.device_id} | Serial: {device.serial_number}
            </p>
          </div>
        </div>

        <PermissionGuard requiredPermission="device.control.input">
          <button
            onClick={() => setIsControlModalOpen(true)}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-xl shadow-sm text-xs flex items-center gap-2 transition-all"
          >
            <Play size={16} />
            <span>Xin quyền điều khiển</span>
          </button>
        </PermissionGuard>
      </div>

      {/* Grid Specs */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Device Information */}
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm space-y-4">
          <h3 className="text-sm font-bold text-slate-900 border-b border-slate-100 pb-2">Thông tin thiết bị</h3>
          <div className="space-y-2.5 text-xs">
            <div className="flex justify-between">
              <span className="text-slate-500">Tên hiển thị:</span>
              <span className="font-semibold text-slate-800">{device.display_name || device.name}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Trạng thái:</span>
              <span
                className={`font-bold uppercase text-[10px] px-2 py-0.5 rounded-full ${
                  device.status === 'online'
                    ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                    : 'bg-slate-100 text-slate-600 border border-slate-200'
                }`}
              >
                {device.status}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Hệ điều hành:</span>
              <span className="font-semibold text-slate-800">Android {device.platform_version || device.android_version}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Mã Serial:</span>
              <span className="font-mono text-slate-700">{device.serial_number}</span>
            </div>
          </div>
        </div>

        {/* Telemetry Status */}
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm space-y-4">
          <h3 className="text-sm font-bold text-slate-900 border-b border-slate-100 pb-2">Thông số Telemetry</h3>
          {device.telemetry ? (
            <div className="space-y-2.5 text-xs">
              <div className="flex justify-between">
                <span className="text-slate-500">Mức Pin:</span>
                <span className="font-bold text-slate-800">{device.telemetry.battery}%</span>
              </div>
              <div className="flex justify-between">
                <span className="text-slate-500">Kết nối mạng:</span>
                <span className="font-bold uppercase text-slate-800">{device.telemetry.network}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-slate-500">CPU Usage:</span>
                <span className="font-semibold text-slate-800">{device.telemetry.cpu_usage}%</span>
              </div>
              <div className="flex justify-between">
                <span className="text-slate-500">RAM Usage:</span>
                <span className="font-semibold text-slate-800">{device.telemetry.ram_usage}%</span>
              </div>
              <div className="flex justify-between">
                <span className="text-slate-500">Nhiệt độ:</span>
                <span className="font-semibold text-slate-800">{device.telemetry.temperature_c}%</span>
              </div>
            </div>
          ) : (
            <div className="text-xs text-slate-400 italic">Chưa có dữ liệu telemetry</div>
          )}
        </div>

        {/* Command History */}
        <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm space-y-4">
          <h3 className="text-sm font-bold text-slate-900 border-b border-slate-100 pb-2">Lịch sử Lệnh gần nhất</h3>
          {commandHistory.length === 0 ? (
            <p className="text-xs text-slate-400 italic">Chưa có lệnh nào được thực thi.</p>
          ) : (
            <div className="space-y-2 overflow-y-auto max-h-48 text-xs pr-1">
              {commandHistory.slice(-5).map((cmd) => (
                <div key={cmd.command_id} className="p-2 bg-slate-50 rounded-xl border border-slate-100">
                  <div className="flex items-center justify-between font-mono text-[11px]">
                    <span className="font-bold text-slate-800">{cmd.command_type}</span>
                    <span className="text-[10px] text-emerald-600 font-bold uppercase">{cmd.status}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Control Modal */}
      {isControlModalOpen && (
        <DeviceControlModal device={device} onClose={() => setIsControlModalOpen(false)} />
      )}
    </div>
  );
};
