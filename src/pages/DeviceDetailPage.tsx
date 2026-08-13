import React, { useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Smartphone,
  ArrowLeft,
  Activity,
  Cpu,
  HardDrive,
  Wifi,
  Battery,
  Globe,
  Package,
  History,
  Play,
  Shield,
  Clock,
  Sparkles,
} from 'lucide-react';
import { mockDevices } from '../data/mockData';
import { DeviceEntity, DeviceCommand } from '../types';
import { defaultMediaClient } from '../services/media-client';
import { defaultCommandEngine } from '../services/command-engine';
import { DeviceControlModal } from '../components/devices/DeviceControlModal';
import { PermissionGuard } from '../components/common/PermissionGuard';

export const DeviceDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();

  const device = mockDevices.find((d) => d.device_id === id) || mockDevices[0];

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [showControlModal, setShowControlModal] = useState(false);
  const [commandHistory, setCommandHistory] = useState<DeviceCommand[]>([]);

  useEffect(() => {
    let mounted = true;

    async function init() {
      await defaultMediaClient.startSession(device.device_id);
      if (mounted && canvasRef.current) {
        defaultMediaClient.attach(canvasRef.current);
      }
    }

    init();
    setCommandHistory(defaultCommandEngine.getCommands(device.device_id));

    return () => {
      mounted = false;
      defaultMediaClient.stop();
    };
  }, [device]);

  const handleTestCommand = async (type: DeviceCommand['command_type']) => {
    const cmd = await defaultCommandEngine.dispatchCommand(device.device_id, type, { test: true });
    setCommandHistory(defaultCommandEngine.getCommands(device.device_id));
  };

  const installedApps = [
    { package: 'com.phonecontrolplatform.app', name: 'PCP Agent Pro', version: 'v1.0.4' },
    { package: 'com.android.chrome', name: 'Google Chrome', version: 'v128.0' },
    { package: 'com.android.settings', name: 'System Settings', version: 'v8.0' },
    { package: 'com.google.android.youtube', name: 'YouTube', version: 'v19.12' },
  ];

  return (
    <div className="space-y-6">
      {/* Back Button & Title Header */}
      <div className="flex items-center justify-between">
        <button
          onClick={() => navigate('/app/devices')}
          className="px-3.5 py-2 bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 font-bold text-xs rounded-xl shadow-sm transition-colors flex items-center gap-2"
        >
          <ArrowLeft size={16} /> Trở về danh sách
        </button>

        <PermissionGuard permission="device.control.acquire">
          <button
            onClick={() => setShowControlModal(true)}
            className="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all flex items-center gap-2"
          >
            <Play size={16} /> Mở bảng điều khiển chi tiết
          </button>
        </PermissionGuard>
      </div>

      {/* Main Grid: Screen Preview + Hardware Telemetry */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Screen Preview Tile (4 cols) */}
        <div className="lg:col-span-4 bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 flex flex-col items-center justify-center space-y-4">
          <div className="flex items-center gap-2.5">
            <span
              className={`w-3 h-3 rounded-full ${
                device.status === 'online' ? 'bg-emerald-500 animate-ping' : 'bg-rose-500'
              }`}
            />
            <h2 className="text-base font-extrabold text-slate-900">{device.display_name}</h2>
          </div>

          <div className="relative rounded-[2rem] p-2 bg-slate-900 border-4 border-slate-700 shadow-2xl">
            <canvas ref={canvasRef} width={240} height={420} className="rounded-[1.5rem] bg-black" />
          </div>

          <div className="flex items-center gap-4 text-xs font-semibold text-slate-600">
            <span>{device.telemetry.battery}% ⚡</span>
            <span className="uppercase">{device.telemetry.network}</span>
            <span className="text-emerald-600 font-bold">18ms</span>
          </div>
        </div>

        {/* Right Details & Telemetry Gauges (8 cols) */}
        <div className="lg:col-span-8 space-y-6">
          {/* Spec Cards */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div className="bg-white border border-slate-100 p-4 rounded-2xl shadow-pcp-card space-y-1">
              <span className="text-[10px] font-extrabold text-slate-400 uppercase">CPU Usage</span>
              <p className="text-xl font-black text-slate-900">{device.telemetry.cpu_usage || 18}%</p>
              <div className="w-full h-1.5 bg-slate-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-blue-600 rounded-full"
                  style={{ width: `${device.telemetry.cpu_usage || 18}%` }}
                />
              </div>
            </div>

            <div className="bg-white border border-slate-100 p-4 rounded-2xl shadow-pcp-card space-y-1">
              <span className="text-[10px] font-extrabold text-slate-400 uppercase">RAM Usage</span>
              <p className="text-xl font-black text-slate-900">{device.telemetry.ram_usage || 42}%</p>
              <div className="w-full h-1.5 bg-slate-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-purple-600 rounded-full"
                  style={{ width: `${device.telemetry.ram_usage || 42}%` }}
                />
              </div>
            </div>

            <div className="bg-white border border-slate-100 p-4 rounded-2xl shadow-pcp-card space-y-1">
              <span className="text-[10px] font-extrabold text-slate-400 uppercase">Android OS</span>
              <p className="text-xl font-black text-slate-900">v{device.android_version}</p>
              <span className="text-[10px] text-slate-400 font-semibold">{device.model}</span>
            </div>

            <div className="bg-white border border-slate-100 p-4 rounded-2xl shadow-pcp-card space-y-1">
              <span className="text-[10px] font-extrabold text-slate-400 uppercase">Serial Number</span>
              <p className="text-xs font-mono font-extrabold text-slate-800 truncate">{device.serial_number}</p>
              <span className="text-[10px] text-emerald-600 font-bold">Verified Hardware</span>
            </div>
          </div>

          {/* Installed Packages List */}
          <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-4">
            <h3 className="text-sm font-extrabold text-slate-900 flex items-center gap-2">
              <Package size={18} className="text-blue-600" /> Danh sách ứng dụng đã cài (Package Simulator)
            </h3>

            <div className="divide-y divide-slate-100">
              {installedApps.map((app) => (
                <div key={app.package} className="py-2.5 flex items-center justify-between text-xs">
                  <div>
                    <p className="font-extrabold text-slate-900">{app.name}</p>
                    <p className="text-[10px] font-mono text-slate-400">{app.package}</p>
                  </div>
                  <span className="px-2 py-0.5 rounded-md bg-slate-100 text-slate-700 font-semibold text-[10px]">
                    {app.version}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Command Execution Log Stream */}
          <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-extrabold text-slate-900 flex items-center gap-2">
                <History size={18} className="text-amber-500" /> Lịch sử thực thi lệnh (Command History)
              </h3>
              <div className="flex gap-2">
                <button
                  onClick={() => handleTestCommand('global_action')}
                  className="px-3 py-1 bg-slate-100 hover:bg-slate-200 text-slate-800 font-bold text-xs rounded-lg"
                >
                  Test Home Key
                </button>
              </div>
            </div>

            <div className="space-y-2">
              {commandHistory.length === 0 ? (
                <p className="text-xs text-slate-400 italic">Chưa có lịch sử lệnh nào cho thiết bị này.</p>
              ) : (
                commandHistory.map((cmd) => (
                  <div
                    key={cmd.command_id}
                    className="p-3 bg-slate-50 rounded-xl border border-slate-100 flex items-center justify-between text-xs"
                  >
                    <div className="space-y-0.5">
                      <div className="flex items-center gap-2 font-mono font-bold text-slate-900">
                        <span>{cmd.command_id}</span>
                        <span className="px-2 py-0.5 rounded-md bg-blue-100 text-blue-800 text-[10px] font-sans">
                          {cmd.command_type}
                        </span>
                      </div>
                      <p className="text-[10px] text-slate-500">By {cmd.actor_name}</p>
                    </div>

                    <span
                      className={`px-2.5 py-1 rounded-full font-bold text-[10px] uppercase ${
                        cmd.status === 'succeeded'
                          ? 'bg-emerald-100 text-emerald-800'
                          : cmd.status === 'executing'
                          ? 'bg-blue-100 text-blue-800 animate-pulse'
                          : 'bg-amber-100 text-amber-800'
                      }`}
                    >
                      {cmd.status}
                    </span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>

      {showControlModal && (
        <DeviceControlModal device={device} onClose={() => setShowControlModal(false)} />
      )}
    </div>
  );
};
