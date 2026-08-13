import React, { useEffect, useRef, useState } from 'react';
import { DeviceEntity, ControlLease } from '../../types';
import { defaultMediaRegistry, MediaClient } from '../../services/media-client';
import { defaultCommandEngine } from '../../services/command-engine';
import {
  X,
  Smartphone,
  Home,
  ArrowLeft,
  Layers,
  Send,
  Lock,
  Power,
  SlidersHorizontal,
  ShieldAlert,
  Clock,
  Sparkles,
} from 'lucide-react';
import { mockCurrentUserSession } from '../../data/mockData';
import { PermissionGuard } from '../common/PermissionGuard';

interface DeviceControlModalProps {
  device: DeviceEntity;
  isOpen: boolean;
  onClose: () => void;
}

interface CommandLogItem {
  id: string;
  msg: string;
  time: string;
}

export const DeviceControlModal: React.FC<DeviceControlModalProps> = ({ device, isOpen, onClose }) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [lease, setLease] = useState<ControlLease | null>(null);
  const [leaseSecondsLeft, setLeaseSecondsLeft] = useState<number>(0);
  const [textPayload, setTextPayload] = useState('');
  const [commandLog, setCommandLog] = useState<CommandLogItem[]>([]);
  const mediaClientRef = useRef<MediaClient | null>(null);

  const addLog = (msg: string) => {
    setCommandLog((prev) => [
      { id: Math.random().toString(), msg, time: new Date().toLocaleTimeString('vi-VN') },
      ...prev.slice(0, 7),
    ]);
  };

  // Initialize MediaClient for device stream
  useEffect(() => {
    if (!isOpen) return;

    const sessionId = `str_${device.device_id}`;
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
  }, [device, isOpen]);

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
    addLog('Released control lease manually');
  };

  const handleCanvasClick = async (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!canvasRef.current) return;

    const rect = canvasRef.current.getBoundingClientRect();
    const x = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    const y = Math.max(0, Math.min(1, (e.clientY - rect.top) / rect.height));

    if (mediaClientRef.current) {
      mediaClientRef.current.simulateTouch(x, y);
    }

    try {
      const now = new Date();
      await defaultCommandEngine.dispatchCommand({
        deviceId: device.device_id,
        type: 'gesture.touch',
        payload: { x: Number(x.toFixed(3)), y: Number(y.toFixed(3)) },
        controlLeaseId: lease?.control_lease_id,
        idempotencyKey: `idemp_${Math.random().toString(36).substring(2, 8)}`,
        issuedAt: now.toISOString(),
        expiresAt: new Date(now.getTime() + 10000).toISOString(),
      });
      addLog(`Touch gesture at (${(x * 100).toFixed(0)}%, ${(y * 100).toFixed(0)}%)`);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Command Failed';
      addLog(`[Error] ${msg}`);
    }
  };

  const handleGlobalAction = async (action: 'global.back' | 'global.home' | 'global.recents') => {
    try {
      const now = new Date();
      await defaultCommandEngine.dispatchCommand({
        deviceId: device.device_id,
        type: action,
        payload: {},
        controlLeaseId: lease?.control_lease_id,
        idempotencyKey: `idemp_${Math.random().toString(36).substring(2, 8)}`,
        issuedAt: now.toISOString(),
        expiresAt: new Date(now.getTime() + 10000).toISOString(),
      });
      addLog(`Global Key: ${action}`);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Command Failed';
      addLog(`[Error] ${msg}`);
    }
  };

  const handleSendText = async () => {
    if (!textPayload.trim()) return;

    try {
      const now = new Date();
      await defaultCommandEngine.dispatchCommand({
        deviceId: device.device_id,
        type: 'input.text',
        payload: { text: textPayload },
        controlLeaseId: lease?.control_lease_id,
        idempotencyKey: `idemp_${Math.random().toString(36).substring(2, 8)}`,
        issuedAt: now.toISOString(),
        expiresAt: new Date(now.getTime() + 10000).toISOString(),
      });
      addLog(`Type text: "${textPayload}"`);
      setTextPayload('');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Command Failed';
      addLog(`[Error] ${msg}`);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/80 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 text-white rounded-3xl shadow-2xl w-full max-w-5xl max-h-[90vh] flex flex-col overflow-hidden animate-fadeIn">
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/50">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-amber-500/10 border border-amber-500/20 text-amber-500 flex items-center justify-center font-bold">
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
            className="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-xl transition-colors"
          >
            <X size={20} />
          </button>
        </div>

        {/* Modal Content Grid */}
        <div className="flex-1 grid grid-cols-1 lg:grid-cols-12 overflow-hidden">
          {/* Left Column: Simulated Screen View (5 Cols) */}
          <div className="lg:col-span-5 p-6 bg-slate-950 flex flex-col items-center justify-center border-b lg:border-b-0 lg:border-r border-slate-800 relative">
            <div className="relative rounded-3xl border-4 border-slate-700 bg-slate-900 shadow-2xl p-2 max-w-[280px]">
              <canvas
                ref={canvasRef}
                width={270}
                height={480}
                onClick={handleCanvasClick}
                className={`rounded-2xl block bg-slate-900 transition-all ${
                  lease ? 'cursor-crosshair hover:opacity-95' : 'cursor-not-allowed opacity-80'
                }`}
              />
              {!lease && (
                <div className="absolute inset-0 m-2 rounded-2xl bg-slate-950/70 backdrop-blur-[2px] flex flex-col items-center justify-center p-4 text-center">
                  <Lock size={32} className="text-amber-400 mb-2 animate-bounce" />
                  <p className="text-xs font-bold text-white mb-1">Require Control Lease</p>
                  <p className="text-[11px] text-slate-400 mb-3">Click button on right panel to acquire 60s lease</p>
                  <button
                    onClick={acquireLease}
                    className="px-3.5 py-2 bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-600 hover:to-amber-700 text-white text-xs font-extrabold rounded-xl shadow-lg transition-all"
                  >
                    Acquire Lease Now
                  </button>
                </div>
              )}
            </div>
            <p className="text-[10px] text-slate-500 mt-3 font-medium">
              Click on canvas to simulate touch gesture
            </p>
          </div>

          {/* Right Column: Interactive Controls & Logs (7 Cols) */}
          <div className="lg:col-span-7 p-6 overflow-y-auto space-y-5 custom-scrollbar bg-slate-900">
            {/* Lease Status Box */}
            <div className="p-4 rounded-2xl bg-slate-950 border border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className={`p-2.5 rounded-xl ${lease ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-slate-800 text-slate-400'}`}>
                  <Clock size={20} />
                </div>
                <div>
                  <h4 className="text-xs font-extrabold text-white">
                    {lease ? 'Control Lease Active' : 'Control Lease Expired'}
                  </h4>
                  <p className="text-[11px] text-slate-400">
                    {lease ? `Remaining: ${leaseSecondsLeft}s (ID: ${lease.control_lease_id})` : 'Acquire lease to send gestures'}
                  </p>
                </div>
              </div>

              {lease ? (
                <button
                  onClick={releaseLease}
                  className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-bold rounded-xl transition-all border border-slate-700"
                >
                  Release Lease
                </button>
              ) : (
                <PermissionGuard permission="device.control.acquire">
                  <button
                    onClick={acquireLease}
                    className="px-3 py-1.5 bg-amber-500 hover:bg-amber-600 text-white text-xs font-extrabold rounded-xl transition-all shadow-md shadow-amber-500/20"
                  >
                    Acquire Lease (60s)
                  </button>
                </PermissionGuard>
              )}
            </div>

            {/* Android Navigation Keys */}
            <div className="space-y-2">
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider">Android Navigation Keys</h4>
              <div className="grid grid-cols-3 gap-2">
                <button
                  disabled={!lease}
                  onClick={() => handleGlobalAction('global.back')}
                  className="flex items-center justify-center gap-2 py-2.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 rounded-xl text-xs font-bold transition-all border border-slate-700"
                >
                  <ArrowLeft size={16} /> Back
                </button>
                <button
                  disabled={!lease}
                  onClick={() => handleGlobalAction('global.home')}
                  className="flex items-center justify-center gap-2 py-2.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 rounded-xl text-xs font-bold transition-all border border-slate-700"
                >
                  <Home size={16} /> Home
                </button>
                <button
                  disabled={!lease}
                  onClick={() => handleGlobalAction('global.recents')}
                  className="flex items-center justify-center gap-2 py-2.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 rounded-xl text-xs font-bold transition-all border border-slate-700"
                >
                  <Layers size={16} /> Recents
                </button>
              </div>
            </div>

            {/* Text Input Payload */}
            <div className="space-y-2">
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider">Send Remote Text Payload</h4>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={textPayload}
                  onChange={(e) => setTextPayload(e.target.value)}
                  disabled={!lease}
                  placeholder="Type message to send to device..."
                  className="flex-1 bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs font-medium text-white placeholder:text-slate-600 focus:outline-none focus:border-amber-500 disabled:opacity-40"
                />
                <button
                  disabled={!lease || !textPayload.trim()}
                  onClick={handleSendText}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-40 text-white font-bold text-xs rounded-xl flex items-center gap-1.5 transition-all shadow-md"
                >
                  <Send size={14} /> Send
                </button>
              </div>
            </div>

            {/* Capabilities Guard Notice */}
            <div className="p-3.5 rounded-2xl bg-amber-500/10 border border-amber-500/20 flex items-start gap-2.5 text-xs text-amber-200">
              <ShieldAlert size={18} className="text-amber-400 shrink-0 mt-0.5" />
              <div>
                <span className="font-bold text-amber-400">Capabilities Guard Note: </span>
                Reboot, Power, Proxy modification and APK Installation require explicit root/ADB capability flags.
              </div>
            </div>

            {/* Sensitive Actions (Disabled on Standard APK) */}
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              <button
                disabled
                title="Capability Disabled on Standard APK"
                className="flex items-center justify-center gap-1.5 py-2 bg-slate-950 text-slate-600 rounded-xl text-xs font-medium cursor-not-allowed border border-slate-800"
              >
                <Power size={14} /> Reboot Device
              </button>
              <button
                disabled
                title="Capability Disabled on Standard APK"
                className="flex items-center justify-center gap-1.5 py-2 bg-slate-950 text-slate-600 rounded-xl text-xs font-medium cursor-not-allowed border border-slate-800"
              >
                <SlidersHorizontal size={14} /> Apply Proxy
              </button>
              <button
                disabled
                title="Capability Disabled on Standard APK"
                className="flex items-center justify-center gap-1.5 py-2 bg-slate-950 text-slate-600 rounded-xl text-xs font-medium cursor-not-allowed border border-slate-800"
              >
                <Sparkles size={14} /> Install APK
              </button>
            </div>

            {/* Command Dispatch Log Stream */}
            <div className="space-y-2 pt-1">
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider">Realtime Dispatch Log</h4>
              <div className="bg-slate-950 border border-slate-800 rounded-2xl p-3 font-mono text-[11px] space-y-1.5 max-h-36 overflow-y-auto custom-scrollbar">
                {commandLog.length === 0 ? (
                  <p className="text-slate-600 italic">No commands dispatched yet in this session.</p>
                ) : (
                  commandLog.map((item) => (
                    <div key={item.id} className="flex items-center justify-between text-slate-300">
                      <span>{item.msg}</span>
                      <span className="text-slate-500 text-[10px]">{item.time}</span>
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
