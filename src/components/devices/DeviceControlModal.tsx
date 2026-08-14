import React, { useEffect, useRef, useState } from 'react';
import { DeviceEntity, ControlLease, DeviceCommandType } from '../../types';
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
  const videoRef = useRef<HTMLVideoElement | null>(null);

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
        resolution: '720p',
        fps: 30,
        bitrate_kbps: 2500,
      });

      if (mounted) {
        if (videoRef.current) {
          mediaClient.attach(videoRef.current);
        }
        if (canvasRef.current) {
          mediaClient.attach(canvasRef.current);
        }
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
      organization_id: session.organization_id,
      user_id: session.user_id,
      user_display_name: session.display_name,
      fencing_token: Math.floor(Math.random() * 1000) + 1,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 300 * 1000).toISOString(),
      ttl_seconds: 300,
    };
    try {
      defaultCommandEngine.registerLease(newLease);
      setLease(newLease);
      addLog('Acquired exclusive control lease (300s)');
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to acquire lease';
      addLog(`Error acquiring lease: ${msg}`);
    }
  };

  const releaseLease = () => {
    if (lease) {
      defaultCommandEngine.revokeLease(lease.control_lease_id);
      setLease(null);
      addLog('Released control lease manually');
    }
  };

  const handleTouch = (normX: number, normY: number) => {
    if (!lease || !session) {
      addLog('Cannot touch: CONTROL_LEASE_REQUIRED');
      return;
    }
    try {
      const cmd = defaultCommandEngine.dispatch(
        {
          deviceId: device.device_id,
          type: 'gesture.touch',
          payload: { x: normX, y: normY },
          controlLeaseId: lease.control_lease_id,
          idempotencyKey: `touch_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
          issuedAt: new Date().toISOString(),
          expiresAt: new Date(Date.now() + 30 * 1000).toISOString(),
        },
        session
      );
      if (mediaClientRef.current) {
        mediaClientRef.current.simulateTouch(normX, normY);
      }
      addLog(`Touch gesture at (x: ${normX.toFixed(2)}, y: ${normY.toFixed(2)}) - Cmd #${cmd.command_id}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Touch command failed';
      addLog(`Touch error: ${msg}`);
    }
  };

  const handleCanvasClick = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;
    handleTouch(x, y);
  };

  const handleVideoClick = (e: React.MouseEvent<HTMLVideoElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;
    handleTouch(x, y);
  };

  const sendKey = (keyName: string) => {
    if (!lease || !session) {
      addLog('Cannot send key: CONTROL_LEASE_REQUIRED');
      return;
    }
    const typeMap: Record<string, DeviceCommandType> = {
      BACK: 'global.back',
      HOME: 'global.home',
      RECENTS: 'global.recents',
    };
    const cmdType: DeviceCommandType = typeMap[keyName] || 'global.back';

    try {
      const cmd = defaultCommandEngine.dispatch(
        {
          deviceId: device.device_id,
          type: cmdType,
          payload: { key: keyName },
          controlLeaseId: lease.control_lease_id,
          idempotencyKey: `key_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
          issuedAt: new Date().toISOString(),
          expiresAt: new Date(Date.now() + 30 * 1000).toISOString(),
        },
        session
      );
      addLog(`Sent key: ${keyName} - Cmd #${cmd.command_id}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Key command failed';
      addLog(`Key error: ${msg}`);
    }
  };

  const sendTextInput = () => {
    if (!lease || !session) {
      addLog('Cannot send text: CONTROL_LEASE_REQUIRED');
      return;
    }
    if (!textPayload.trim()) return;

    try {
      const cmd = defaultCommandEngine.dispatch(
        {
          deviceId: device.device_id,
          type: 'input.text',
          payload: { text: textPayload },
          controlLeaseId: lease.control_lease_id,
          idempotencyKey: `text_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
          issuedAt: new Date().toISOString(),
          expiresAt: new Date(Date.now() + 30 * 1000).toISOString(),
        },
        session
      );
      addLog(`Typed text: "${textPayload}" - Cmd #${cmd.command_id}`);
      setTextPayload('');
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Type text failed';
      addLog(`Text error: ${msg}`);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-md animate-fadeIn">
      <div className="bg-slate-900 border border-slate-800 rounded-3xl w-full max-w-4xl max-h-[90vh] flex flex-col overflow-hidden shadow-2xl">
        {/* Modal Header */}
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-900/80">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-400">
              <Smartphone size={22} />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-extrabold text-base text-white">{device.display_name}</h3>
                <span className="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 font-bold text-[10px] border border-emerald-500/20">
                  {device.status.toUpperCase()}
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono">
                {device.model} ({device.android_version}) • ID: {device.device_id}
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
          {/* Stream Player (Left Column) */}
          <div className="md:col-span-6 bg-slate-950 p-6 flex flex-col items-center justify-center border-b md:border-b-0 md:border-r border-slate-800/80 relative">
            <div className="relative rounded-3xl overflow-hidden shadow-2xl border-4 border-slate-800 bg-black aspect-[9/16] w-full max-w-[280px]">
              <video
                ref={videoRef}
                autoPlay
                playsInline
                muted
                onClick={handleVideoClick}
                className="w-full h-full cursor-crosshair object-contain bg-black"
              />
              <canvas
                ref={canvasRef}
                width={360}
                height={640}
                onClick={handleCanvasClick}
                data-session-id={sessionId}
                className="absolute inset-0 w-full h-full cursor-crosshair object-contain"
              />

              {!lease && (
                <div className="absolute inset-0 bg-slate-950/60 backdrop-blur-[2px] flex flex-col items-center justify-center p-4 text-center space-y-3 cursor-pointer z-10" onClick={acquireLease}>
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
                      Lấy Quyền (Lease)
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
                  className="px-3 py-1.5 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 text-xs font-bold rounded-xl transition-all"
                >
                  Release Lease
                </button>
              ) : (
                <PermissionGuard permission="device.control.acquire">
                  <button
                    onClick={acquireLease}
                    className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold rounded-xl transition-all shadow-md shadow-blue-500/20"
                  >
                    Lấy Quyền (Lease)
                  </button>
                </PermissionGuard>
              )}
            </div>

            {/* Quick Action Buttons */}
            <div className="space-y-2">
              <span className="text-[10px] font-extrabold text-slate-400 uppercase tracking-wider">
                Phím Cứng Điều Hướng
              </span>
              <PermissionGuard permission="device.control.input">
                <div className="grid grid-cols-3 gap-2">
                  <button
                    onClick={() => sendKey('BACK')}
                    disabled={!lease}
                    className="p-2.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 font-bold text-xs rounded-xl border border-slate-700/60 transition-all active:scale-95 flex items-center justify-center gap-1.5"
                  >
                    <RotateCcw size={14} /> Back
                  </button>
                  <button
                    onClick={() => sendKey('HOME')}
                    disabled={!lease}
                    className="p-2.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 font-bold text-xs rounded-xl border border-slate-700/60 transition-all active:scale-95 flex items-center justify-center gap-1.5"
                  >
                    <Smartphone size={14} /> Home
                  </button>
                  <button
                    onClick={() => sendKey('RECENTS')}
                    disabled={!lease}
                    className="p-2.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 font-bold text-xs rounded-xl border border-slate-700/60 transition-all active:scale-95 flex items-center justify-center gap-1.5"
                  >
                    <Play size={14} /> Recents
                  </button>
                </div>
              </PermissionGuard>
            </div>

            {/* Text Input Panel */}
            <div className="space-y-2">
              <span className="text-[10px] font-extrabold text-slate-400 uppercase tracking-wider">
                Gửi Văn Bản (Type Text)
              </span>
              <PermissionGuard permission="device.control.input">
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={textPayload}
                    onChange={(e) => setTextPayload(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && sendTextInput()}
                    placeholder="Nhập nội dung cần truyền sang thiết bị..."
                    disabled={!lease}
                    className="flex-1 bg-slate-950 border border-slate-800 focus:border-blue-500 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-500 disabled:opacity-40 outline-none transition-all"
                  />
                  <button
                    onClick={sendTextInput}
                    disabled={!lease || !textPayload.trim()}
                    className="px-4 py-2 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 disabled:opacity-40 text-white font-extrabold text-xs rounded-xl transition-all shadow-md shadow-blue-500/20 active:scale-95"
                  >
                    Send
                  </button>
                </div>
              </PermissionGuard>
            </div>

            {/* Real-time Command Audit Log */}
            <div className="space-y-2 flex-1 flex flex-col min-h-[140px]">
              <div className="flex items-center justify-between">
                <span className="text-[10px] font-extrabold text-slate-400 uppercase tracking-wider flex items-center gap-1">
                  <Clock size={12} /> Nhật Ký Lệnh Dispatch (Audit Trail)
                </span>
                <span className="text-[10px] font-mono text-slate-500">{commandLog.length} events</span>
              </div>

              <div className="bg-slate-950 border border-slate-800/80 rounded-2xl p-3 flex-1 overflow-y-auto font-mono text-[11px] space-y-1.5 max-h-[160px]">
                {commandLog.length === 0 ? (
                  <p className="text-slate-600 text-center py-4 italic">Chưa có lệnh nào được phát đi...</p>
                ) : (
                  commandLog.map((log) => (
                    <div key={log.id} className="flex items-start justify-between text-slate-300 border-b border-slate-900 pb-1">
                      <span className="text-blue-400">[{log.time}]</span>
                      <span className="flex-1 ml-2 text-slate-200 truncate">{log.msg}</span>
                      <CheckCircle size={12} className="text-emerald-400 ml-1 shrink-0 mt-0.5" />
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
