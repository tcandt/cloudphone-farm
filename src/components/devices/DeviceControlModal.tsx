import React, { useEffect, useRef, useState } from 'react';
import { DeviceEntity, ControlLease } from '../../types';
import { defaultMediaClient } from '../../services/media-client';
import {
  X,
  Smartphone,
  Wifi,
  Battery,
  RotateCcw,
  Home,
  ArrowLeft,
  Layers,
  Send,
  Lock,
  Power,
  RefreshCw,
  SlidersHorizontal,
  ShieldAlert,
  Clock,
  Sparkles,
} from 'lucide-react';
import { mockCurrentUserSession } from '../../data/mockData';
import { PermissionGuard } from '../common/PermissionGuard';

interface DeviceControlModalProps {
  device: DeviceEntity;
  onClose: () => void;
}

export const DeviceControlModal: React.FC<DeviceControlModalProps> = ({ device, onClose }) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [lease, setLease] = useState<ControlLease | null>(null);
  const [leaseSecondsLeft, setLeaseSecondsLeft] = useState<number>(0);
  const [textInput, setTextInput] = useState('');
  const [commandLog, setCommandLog] = useState<{ id: string; msg: string; time: string }[]>([]);

  // Initialize MediaClient stream
  useEffect(() => {
    let mounted = true;

    async function initStream() {
      await defaultMediaClient.startSession(device.device_id, {
        resolution: '480p',
        fps: 30,
        bitrate_kbps: 1500,
      });

      if (mounted && canvasRef.current) {
        defaultMediaClient.attach(canvasRef.current);
      }
    }

    initStream();

    return () => {
      mounted = false;
      defaultMediaClient.stop();
    };
  }, [device]);

  // Handle Lease Timer Countdown
  useEffect(() => {
    if (!lease) return;

    const timer = setInterval(() => {
      const remaining = Math.max(0, Math.floor((new Date(lease.expires_at).getTime() - Date.now()) / 1000));
      setLeaseSecondsLeft(remaining);

      if (remaining <= 0) {
        setLease(null);
        addLog('Control lease ended (Expired)');
      }
    }, 1000);

    return () => clearInterval(timer);
  }, [lease]);

  const addLog = (msg: string) => {
    setCommandLog((prev) => [
      { id: Math.random().toString(), msg, time: new Date().toLocaleTimeString('vi-VN') },
      ...prev.slice(0, 7),
    ]);
  };

  const acquireLease = () => {
    const newLease: ControlLease = {
      control_lease_id: `lease_${Math.random().toString(36).substring(2, 8)}`,
      device_id: device.device_id,
      organization_id: device.organization_id,
      user_id: mockCurrentUserSession.user_id,
      user_display_name: mockCurrentUserSession.display_name,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    };
    setLease(newLease);
    setLeaseSecondsLeft(60);
    addLog('Acquired 60s exclusive control lease');
  };

  const releaseLease = () => {
    setLease(null);
    setLeaseSecondsLeft(0);
    addLog('Released control lease');
  };

  // Canvas Mouse Click Touch Simulation
  const handleCanvasClick = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!lease) {
      addLog('Lỗi: Cần xin quyền điều khiển (Lease) trước khi chạm!');
      return;
    }

    if (!canvasRef.current) return;
    const rect = canvasRef.current.getBoundingClientRect();
    const xRatio = (e.clientX - rect.left) / rect.width;
    const yRatio = (e.clientY - rect.top) / rect.height;

    defaultMediaClient.simulateTouch(xRatio, yRatio);
    addLog(`Touch event: (${xRatio.toFixed(2)}, ${yRatio.toFixed(2)})`);
  };

  const sendGlobalAction = (action: 'back' | 'home' | 'recents') => {
    if (!lease) {
      addLog('Lỗi: Cần lease active để gửi phím!');
      return;
    }
    addLog(`Gửi phím Android: ${action.toUpperCase()}`);
  };

  const sendTextPayload = () => {
    if (!textInput.trim()) return;
    if (!lease) {
      addLog('Lỗi: Cần lease active để gửi văn bản!');
      return;
    }
    addLog(`Gửi văn bản: "${textInput}"`);
    setTextInput('');
  };

  const capabilities = device.capabilities;

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-white border border-slate-100 shadow-2xl rounded-3xl w-full max-w-4xl max-h-[90vh] flex flex-col overflow-hidden animate-fadeIn">
        {/* Modal Header */}
        <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between bg-slate-50/50">
          <div className="flex items-center gap-3">
            <div className="p-2.5 bg-blue-50 text-blue-600 rounded-2xl">
              <Smartphone size={20} />
            </div>
            <div>
              <h2 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
                {device.display_name}
                <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-slate-200 text-slate-700">
                  {device.model}
                </span>
              </h2>
              <p className="text-xs text-slate-500 font-medium">
                Android {device.android_version} • Serial: {device.serial_number}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {/* Lease Status Badge */}
            {lease ? (
              <div className="flex items-center gap-2 px-3 py-1.5 bg-emerald-50 border border-emerald-200 text-emerald-700 rounded-xl text-xs font-extrabold">
                <Clock size={14} className="animate-spin" />
                <span>Lease: {leaseSecondsLeft}s</span>
                <button
                  onClick={releaseLease}
                  className="ml-2 text-[10px] uppercase font-bold text-rose-600 hover:underline"
                >
                  Giải phóng
                </button>
              </div>
            ) : (
              <PermissionGuard permission="device.control.acquire">
                <button
                  onClick={acquireLease}
                  className="px-4 py-2 bg-gradient-to-r from-amber-500 to-orange-500 text-white font-bold text-xs rounded-xl shadow-md hover:opacity-95 transition-all active:scale-95 flex items-center gap-1.5"
                >
                  <Sparkles size={14} /> Xin quyền điều khiển (60s)
                </button>
              </PermissionGuard>
            )}

            <button
              onClick={onClose}
              className="p-2 text-slate-400 hover:text-slate-700 hover:bg-slate-100 rounded-xl transition-colors"
            >
              <X size={20} />
            </button>
          </div>
        </div>

        {/* Modal Body */}
        <div className="flex-1 overflow-y-auto p-6 grid grid-cols-1 md:grid-cols-12 gap-6 custom-scrollbar">
          {/* Left Screen Interactive Canvas (5 cols) */}
          <div className="md:col-span-5 flex flex-col items-center justify-center space-y-3">
            <div className="relative rounded-[2.5rem] p-3 bg-slate-900 border-4 border-slate-700 shadow-2xl">
              {/* Camera Notch */}
              <div className="absolute top-5 left-1/2 -translate-x-1/2 w-16 h-3 bg-slate-800 rounded-full z-10" />

              <canvas
                ref={canvasRef}
                width={320}
                height={580}
                onClick={handleCanvasClick}
                className={`rounded-[2rem] bg-black cursor-pointer shadow-inner ${
                  !lease ? 'opacity-75 cursor-not-allowed' : ''
                }`}
              />

              {!lease && (
                <div className="absolute inset-0 m-3 rounded-[2rem] bg-slate-950/60 backdrop-blur-[2px] flex items-center justify-center text-white text-center p-4">
                  <div className="space-y-2">
                    <Lock size={32} className="mx-auto text-amber-400" />
                    <p className="text-xs font-bold">Chưa có Control Lease</p>
                    <p className="text-[10px] text-slate-300">Bấm "Xin quyền điều khiển" ở trên để tương tác</p>
                  </div>
                </div>
              )}
            </div>
            <span className="text-[11px] font-medium text-slate-400 text-center">
              Nhấp trực tiếp lên màn hình simulated để gửi cử chỉ Touch
            </span>
          </div>

          {/* Right Control Panels (7 cols) */}
          <div className="md:col-span-7 space-y-5">
            {/* Global Actions (Home, Back, Recents) */}
            <div className="bg-slate-50 border border-slate-100 rounded-2xl p-4 space-y-3">
              <h3 className="text-xs font-extrabold text-slate-800 uppercase tracking-wider">
                Phím điều hướng Android
              </h3>

              <div className="grid grid-cols-3 gap-2">
                <button
                  onClick={() => sendGlobalAction('back')}
                  disabled={!lease || !capabilities.control.global_actions.includes('back')}
                  className="py-2.5 bg-white border border-slate-200 hover:bg-slate-100 rounded-xl text-slate-800 font-bold text-xs flex items-center justify-center gap-2 shadow-sm disabled:opacity-40"
                >
                  <ArrowLeft size={16} /> Back
                </button>
                <button
                  onClick={() => sendGlobalAction('home')}
                  disabled={!lease || !capabilities.control.global_actions.includes('home')}
                  className="py-2.5 bg-white border border-slate-200 hover:bg-slate-100 rounded-xl text-slate-800 font-bold text-xs flex items-center justify-center gap-2 shadow-sm disabled:opacity-40"
                >
                  <Home size={16} /> Home
                </button>
                <button
                  onClick={() => sendGlobalAction('recents')}
                  disabled={!lease || !capabilities.control.global_actions.includes('recents')}
                  className="py-2.5 bg-white border border-slate-200 hover:bg-slate-100 rounded-xl text-slate-800 font-bold text-xs flex items-center justify-center gap-2 shadow-sm disabled:opacity-40"
                >
                  <Layers size={16} /> Recents
                </button>
              </div>
            </div>

            {/* Text Input Payload */}
            <div className="bg-slate-50 border border-slate-100 rounded-2xl p-4 space-y-3">
              <h3 className="text-xs font-extrabold text-slate-800 uppercase tracking-wider">
                Gửi văn bản từ xa (Remote Text Input)
              </h3>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={textInput}
                  onChange={(e) => setTextInput(e.target.value)}
                  disabled={!lease || capabilities.control.text_input === 'none'}
                  placeholder="Nhập nội dung cần nhập vào phone..."
                  className="flex-1 px-3.5 py-2 bg-white border border-slate-200 rounded-xl text-xs font-medium focus:ring-2 focus:ring-blue-500 outline-none disabled:opacity-40"
                />
                <button
                  onClick={sendTextPayload}
                  disabled={!lease || capabilities.control.text_input === 'none'}
                  className="px-4 py-2 bg-blue-600 text-white font-bold text-xs rounded-xl shadow-sm hover:bg-blue-700 transition-colors disabled:opacity-40 flex items-center gap-1.5"
                >
                  <Send size={14} /> Gửi
                </button>
              </div>
            </div>

            {/* Capability Security Note (Amendment #7) */}
            <div className="bg-amber-50/60 border border-amber-200/80 rounded-2xl p-4 space-y-2 text-xs text-amber-900">
              <div className="flex items-center gap-2 font-bold text-amber-800">
                <ShieldAlert size={16} className="text-amber-600" /> Capabilities & Giới hạn APK Standard
              </div>
              <p className="text-[11px] leading-relaxed text-amber-800/90">
                Các nút nhạy cảm (Khởi động lại, Tắt nguồn, Thay Proxy trực tiếp, Cài đặt APK) bị vô hiệu hóa vì thiết bị đang kết nối qua ứng dụng PCP Agent tiêu chuẩn không root/ADB.
              </p>

              <div className="grid grid-cols-2 gap-2 pt-1">
                <button
                  disabled
                  className="py-2 bg-amber-100/50 border border-amber-200 rounded-xl text-[11px] font-bold text-amber-500 cursor-not-allowed flex items-center justify-center gap-1"
                >
                  <Power size={13} /> Reboot (ADB/Root Only)
                </button>
                <button
                  disabled
                  className="py-2 bg-amber-100/50 border border-amber-200 rounded-xl text-[11px] font-bold text-amber-500 cursor-not-allowed flex items-center justify-center gap-1"
                >
                  <SlidersHorizontal size={13} /> Change Proxy (ADB Only)
                </button>
              </div>
            </div>

            {/* Activity / Command Execution Console Log */}
            <div className="bg-slate-900 rounded-2xl p-4 text-xs font-mono text-slate-300 space-y-2">
              <span className="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">
                Command Log Stream:
              </span>
              <div className="space-y-1 max-h-28 overflow-y-auto custom-scrollbar text-[11px]">
                {commandLog.length === 0 ? (
                  <p className="text-slate-500 italic">Chưa có lệnh nào được thực thi.</p>
                ) : (
                  commandLog.map((log) => (
                    <div key={log.id} className="flex items-center gap-2">
                      <span className="text-slate-500 font-bold">[{log.time}]</span>
                      <span className="text-emerald-400">✓ {log.msg}</span>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
