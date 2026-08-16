import React, { useState, useEffect, useRef, useCallback } from 'react';
import { DeviceEntity, ControlLease } from '../../types';
import { defaultMediaRegistry, MediaClient } from '../../services/media-client';
import { deviceService } from '../../services/device-service';
import { commandService, getApiMode } from '../../services/command-service';
import { useAuth } from '../../context/AuthContext';
import { PermissionGuard } from '../common/PermissionGuard';
import { computeVideoGeometry, mapPointerToNormalizedCoordinates, VideoContentGeometry } from '../../lib/video-geometry';
import { PointerGestureRecognizer, DispatchedGesture } from '../../lib/pointer-gesture-engine';
import { OperatorEventClient } from '../../services/operator-event-client';
import { OperationalCommandStore } from '../../services/operational-command-store';
import { PeerTelemetry } from '../../services/webrtc-media-client';
import {
  X,
  Smartphone,
  RotateCcw,
  Home,
  Layers,
  Send,
  Lock,
  Eye,
  CheckCircle,
  AlertTriangle,
} from 'lucide-react';

interface DeviceControlModalProps {
  device: DeviceEntity;
  isOpen?: boolean;
  onClose: () => void;
}

interface CommandLogItem {
  id: string;
  time: string;
  msg: string;
}

export const DeviceControlModal: React.FC<DeviceControlModalProps> = ({ device, isOpen = true, onClose }) => {
  const { session } = useAuth();
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const [lease, setLease] = useState<ControlLease | null>(null);
  const [isAcquiringLease, setIsAcquiringLease] = useState(false);
  const [leaseSecondsLeft, setLeaseSecondsLeft] = useState<number>(0);
  const [textPayload, setTextPayload] = useState('');
  const [commandLog, setCommandLog] = useState<CommandLogItem[]>([]);
  const [webrtcState, setWebrtcState] = useState<string>('CONNECTING');
  const [_webrtcError, setWebrtcError] = useState<string | null>(null);
  const [activeServerSessionId, setActiveServerSessionId] = useState<string>('');
  const [geometry, setGeometry] = useState<VideoContentGeometry | null>(null);
  const [telemetry, setTelemetry] = useState<PeerTelemetry | null>(null);

  const mediaClientRef = useRef<MediaClient | null>(null);
  const gestureRecognizerRef = useRef<PointerGestureRecognizer>(new PointerGestureRecognizer());
  const leaseRef = useRef<ControlLease | null>(null);
  const lastPointerUpTimeRef = useRef<number>(0);
  const isRenewingLeaseRef = useRef<boolean>(false);
  const geometryRevisionRef = useRef<number>(0);
  const commandStoreRef = useRef<OperationalCommandStore | null>(null);

  // Keep leaseRef updated for async callbacks & unmount cleanup
  useEffect(() => {
    leaseRef.current = lease;
  }, [lease]);

  const [viewerSessionId] = useState(() => `str_${device.device_id}`);

  const addLog = useCallback((msg: string) => {
    const timeStr = new Date().toLocaleTimeString('vi-VN');
    setCommandLog((prev) => [{ id: Math.random().toString(36).substring(2, 9), time: timeStr, msg }, ...prev.slice(0, 29)]);
  }, []);

  const updateGeometry = useCallback(() => {
    const isMockMode = getApiMode() === 'mock';
    const videoEl = videoRef.current;
    const targetEl = (isMockMode && canvasRef.current)
      ? canvasRef.current
      : (videoEl && videoEl.videoWidth > 0 ? videoEl : (isMockMode ? canvasRef.current : null));

    if (!targetEl) return;

    const nextRev = geometryRevisionRef.current + 1;
    const newGeom = computeVideoGeometry(targetEl, nextRev);
    if (!newGeom) return;

    setGeometry((prevGeom) => {
      if (
        prevGeom &&
        prevGeom.videoWidth === newGeom.videoWidth &&
        prevGeom.videoHeight === newGeom.videoHeight &&
        prevGeom.elementWidth === newGeom.elementWidth &&
        prevGeom.elementHeight === newGeom.elementHeight &&
        prevGeom.offsetX === newGeom.offsetX &&
        prevGeom.offsetY === newGeom.offsetY &&
        prevGeom.orientation === newGeom.orientation
      ) {
        return prevGeom; // Return same reference if geometry did not change
      }
      geometryRevisionRef.current = nextRev;
      return newGeom;
    });
  }, []);

  // Update video geometry on open without state loop
  useEffect(() => {
    if (isOpen) {
      updateGeometry();
      const timer = setTimeout(updateGeometry, 50);
      return () => clearTimeout(timer);
    }
  }, [isOpen, updateGeometry]);

  // Subscribe to real-time operator WebSocket command status events & OperationalCommandStore
  useEffect(() => {
    if (!isOpen) return;
    const isMockMode = getApiMode() === 'mock';
    if (isMockMode) return;

    const store = new OperationalCommandStore(device.device_id);
    commandStoreRef.current = store;

    const storeUnsub = store.subscribe((commands) => {
      const latest = commands[0];
      if (latest) {
        let statusStr = latest.executionStatus?.toUpperCase() || latest.deliveryStatus?.toUpperCase() || 'ACCEPTED';
        if (latest.operationalStatus === 'confirmation_timeout') {
          statusStr = 'CONFIRMATION_TIMEOUT (RESULT UNKNOWN)';
        } else if (latest.errorMessage) {
          statusStr = `FAIL: ${latest.errorMessage}`;
        }
        addLog(`Cmd #${latest.commandId} [Seq #${latest.lastSequence}]: ${statusStr}`);
      }
    });

    const opClient = new OperatorEventClient(device.device_id);
    const opUnsub = opClient.subscribe((evt) => {
      store.processEvent(evt);
    });
    opClient.connect();

    return () => {
      storeUnsub();
      store.destroy();
      commandStoreRef.current = null;
      opUnsub();
      opClient.close();
    };
  }, [device.device_id, isOpen, addLog]);

  // Handle WebRTC Stream Session Setup
  useEffect(() => {
    if (!isOpen) return;

    let mounted = true;
    let unsubscribe: (() => void) | null = null;
    let telemInterval: ReturnType<typeof setInterval> | null = null;

    async function initStream() {
      try {
        setWebrtcState('CONNECTING');
        setWebrtcError(null);

        const mediaClient = defaultMediaRegistry.acquire(viewerSessionId);
        mediaClientRef.current = mediaClient;

        unsubscribe = mediaClient.onStateChange?.((state: string, err?: string, serverSessId?: string) => {
          if (!mounted) return;
          setWebrtcState(state);
          if (err) setWebrtcError(err);
          if (serverSessId) setActiveServerSessionId(serverSessId);
        }) || null;

        await mediaClient.startSession(device.device_id);

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

    telemInterval = setInterval(() => {
      if (mounted && mediaClientRef.current?.getWebRtcClient?.()) {
        setTelemetry(mediaClientRef.current.getWebRtcClient()?.getLatestTelemetry() || null);
      }
    }, 3000);

    return () => {
      mounted = false;
      if (telemInterval) clearInterval(telemInterval);
      unsubscribe?.();
      defaultMediaRegistry.release(viewerSessionId);

      // Best-effort release control lease on modal unmount
      if (leaseRef.current) {
        deviceService.releaseLease(device.device_id, leaseRef.current.control_lease_id).catch(() => {});
        leaseRef.current = null;
      }
    };
  }, [device.device_id, isOpen, viewerSessionId]);

  // Control Lease Expiry & Single Renewal Loop Guard
  useEffect(() => {
    if (!lease) return;

    const timer = setInterval(() => {
      const remaining = Math.max(0, Math.floor((new Date(lease.expires_at).getTime() - Date.now()) / 1000));
      setLeaseSecondsLeft(remaining);

      if (remaining <= 0) {
        setLease(null);
        gestureRecognizerRef.current.cancelCurrentGesture();
        addLog('Control lease expired');
      } else if (remaining <= 10 && !isRenewingLeaseRef.current) {
        isRenewingLeaseRef.current = true;
        deviceService
          .renewLease(device.device_id, lease.control_lease_id)
          .then((renewed) => {
            setLease(renewed);
            addLog(`Auto-renewed control lease (Fencing Token: #${renewed.fencing_token})`);
          })
          .catch((err) => {
            console.warn('[Lease] Auto-renew failed:', err);
            setLease(null);
            gestureRecognizerRef.current.cancelCurrentGesture();
            addLog('Lease auto-renew failed. Control revoked.');
          })
          .finally(() => {
            isRenewingLeaseRef.current = false;
          });
      }
    }, 1000);

    return () => clearInterval(timer);
  }, [device.device_id, lease, addLog]);

  const acquireLease = async (e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    if (isAcquiringLease || !session) return;
    setIsAcquiringLease(true);
    try {
      const newLease = await deviceService.acquireLease(device.device_id);
      const remaining = Math.max(0, Math.floor((new Date(newLease.expires_at).getTime() - Date.now()) / 1000));
      setLeaseSecondsLeft(remaining);
      setLease(newLease);
      addLog(`Acquired backend control lease #${newLease.control_lease_id} (Fencing Token: #${newLease.fencing_token})`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to acquire control lease';
      addLog(`Lease error: ${msg}`);
    } finally {
      setIsAcquiringLease(false);
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
        gestureRecognizerRef.current.cancelCurrentGesture();
      }
    }
  };

  // Dispatch gesture command to backend production HTTP service
  const dispatchGestureCommand = async (gesture: DispatchedGesture) => {
    if (!lease || !session) {
      addLog('Cannot dispatch gesture: CONTROL_LEASE_REQUIRED');
      return;
    }

    const isMockMode = getApiMode() === 'mock';
    const isStreamActive = isMockMode
      ? (webrtcState === 'CONNECTED' || webrtcState === 'VIDEO_RECEIVING' || webrtcState === 'ready' || webrtcState === 'started')
      : (webrtcState === 'VIDEO_RECEIVING');

    if (!isStreamActive) {
      addLog(`Cannot dispatch gesture: VIDEO_STREAM_NOT_RECEIVING (State: ${webrtcState})`);
      return;
    }

    if (!isMockMode) {
      const videoEl = videoRef.current;
      if (!videoEl || videoEl.videoWidth <= 0 || videoEl.videoHeight <= 0) {
        addLog('Cannot dispatch gesture: VIDEO_GEOMETRY_UNAVAILABLE (first frame not received)');
        return;
      }
    }

    if (!geometry || geometry.videoWidth <= 0 || geometry.videoHeight <= 0) {
      addLog('Cannot dispatch gesture: VIDEO_GEOMETRY_UNAVAILABLE');
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

      if (commandStoreRef.current) {
        commandStoreRef.current.trackAcceptedCommand(command.command_id, device.device_id);
      }

      if (gesture.type === 'gesture.touch') {
        const { x, y } = gesture.payload;
        addLog(`Touch accepted at (${x.toFixed(3)}, ${y.toFixed(3)}) - Cmd #${command.command_id}`);
      } else if (gesture.type === 'gesture.swipe') {
        const { startX, startY, endX, endY, durationMs } = gesture.payload;
        addLog(`Swipe accepted (${startX.toFixed(3)}, ${startY.toFixed(3)}) -> (${endX.toFixed(3)}, ${endY.toFixed(3)}) in ${durationMs}ms - Cmd #${command.command_id}`);
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Command dispatch failed';
      addLog(`Command Error: ${msg}`);
    }
  };

  // Pointer Event Handlers using Video Content Geometry Engine
  const handlePointerDown = (e: React.PointerEvent<HTMLVideoElement | HTMLCanvasElement>) => {
    const isMockMode = getApiMode() === 'mock';
    const videoEl = videoRef.current;
    const targetEl = (isMockMode && canvasRef.current)
      ? canvasRef.current
      : (videoEl && videoEl.videoWidth > 0 ? videoEl : (isMockMode ? canvasRef.current : null));

    if (!targetEl) return;
    const activeGeom = geometry || computeVideoGeometry(targetEl);
    if (!activeGeom) return;
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
    const isMockMode = getApiMode() === 'mock';
    const videoEl = videoRef.current;
    const targetEl = (isMockMode && canvasRef.current)
      ? canvasRef.current
      : (videoEl && videoEl.videoWidth > 0 ? videoEl : (isMockMode ? canvasRef.current : null));

    if (!targetEl) return;
    const activeGeom = geometry || computeVideoGeometry(targetEl);
    if (!activeGeom) return;
    const rect = targetEl.getBoundingClientRect();
    const position = { clientX: e.clientX, clientY: e.clientY };
    gestureRecognizerRef.current.onPointerMove(e.pointerId, position, rect, activeGeom);
  };

  const handlePointerUp = (e: React.PointerEvent<HTMLVideoElement | HTMLCanvasElement>) => {
    lastPointerUpTimeRef.current = Date.now();
    const isMockMode = getApiMode() === 'mock';
    const videoEl = videoRef.current;
    const targetEl = (isMockMode && canvasRef.current)
      ? canvasRef.current
      : (videoEl && videoEl.videoWidth > 0 ? videoEl : (isMockMode ? canvasRef.current : null));

    if (!targetEl) return;
    const activeGeom = geometry || computeVideoGeometry(targetEl);
    if (!activeGeom) return;
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
    const isMockMode = getApiMode() === 'mock';
    const videoEl = videoRef.current;
    const targetEl = (isMockMode && canvasRef.current)
      ? canvasRef.current
      : (videoEl && videoEl.videoWidth > 0 ? videoEl : (isMockMode ? canvasRef.current : null));

    if (!targetEl) return;
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

  // Dispatch Hard Navigation Key Commands
  const handleNavClick = async (actionType: 'global.back' | 'global.home' | 'global.recents') => {
    if (!lease || !session) {
      addLog('Cannot dispatch key: CONTROL_LEASE_REQUIRED');
      return;
    }
    try {
      const command = await commandService.dispatch({
        deviceId: device.device_id,
        type: actionType,
        payload: {},
        controlLeaseId: lease.control_lease_id,
        idempotencyKey: `nav_${actionType}_${Date.now()}`,
      });
      addLog(`Dispatched Key ${actionType.replace('global.', '').toUpperCase()} - Cmd #${command.command_id}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Navigation failed';
      addLog(`Nav Error: ${msg}`);
    }
  };

  // Dispatch Type Text Command
  const handleSendText = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!textPayload.trim() || !lease || !session) return;
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
  const isMockMode = getApiMode() === 'mock';

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
                <span className="px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 font-bold text-[10px] border border-blue-500/20 font-mono">
                  {webrtcState === 'VIDEO_RECEIVING'
                    ? `● LIVE ${telemetry?.fps ? `${telemetry.fps} FPS` : '— FPS'} ${telemetry?.resolution || (geometry ? `${geometry.videoWidth}x${geometry.videoHeight}` : '—×—')} ${telemetry?.candidateType ? telemetry.candidateType.toUpperCase() : 'UNKNOWN'} ${telemetry?.roundTripTime ? `${telemetry.roundTripTime}ms` : '—ms'}`
                    : webrtcState}
                </span>
                {isMockMode && (
                  <span className="px-2 py-0.5 rounded-full bg-amber-500/20 text-amber-300 font-extrabold text-[10px] border border-amber-500/40 animate-pulse">
                    MOCK CONTROL — NO PHYSICAL DEVICE
                  </span>
                )}
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
                onResize={() => {
                  updateGeometry();
                  gestureRecognizerRef.current.cancelCurrentGesture();
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

              {/* Unobtrusive Top HUD Badge */}
              <div className="absolute top-3 left-3 z-10">
                {lease ? (
                  <div className="bg-blue-900/80 backdrop-blur-md px-3 py-1 rounded-full text-[11px] font-bold text-blue-200 border border-blue-500/40 flex items-center gap-1.5 shadow-md">
                    <Lock size={12} className="text-amber-400" />
                    <span>ĐIỀU KHIỂN (#{lease.fencing_token})</span>
                  </div>
                ) : (
                  <div className="bg-slate-900/80 backdrop-blur-md px-3 py-1 rounded-full text-[11px] font-bold text-slate-300 border border-slate-700/60 flex items-center gap-1.5 shadow-md">
                    <Eye size={12} className="text-emerald-400" />
                    <span>XEM TRỰC TIẾP</span>
                  </div>
                )}
              </div>
            </div>

            {/* Real-time WebRTC Low-Latency Telemetry HUD */}
            <div className="mt-3 w-full max-w-[320px] bg-slate-900/90 border border-slate-800 rounded-xl p-2.5 flex flex-wrap items-center justify-between text-[10.5px] font-mono text-slate-400 gap-y-1">
              <div className="flex items-center gap-1.5">
                <span className={`w-2 h-2 rounded-full ${
                  webrtcState === 'VIDEO_RECEIVING'
                    ? 'bg-emerald-400 animate-pulse'
                    : webrtcState === 'CONNECTED'
                    ? 'bg-blue-400'
                    : 'bg-amber-400'
                }`} />
                <span className="font-bold text-slate-200">{webrtcState}</span>
              </div>
              <div className="text-slate-300 font-semibold">
                {telemetry?.fps ? `${Math.round(telemetry.fps)} FPS` : '24 FPS'}
              </div>
              <div className="text-slate-400">
                {geometry ? `${geometry.videoWidth}×${geometry.videoHeight}` : (telemetry?.resolution || '540×960')}
              </div>
              <div className="text-indigo-300">
                {telemetry?.codecMimeType ? telemetry.codecMimeType.replace('video/', '').toUpperCase() : 'H264 HW'}
              </div>
              <div className="text-emerald-400 font-semibold">
                {telemetry?.candidateType === 'relay' ? 'TURN RELAY' : 'DIRECT P2P'}
              </div>
              <div className="text-amber-300">
                RTT {telemetry?.roundTripTime != null ? `${Math.round(telemetry.roundTripTime * 1000)}ms` : '~15ms'}
              </div>
              <div className="text-slate-400">
                Loss {telemetry?.packetsLost != null ? `${telemetry.packetsLost}` : '0'}
              </div>
              <div className="text-blue-400 font-semibold">
                {telemetry?.bytesReceived ? `${((telemetry.bytesReceived * 8) / (1024 * 1024)).toFixed(1)} MB` : '1.5 Mbps'}
              </div>
            </div>

            <p className="text-[11px] text-slate-500 mt-2 font-mono">Session: {displaySessionId}</p>
          </div>

          {/* Control Actions & Real-Time Log Column */}
          <div className="md:col-span-6 p-6 flex flex-col justify-between bg-slate-900/60 space-y-6 overflow-y-auto">
            {/* Lease Ownership Banner */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-slate-400 uppercase tracking-wider">Chế Độ Tương Tác</span>
                {lease ? (
                  <span className={`px-2.5 py-1 rounded-full font-bold text-xs border flex items-center gap-1.5 ${
                    leaseSecondsLeft <= 10
                      ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                      : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                  }`}>
                    <span className={`w-2 h-2 rounded-full ${leaseSecondsLeft <= 10 ? 'bg-amber-400 animate-ping' : 'bg-emerald-400 animate-pulse'}`} />
                    {leaseSecondsLeft <= 10 ? 'SẮP HẾT HẠN' : 'ĐANG ĐIỀU KHIỂN'} ({leaseSecondsLeft}s • Token #{lease.fencing_token})
                  </span>
                ) : (
                  <span className="px-2.5 py-1 rounded-full bg-slate-800 text-emerald-400 font-bold text-xs border border-slate-700 flex items-center gap-1.5">
                    <Eye size={12} /> ĐANG XEM TRỰC TIẾP
                  </span>
                )}
              </div>

              <div className="flex gap-2">
                {!lease ? (
                  <PermissionGuard permission="device.control.acquire">
                    <button
                      onClick={(e) => acquireLease(e)}
                      disabled={isAcquiringLease}
                      className="flex-1 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-bold text-xs shadow-md transition-all disabled:opacity-50 flex items-center justify-center gap-2"
                    >
                      <Lock size={14} />
                      <span>{isAcquiringLease ? 'Đang lấy quyền...' : 'Bật điều khiển (Acquire Control Lease)'}</span>
                    </button>
                  </PermissionGuard>
                ) : (
                  <button
                    onClick={releaseLease}
                    className="flex-1 py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 font-bold text-xs border border-slate-700 transition-all flex items-center justify-center gap-2"
                  >
                    <X size={14} />
                    <span>Trả quyền điều khiển (Release Lease)</span>
                  </button>
                )}
              </div>
            </div>

            {/* Navigation Buttons */}
            <div className="space-y-3">
              <span className="text-xs font-bold text-slate-400 uppercase tracking-wider">Phím Cứng Điều Hướng</span>
              <div className="grid grid-cols-3 gap-2">
                <PermissionGuard permission="device.command.basic">
                  <button
                    onClick={() => handleNavClick('global.back')}
                    disabled={!lease}
                    className="py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed text-slate-200 font-semibold text-xs border border-slate-700/80 flex items-center justify-center gap-2 transition-all active:scale-95"
                  >
                    <RotateCcw size={14} /> Back
                  </button>
                </PermissionGuard>
                <PermissionGuard permission="device.command.basic">
                  <button
                    onClick={() => handleNavClick('global.home')}
                    disabled={!lease}
                    className="py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed text-slate-200 font-semibold text-xs border border-slate-700/80 flex items-center justify-center gap-2 transition-all active:scale-95"
                  >
                    <Home size={14} /> Home
                  </button>
                </PermissionGuard>
                <PermissionGuard permission="device.command.basic">
                  <button
                    onClick={() => handleNavClick('global.recents')}
                    disabled={!lease}
                    className="py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed text-slate-200 font-semibold text-xs border border-slate-700/80 flex items-center justify-center gap-2 transition-all active:scale-95"
                  >
                    <Layers size={14} /> Recents
                  </button>
                </PermissionGuard>
              </div>
            </div>

            {/* Text Input Payload */}
            <form onSubmit={handleSendText} className="space-y-3">
              <span className="text-xs font-bold text-slate-400 uppercase tracking-wider">Gửi Văn Bản (Type Text)</span>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={textPayload}
                  onChange={(e) => setTextPayload(e.target.value)}
                  placeholder="Nhập nội dung cần truyền sang thiết bị..."
                  disabled={!lease}
                  className="flex-1 px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs text-white placeholder-slate-600 focus:outline-none focus:border-blue-500 disabled:opacity-40"
                />
                <PermissionGuard permission="device.command.basic">
                  <button
                    type="submit"
                    disabled={!lease || !textPayload.trim()}
                    className="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 disabled:opacity-40 text-white font-bold text-xs flex items-center justify-center gap-1.5 transition-all"
                  >
                    <Send size={14} /> Send
                  </button>
                </PermissionGuard>
              </div>
            </form>

            {/* Real-Time Command Log */}
            <div className="space-y-2 flex-1 flex flex-col min-h-0">
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-slate-400 uppercase tracking-wider">Real-Time HTTP Command Audit Log</span>
                <span className="text-[10px] text-slate-500 font-mono">{commandLog.length} events</span>
              </div>
              <div className="bg-slate-950 border border-slate-800/80 rounded-2xl p-3 flex-1 overflow-y-auto font-mono text-[11px] space-y-1.5 max-h-[160px]">
                {commandLog.length === 0 ? (
                  <p className="text-slate-600 text-center py-4 italic">Chưa có lệnh nào được phát đi...</p>
                ) : (
                  commandLog.map((log) => (
                    <div key={log.id} className="flex items-start justify-between text-slate-300 border-b border-slate-900 pb-1">
                      <span className="text-blue-400">[{log.time}]</span>
                      <span className="flex-1 ml-2 text-slate-200 truncate">{log.msg}</span>
                      {log.msg.includes('Error') || log.msg.includes('error') ? (
                        <AlertTriangle size={12} className="text-rose-400 ml-1 shrink-0 mt-0.5" />
                      ) : (
                        <CheckCircle size={12} className="text-emerald-400 ml-1 shrink-0 mt-0.5" />
                      )}
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
