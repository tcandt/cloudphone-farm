import React, { useEffect, useRef, useState } from 'react';
import { ShieldCheck, Video, VideoOff } from 'lucide-react';
import { mockDevices } from '../data/mockData';
import { defaultMediaRegistry, MediaClient } from '../services/media-client';
import { DeviceEntity } from '../types';

interface SingleStreamCardProps {
  device: DeviceEntity;
}

const SingleStreamCard: React.FC<SingleStreamCardProps> = ({ device }) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [isStreaming, setIsStreaming] = useState(true);
  const [sessionId] = useState(() => `str_live_${device.device_id}_${Math.random().toString(36).substring(2, 7)}`);
  const mediaClientRef = useRef<MediaClient | null>(null);

  useEffect(() => {
    if (!isStreaming) return;

    const mediaClient = defaultMediaRegistry.acquire(sessionId);
    mediaClientRef.current = mediaClient;
    let mounted = true;

    async function start() {
      await mediaClient.startSession(device.device_id, {
        resolution: '480p',
        fps: 30,
        bitrate_kbps: 1200,
      });

      if (mounted && canvasRef.current) {
        mediaClient.attach(canvasRef.current);
      }
    }

    start();

    return () => {
      mounted = false;
      defaultMediaRegistry.release(sessionId);
    };
  }, [device.device_id, isStreaming, sessionId]);

  const toggleStream = () => {
    setIsStreaming((prev) => !prev);
  };

  return (
    <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-5 space-y-3" data-testid={`stream-card-${device.device_id}`}>
      <div className="flex items-center justify-between">
        <div>
          <span className="font-extrabold text-xs text-slate-900 block">{device.display_name}</span>
          <span className="text-[10px] text-slate-400 font-mono">ID: {device.device_id}</span>
        </div>
        <div className="flex items-center gap-2">
          {isStreaming ? (
            <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 font-bold text-[10px] flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span> LIVE 30fps
            </span>
          ) : (
            <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-500 font-bold text-[10px]">
              STOPPED
            </span>
          )}
          <button
            onClick={toggleStream}
            className="p-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 transition-colors"
            title={isStreaming ? 'Stop Stream' : 'Start Stream'}
          >
            {isStreaming ? <VideoOff size={14} /> : <Video size={14} />}
          </button>
        </div>
      </div>

      <div className="bg-slate-950 rounded-2xl aspect-[9/16] flex items-center justify-center p-2 relative overflow-hidden">
        {isStreaming ? (
          <canvas
            ref={canvasRef}
            width={360}
            height={640}
            data-session-id={sessionId}
            className="w-full h-full object-contain"
          />
        ) : (
          <div className="text-center text-slate-500 space-y-1">
            <VideoOff size={24} className="mx-auto text-slate-600" />
            <p className="text-xs font-bold text-slate-400">Stream Paused</p>
          </div>
        )}
      </div>

      <p className="text-[10px] text-slate-400 font-mono truncate">Session: {sessionId}</p>
    </div>
  );
};

export const LiveMonitorPage: React.FC = () => {
  const liveDevices = mockDevices.slice(0, 2);

  return (
    <div className="space-y-6">
      {/* Title */}
      <div>
        <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">Authorized LIVE Monitor Console</h1>
        <p className="text-xs text-slate-500 font-medium">
          Quan sát thời gian thực nhiều luồng màn hình thiết bị độc lập dành cho mục đích QA, Lab và Hỗ trợ được ủy quyền
        </p>
      </div>

      {/* Authorized Consent Banner */}
      <div className="p-4 bg-blue-50 border border-blue-200 rounded-3xl flex items-start gap-3 text-xs text-blue-900 leading-relaxed">
        <ShieldCheck size={20} className="text-blue-600 flex-shrink-0 mt-0.5" />
        <div>
          <p className="font-extrabold text-blue-950">Chế độ Quan sát Độc lập (Dual Independent Streams)</p>
          <p className="text-[11px] text-blue-800/90 mt-0.5">
            Tính năng LIVE Monitor khởi tạo các phiên WebRTC MediaClient độc lập với Session ID riêng biệt. Việc dừng hoặc khởi chạy lại một luồng hoàn toàn không gây ảnh hưởng đến các phiên quan sát khác.
          </p>
        </div>
      </div>

      {/* Multi-Stream Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 max-w-4xl">
        {liveDevices.map((device) => (
          <SingleStreamCard key={device.device_id} device={device} />
        ))}
      </div>
    </div>
  );
};
