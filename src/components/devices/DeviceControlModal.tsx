import React, { useEffect, useRef, useState } from 'react';
import { DeviceEntity, ControlLease } from '../../types';
import { defaultMediaRegistry, MediaClient } from '../../services/media-client';
import { defaultCommandEngine } from '../../services/command-engine';
import { useAuth } from '../../context/AuthContext';
import { PermissionGuard } from '../common/PermissionGuard';
import {
  X,
  Play,
  RotateCcw,
  Smartphone,
  CheckCircle,
  Clock,
  Shield,
  Wifi,
} from 'lucide-react';

interface DeviceControlModalProps {
  device: DeviceEntity;
  isOpen?: boolean;
  onClose: () => void;
}

interface CommandLogItem {
  id: string;
  msg: string;
  time: string;
}

export const DeviceControlModal: React.FC<DeviceControlModalProps> = ({ device, isOpen = true, onClose }) => {
  const { session } = useAuth();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [lease, setLease] = useState<ControlLease | null>(null);
  const [leaseSecondsLeft, setLeaseSecondsLeft] = useState<number>(0);
  const [textPayload, setTextPayload] = useState('');
  const [commandLog, setCommandLog] = useState<CommandLogItem[]>([]);
  const mediaClientRef = useRef<MediaClient | null>(null);

  // Unique stream session ID per viewer instance
  const [sessionId] = useState(() => `str_${device.device_id}_${Math.random().toString(36).substring(2, 7)}`);

  const addLog = (msg: string) => {
    setCommandLog((prev) => [
      { id: Math.random().toString(), msg, time: new Date().toLocaleTimeString('vi-VN') },
      ...prev.slice(0, 7),
    ]);
  };

  // Initialize MediaClient for device stream
  useEffect(() => {
    if (!isOpen) return;

    const mediaClient = defaultMediaRegistry.acquire(sessionId);
    mediaClientRef.current = mediaClient;

    let mounted = true;

    async function initStream() {
      await mediaClient.startSession(device.device_id, {
        resolution: '480p',
        fps: 30,
        bitrate_kbps: 1500,
      });

      if (mounted && canvasRef.current) {
        mediaClient.attach(canvasRef.current);
      }
    }

    initStream();

    return () => {
      mounted = false;
      defaultMediaRegistry.release(sessionId);
    };
  }, [device, isOpen, sessionId]);

  // Handle Lease Timer Countdown
  useEffect(() => {
    if (!lease) return;

    const timer = setInterval(() => {
      const remaining = Math.max(0, Math.floor((new Date(lease.expires_at).getTime() - Date.now()) / 1000));
      setLeaseSecondsLeft(remaining);

      if (remaining <= 0) {
        if (lease) defaultCommandEngine.revokeLease(lease.control_lease_id);
        setLease(null);
        addLog('Control lease ended (Expired)');
      }
    }, 1000);

    return () => clearInterval(timer);
  }, [lease]);

  const acquireLease = () => {
    if (!session) return;
    const newLease: ControlLease = {
      control_lease_id: `lease_${Math.random().toString(36).substring(2, 8)}`,
      device_id: device.device_id,
      organization_id: device.organization_id,
      user_id: session.user_id,
      user_display_name: session.display_name,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    };

    defaultCommandEngine.registerLease(newLease);
    setLease(newLease);
    setLeaseSecondsLeft(60);
    addLog('Acquired 60s exclusive control lease');
  };

  const releaseLease = () => {
    if (lease) {
      defaultCommandEngine.revokeLease(lease.control_lease_id);
    }
    setLease(null);
    setLeaseSecondsLeft(0);
    addLog('Released control lease manually');
  };

  const handleCanvasClick = async (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!canvasRef.current || !session) return;

    const rect = canvasRef.current.getBoundingClientRect();
    const x = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    const y = Math.max(0, Math.min(1, (e.clientY - rect.top) / rect.height));

    if (mediaClientRef.current) {
      mediaClientRef.current.simulateTouch(x, y);
    }

    try {
      const now = new Date();
      defaultCommandEngine.dispatch(
        {
          deviceId: device.device_id,
          type: 'gesture.touch',
          payload: { x: Number(x.toFixed(3)), y: Number(y.toFixed(3)) },
          controlLeaseId: lease?.control_lease_id,
          idempotencyKey: `idemp_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        session
      );
      addLog(`Touch gesture at (${(x * 100).toFixed(0)}%, ${(y * 100).toFixed(0)}%)`);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Command Failed';
      addLog(`[Error] ${msg}`);
    }
  };

  const handleGlobalAction = async (action: 'global.back' | 'global.home' | 'global.recents') => {
    if (!session) return;
    try {
      const now = new Date();
      defaultCommandEngine.dispatch(
        {
          deviceId: device.device_id,
          type: action,
          payload: {},
          controlLeaseId: lease?.control_lease_id,
          idempotencyKey: `idemp_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        session
      );
      addLog(`Global Key: ${action}`);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Command Failed';
      addLog(`[Error] ${msg}`);
    }
  };

  const handleSendText = async () => {
    if (!textPayload.trim() || !session) return;
    try {
      const now = new Date();
      defaultCommandEngine.dispatch(
        {
          deviceId: device.device_id,
          type: 'input.text',
          payload: { text: textPayload },
          controlLeaseId: lease?.control_lease_id,
          idempotencyKey: `idemp_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        session
      );
      addLog(`Input text: "${textPayload}"`);
      setTextPayload('');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Command Failed';
      addLog(`[Error] ${msg}`);
    }
  };

  const handleReboot = async () => {
    if (!session) return;
    try {
      const now = new Date();
      defaultCommandEngine.dispatch(
        {
          deviceId: device.device_id,
          type: 'device.reboot',
          payload: {},
          idempotencyKey: `idemp_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        session
      );
      addLog('Dispatched: Device Reboot');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Command Failed';
      addLog(`[Error] ${msg}`);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-md animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 rounded-3xl shadow-2xl w-full max-w-5xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Modal Header */}
        <div className="p-4 px-6 border-b border-slate-800/80 bg-slate-900/90 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-2xl bg-blue-500/10 border border-blue-500/20 text-blue-400">
              <Smartphone size={20} />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-extrabold text-base tracking-tight text-white">{device.display_name}</h3>
                <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-800 text-slate-300 border border-slate-700">
                  {device.model}
                </span>
                <span className="flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span> ONLINE
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono">
                Serial: {device.serial_number} | ID: {device.device_id}
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-white bg-slate-800/60 hover:bg-slate-800 rounded-full transition-all"
          >
            <X size={18} />
          </button>
        </div>

        {/* Modal Body Grid */}
        <div className="grid grid-cols-1 md:grid-cols-12 overflow-hidden flex-1">
          {/* Stream Canvas (Left Column) */}
          <div className="md:col-span-6 bg-slate-950 p-6 flex flex-col items-center justify-center border-b md:border-b-0 md:border-r border-slate-800/80 relative">
            <div className="relative rounded-3xl overflow-hidden shadow-2xl border-4 border-slate-800 bg-black aspect-[9/16] w-full max-w-[280px]">
              <canvas
                ref={canvasRef}
                width={360}
                height={640}
                onClick={handleCanvasClick}
                className="w-full h-full cursor-crosshair object-contain"
              />

              {!lease && (
                <div className="absolute inset-0 bg-slate-950/60 backdrop-blur-[2px] flex flex-col items-center justify-center p-4 text-center space-y-3 cursor-pointer" onClick={acquireLease}>
                  <Shield size={32} className="text-amber-400 animate-bounce" />
                  <p className="text-xs font-bold text-slate-200">Interactive Control Lock</p>
                  <p className="text-[11px] text-slate-400 max-w-[180px]">
                    Acquire a control lease to interact with touch gestures and send input commands.
                  </p>
                  <PermissionGuard permission="device.control.acquire">
                    <button
                      onClick={acquireLease}
                      className="px-4 py-2 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white font-extrabold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all active:scale-95"
                    >
                      Acquire Control Lease
                    </button>
                  </PermissionGuard>
                </div>
              )}
            </div>

            <p className="text-[10px] text-slate-500 mt-3 font-mono flex items-center gap-1">
              <Wifi size={12} className="text-emerald-400" /> Session ID: {sessionId}
            </p>
          </div>

          {/* Control Panel (Right Column) */}
          <div className="md:col-span-6 p-6 space-y-5 bg-slate-900/50 flex flex-col overflow-y-auto">
            {/* Lease Status Card */}
            <div className="bg-slate-800/60 border border-slate-800 rounded-2xl p-4 flex items-center justify-between">
              <div className="space-y-1">
                <span className="text-[10px] font-extrabold text-slate-400 uppercase tracking-wider">
                  Quyền Điều Khiển (Lease)
                </span>
                {lease ? (
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
                    <span className="text-xs font-extrabold text-emerald-400">
                      Active ({leaseSecondsLeft}s còn lại)
                    </span>
                  </div>
                ) : (
                  <div className="text-xs font-bold text-amber-400">Chỉ xem (View Only)</div>
                )}
              </div>

              {lease ? (
                <button
                  onClick={releaseLease}
                  className="px-3 py-1.5 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 font-bold text-xs rounded-xl transition-all"
                >
                  Nhả Lease
                </button>
              ) : (
                <PermissionGuard permission="device.control.acquire">
                  <button
                    onClick={acquireLease}
                    className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white font-extrabold text-xs rounded-xl shadow-md transition-all"
                  >
                    Lấy Quyền (Lease)
                  </button>
                </PermissionGuard>
              )}
            </div>

            {/* Global Actions */}
            <div className="space-y-2">
              <span className="text-[11px] font-extrabold text-slate-400 uppercase tracking-wider">
                Phím Cứng Hệ Thống
              </span>
              <div className="grid grid-cols-3 gap-2">
                <button
                  onClick={() => handleGlobalAction('global.back')}
                  disabled={!lease}
                  className="p-3 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed border border-slate-700/60 rounded-xl text-slate-200 text-xs font-bold transition-all flex items-center justify-center gap-1.5"
                >
                  <RotateCcw size={14} /> Back
                </button>
                <button
                  onClick={() => handleGlobalAction('global.home')}
                  disabled={!lease}
                  className="p-3 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed border border-slate-700/60 rounded-xl text-slate-200 text-xs font-bold transition-all flex items-center justify-center gap-1.5"
                >
                  <Play size={14} /> Home
                </button>
                <button
                  onClick={() => handleGlobalAction('global.recents')}
                  disabled={!lease}
                  className="p-3 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed border border-slate-700/60 rounded-xl text-slate-200 text-xs font-bold transition-all flex items-center justify-center gap-1.5"
                >
                  <Smartphone size={14} /> Recents
                </button>
              </div>
            </div>

            {/* Text Input */}
            <div className="space-y-2">
              <span className="text-[11px] font-extrabold text-slate-400 uppercase tracking-wider">
                Gửi Văn Bản Từ Bàn Phím
              </span>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={textPayload}
                  onChange={(e) => setTextPayload(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleSendText()}
                  disabled={!lease}
                  placeholder={lease ? 'Nhập văn bản rồi ấn Enter...' : 'Cần lấy Lease để nhập chữ'}
                  className="flex-1 bg-slate-950 border border-slate-800 text-slate-200 text-xs rounded-xl px-3 py-2 focus:outline-none focus:border-blue-500 disabled:opacity-50 font-sans"
                />
                <button
                  onClick={handleSendText}
                  disabled={!lease || !textPayload.trim()}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-40 text-white text-xs font-bold rounded-xl transition-all"
                >
                  Gửi
                </button>
              </div>
            </div>

            {/* Sensitive Admin Action */}
            <div className="pt-2 border-t border-slate-800/80">
              <PermissionGuard permission="device.command.sensitive">
                <button
                  onClick={handleReboot}
                  className="w-full py-2.5 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 text-xs font-extrabold rounded-xl transition-all"
                >
                  Khởi Động Lại Thiết Bị (Reboot)
                </button>
              </PermissionGuard>
            </div>

            {/* Real-time Command Audit Log */}
            <div className="flex-1 bg-slate-950 border border-slate-800 rounded-2xl p-4 space-y-2 font-mono overflow-y-auto max-h-[160px]">
              <div className="flex items-center justify-between text-[10px] text-slate-500 border-b border-slate-800 pb-1.5">
                <span className="font-bold uppercase tracking-wider flex items-center gap-1">
                  <Clock size={12} /> Nhật Ký Thực Thi Command
                </span>
                <span className="text-emerald-400 flex items-center gap-1">
                  <CheckCircle size={12} /> WebSocket Ready
                </span>
              </div>
              {commandLog.length === 0 ? (
                <p className="text-[11px] text-slate-600 italic">Chưa có thao tác nào được ghi nhận.</p>
              ) : (
                <div className="space-y-1">
                  {commandLog.map((log) => (
                    <div key={log.id} className="text-[11px] flex justify-between gap-2">
                      <span className="text-slate-300">{log.msg}</span>
                      <span className="text-slate-600 text-[10px]">{log.time}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
