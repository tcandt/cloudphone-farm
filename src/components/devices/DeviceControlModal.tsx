import React, { useEffect, useRef, useState, useCallback } from 'react';
import { DeviceEntity, ControlLease, DeviceCommandType } from '../../types';
import { defaultMediaRegistry, MediaClient } from '../../services/media-client';
import { deviceService } from '../../services/device-service';
import { commandService } from '../../services/command-service';
import { useAuth } from '../../context/AuthContext';
import { PermissionGuard } from '../common/PermissionGuard';
import { computeVideoGeometry, mapPointerToNormalizedCoordinates, VideoContentGeometry } from '../../lib/video-geometry';
import { PointerGestureRecognizer, DispatchedGesture } from '../../lib/pointer-gesture-engine';
import {
  X,
  Play,
  RotateCcw,
  Smartphone,
  CheckCircle,
  Clock,
  Shield,
  Wifi,
  AlertCircle,
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
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const [lease, setLease] = useState<ControlLease | null>(null);
  const [leaseSecondsLeft, setLeaseSecondsLeft] = useState<number>(0);
  const [textPayload, setTextPayload] = useState('');
  const [commandLog, setCommandLog] = useState<CommandLogItem[]>([]);
  const [webrtcState, setWebrtcState] = useState<string>('CONNECTING');
  const [webrtcError, setWebrtcError] = useState<string | null>(null);
  const [activeServerSessionId, setActiveServerSessionId] = useState<string>('');
  const [geometry, setGeometry] = useState<VideoContentGeometry | null>(null);
  const [geometryRevision, setGeometryRevision] = useState(0);

  const mediaClientRef = useRef<MediaClient | null>(null);
  const gestureRecognizerRef = useRef<PointerGestureRecognizer>(new PointerGestureRecognizer());
  const leaseRef = useRef<ControlLease | null>(null);
  const lastPointerUpTimeRef = useRef<number>(0);

  // Keep leaseRef updated for async callbacks & unmount cleanup
  useEffect(() => {
    leaseRef.current = lease;
  }, [lease]);

  const [viewerSessionId] = useState(() => `str_${device.device_id}_${Math.random().toString(36).substring(2, 7)}`);

  const addLog = useCallback((msg: string) => {
    setCommandLog((prev) => [
      { id: Math.random().toString(), msg, time: new Date().toLocaleTimeString('vi-VN') },
      ...prev.slice(0, 9),
    ]);
  }, []);

  // Recalculate video geometry on video metadata load or element resize
  const updateGeometry = useCallback(() => {
    if (!videoRef.current) return;
    const nextRevision = geometryRevision + 1;
    const geom = computeVideoGeometry(videoRef.current, nextRevision);
    if (geom) {
      setGeometry(geom);
      setGeometryRevision(nextRevision);
    }
  }, [geometryRevision]);

  useEffect(() => {
    if (!containerRef.current || !videoRef.current) return;
    const ro = new ResizeObserver(() => {
      updateGeometry();
    });
    ro.observe(containerRef.current);
    ro.observe(videoRef.current);
    return () => ro.disconnect();
  }, [updateGeometry]);

  // MediaClient Stream Initialization
  useEffect(() => {
    if (!isOpen) return;

    const mediaClient = defaultMediaRegistry.acquire(viewerSessionId);
    mediaClientRef.current = mediaClient;

    let mounted = true;

    const unsubscribe = mediaClient.onStateChange?.((state, err, serverSessId) => {
      if (mounted) {
        setWebrtcState(state);
        if (err) setWebrtcError(err);
        if (serverSessId) setActiveServerSessionId(serverSessId);
      }
    });

    async function initStream() {
      try {
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
      } catch (err) {
        if (mounted) {
          const msg = err instanceof Error ? err.message : 'Failed to start media session';
          setWebrtcState('FAILED');
          setWebrtcError(msg);
        }
      }
    }

    initStream();

    return () => {
      mounted = false;
      unsubscribe?.();
      defaultMediaRegistry.release(viewerSessionId);

      // Best-effort release control lease on modal unmount
      if (leaseRef.current) {
        deviceService.releaseLease(device.device_id, leaseRef.current.control_lease_id).catch(() => {});
        leaseRef.current = null;
      }
    };
  }, [device.device_id, isOpen, viewerSessionId]);

  // Control Lease Expiry & Auto-Renew Loop
  useEffect(() => {
    if (!lease) return;

    const timer = setInterval(() => {
      const remaining = Math.max(0, Math.floor((new Date(lease.expires_at).getTime() - Date.now()) / 1000));
      setLeaseSecondsLeft(remaining);

      if (remaining <= 0) {
        setLease(null);
        addLog('Control lease expired');
      } else if (remaining <= 10) {
        // Auto-renew lease 10s before expiry
        deviceService
          .renewLease(device.device_id, lease.control_lease_id)
          .then((renewed) => {
            setLease(renewed);
            addLog(`Auto-renewed control lease (Fencing Token: #${renewed.fencing_token})`);
          })
          .catch((err) => {
            console.warn('[Lease] Auto-renew failed:', err);
            setLease(null);
            addLog('Lease auto-renew failed. Control revoked.');
          });
      }
    }, 1000);

    return () => clearInterval(timer);
  }, [device.device_id, lease, addLog]);

  const acquireLease = async () => {
    if (!session) return;
    try {
      const newLease = await deviceService.acquireLease(device.device_id);
      const remaining = Math.max(0, Math.floor((new Date(newLease.expires_at).getTime() - Date.now()) / 1000));
      setLeaseSecondsLeft(remaining);
      setLease(newLease);
      addLog(`Acquired backend control lease #${newLease.control_lease_id} (Fencing Token: #${newLease.fencing_token})`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to acquire control lease';
      addLog(`Lease error: ${msg}`);
    }
  };

  const releaseLease = async () => {
    if (lease) {
      try {
        await deviceService.releaseLease(device.device_id, lease.control_lease_id);
        addLog('Released control lease manually');
      } catch (err) {
        console.warn('Error releasing lease:', err);
      } finally {
        setLease(null);
      }
    }
  };

  // Dispatch gesture command to backend production HTTP service
  const dispatchGestureCommand = async (gesture: DispatchedGesture) => {
    if (!lease || !session) {
      addLog('Cannot dispatch gesture: CONTROL_LEASE_REQUIRED');
      return;
    }

    try {
      const command = await commandService.dispatch({
        deviceId: device.device_id,
        type: gesture.type,
        payload: gesture.payload as unknown as Record<string, unknown>,
        controlLeaseId: lease.control_lease_id,
        idempotencyKey: `cmd_${gesture.type}_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
      });

      if (gesture.type === 'gesture.touch') {
        const { x, y } = gesture.payload;
        addLog(`Touch accepted at (${x.toFixed(3)}, ${y.toFixed(3)}) - Cmd #${command.command_id}`);
      } else {
        const { startX, startY, endX, endY } = gesture.payload;
        addLog(`Swipe accepted (${startX.toFixed(2)},${startY.toFixed(2)})->(${endX.toFixed(2)},${endY.toFixed(2)}) - Cmd #${command.command_id}`);
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Command dispatch failed';
      addLog(`Command Error: ${msg}`);
    }
  };

  // Pointer Event Handlers using Video Content Geometry Engine
  const handlePointerDown = (e: React.PointerEvent<HTMLVideoElement | HTMLCanvasElement>) => {
    const targetEl = videoRef.current || (e.target as HTMLElement);
    const activeGeom = geometry || computeVideoGeometry(targetEl);
    const rect = targetEl.getBoundingClientRect();
    const position = { clientX: e.clientX, clientY: e.clientY };

    const accepted = gestureRecognizerRef.current.onPointerDown(e.pointerId, position, rect, activeGeom);
    if (accepted) {
      try {
        (e.target as HTMLElement).setPointerCapture?.(e.pointerId);
      } catch {
        // Ignore pointer capture errors in synthetic/test environments
      }
    }
  };

  const handlePointerMove = (e: React.PointerEvent<HTMLVideoElement | HTMLCanvasElement>) => {
    const targetEl = videoRef.current || (e.target as HTMLElement);
    const activeGeom = geometry || computeVideoGeometry(targetEl);
    const rect = targetEl.getBoundingClientRect();
    const position = { clientX: e.clientX, clientY: e.clientY };
    gestureRecognizerRef.current.onPointerMove(e.pointerId, position, rect, activeGeom);
  };

  const handlePointerUp = (e: React.PointerEvent<HTMLVideoElement | HTMLCanvasElement>) => {
    lastPointerUpTimeRef.current = Date.now();
    const targetEl = videoRef.current || (e.target as HTMLElement);
    const activeGeom = geometry || computeVideoGeometry(targetEl);
    const rect = targetEl.getBoundingClientRect();
    const position = { clientX: e.clientX, clientY: e.clientY };

    const gesture = gestureRecognizerRef.current.onPointerUp(e.pointerId, position, rect, activeGeom);
    if (gesture) {
      dispatchGestureCommand(gesture);
    }
  };

  const handlePointerCancel = (_e: React.PointerEvent<HTMLVideoElement | HTMLCanvasElement>) => {
    gestureRecognizerRef.current.cancelCurrentGesture();
  };

  const handleClickFallback = (e: React.MouseEvent<HTMLVideoElement | HTMLCanvasElement>) => {
    if (Date.now() - lastPointerUpTimeRef.current < 200) {
      return; // Ignore fallback click if pointerup already dispatched gesture
    }
    const targetEl = videoRef.current || (e.target as HTMLElement);
    const activeGeom = geometry || computeVideoGeometry(targetEl);
    if (!activeGeom) return;
    const rect = targetEl.getBoundingClientRect();
    const position = { clientX: e.clientX, clientY: e.clientY };
    const point = mapPointerToNormalizedCoordinates(position, rect, activeGeom);
    if (point) {
      dispatchGestureCommand({
        type: 'gesture.touch',
        payload: {
          x: point.x,
          y: point.y,
          coordinateSpace: 'normalized_display_v1',
          orientation: activeGeom.orientation,
        },
      });
    }
  };

  // Dispatch Global Hard Keys
  const sendKey = async (keyName: string) => {
    if (!lease || !session) {
      addLog('Cannot send key: CONTROL_LEASE_REQUIRED');
      return;
    }
    const typeMap: Record<string, DeviceCommandType> = {
      BACK: 'global.back',
      HOME: 'global.home',
      RECENTS: 'global.recents',
    };
    const cmdType = typeMap[keyName] || 'global.back';

    try {
      const command = await commandService.dispatch({
        deviceId: device.device_id,
        type: cmdType,
        payload: { key: keyName },
        controlLeaseId: lease.control_lease_id,
        idempotencyKey: `key_${keyName}_${Date.now()}`,
      });
      addLog(`Sent key: ${keyName} - Cmd #${command.command_id}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Key command failed';
      addLog(`Key error: ${msg}`);
    }
  };

  // Dispatch Text Input
  const sendTextInput = async () => {
    if (!lease || !session) {
      addLog('Cannot send text: CONTROL_LEASE_REQUIRED');
      return;
    }
    if (!textPayload.trim()) return;

    try {
      const command = await commandService.dispatch({
        deviceId: device.device_id,
        type: 'input.text',
        payload: { text: textPayload },
        controlLeaseId: lease.control_lease_id,
        idempotencyKey: `text_${Date.now()}`,
      });
      addLog(`Typed text: "${textPayload}" - Cmd #${command.command_id}`);
      setTextPayload('');
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Type text failed';
      addLog(`Text error: ${msg}`);
    }
  };

  if (!isOpen) return null;

  const displaySessionId = activeServerSessionId || viewerSessionId;
  const isLandscape = geometry?.orientation === 'landscape';

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
                <span className="px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 font-bold text-[10px] border border-blue-500/20">
                  {webrtcState === 'VIDEO_RECEIVING' ? '● LIVE 30fps' : webrtcState}
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono">
                {device.model} ({device.android_version}) • Geometry: {geometry ? `${geometry.videoWidth}x${geometry.videoHeight} (${geometry.orientation})` : '720x1280'}
              </p>
            </div>
          </div>

          <button
            onClick={() => {
              releaseLease();
              onClose();
            }}
            className="p-2 text-slate-400 hover:text-white bg-slate-800/60 hover:bg-slate-800 rounded-full transition-all"
          >
            <X size={18} />
          </button>
        </div>

        {/* Modal Body Grid */}
        <div className="grid grid-cols-1 md:grid-cols-12 overflow-hidden flex-1">
          {/* Stream Player Column */}
          <div
            ref={containerRef}
            className="md:col-span-6 bg-slate-950 p-6 flex flex-col items-center justify-center border-b md:border-b-0 md:border-r border-slate-800/80 relative"
          >
            <div
              className={`relative rounded-3xl overflow-hidden shadow-2xl border-4 border-slate-800 bg-black w-full max-w-[320px] transition-all duration-300 ${
                isLandscape ? 'aspect-[16/9]' : 'aspect-[9/16]'
              }`}
            >
              <video
                ref={videoRef}
                autoPlay
                playsInline
                muted
                onLoadedMetadata={updateGeometry}
                onLoadedData={() => {
                  updateGeometry();
                  mediaClientRef.current?.getWebRtcClient?.()?.notifyVideoFrameReceived();
                }}
                onPlaying={() => {
                  updateGeometry();
                  mediaClientRef.current?.getWebRtcClient?.()?.notifyVideoFrameReceived();
                }}
                onPointerDown={handlePointerDown}
                onPointerMove={handlePointerMove}
                onPointerUp={handlePointerUp}
                onPointerCancel={handlePointerCancel}
                onClick={handleClickFallback}
                className="w-full h-full cursor-crosshair object-contain bg-black touch-none select-none"
              />
              <canvas
                ref={canvasRef}
                width={geometry?.videoWidth || 360}
                height={geometry?.videoHeight || 640}
                onPointerDown={handlePointerDown}
                onPointerMove={handlePointerMove}
                onPointerUp={handlePointerUp}
                onPointerCancel={handlePointerCancel}
                onClick={handleClickFallback}
                data-session-id={displaySessionId}
                className="absolute inset-0 w-full h-full cursor-crosshair object-contain touch-none select-none"
              />

              {webrtcState === 'FAILED' && (
                <div className="absolute inset-0 bg-slate-950/90 flex flex-col items-center justify-center p-4 text-center space-y-2 z-20">
                  <AlertCircle size={32} className="text-rose-500" />
                  <p className="text-xs font-bold text-rose-400">WebRTC Media Error</p>
                  <p className="text-[11px] text-slate-400 max-w-[200px] font-mono">{webrtcError || 'Media connection failed'}</p>
                </div>
              )}

              {!lease && (
                <div
                  className="absolute inset-0 bg-slate-950/60 backdrop-blur-[2px] flex flex-col items-center justify-center p-4 text-center space-y-3 cursor-pointer z-10"
                  onClick={acquireLease}
                >
                  <Shield size={32} className="text-amber-400 animate-bounce" />
                  <p className="text-xs font-bold text-slate-200">Interactive Control Lock</p>
                  <p className="text-[11px] text-slate-400 max-w-[180px]">
                    Lấy quyền Control Lease để tương tác cảm ứng và phát lệnh tới thiết bị.
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
              <Wifi size={12} className="text-emerald-400" /> Session: {displaySessionId}
            </p>
          </div>

          {/* Control Panel Column */}
          <div className="md:col-span-6 p-6 space-y-5 bg-slate-900/50 flex flex-col overflow-y-auto">
            {/* Lease Status Card */}
            <div className="bg-slate-800/60 border border-slate-800 rounded-2xl p-4 flex items-center justify-between">
              <div className="space-y-1">
                <span className="text-[10px] font-extrabold text-slate-400 uppercase tracking-wider">
                  Quyền Điều Khiển Backend
                </span>
                {lease ? (
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
                    <span className="text-xs font-extrabold text-emerald-400">
                      Active ({leaseSecondsLeft}s còn lại • Token #{lease.fencing_token})
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
                  <Clock size={12} /> Real-Time HTTP Command Audit Log
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
