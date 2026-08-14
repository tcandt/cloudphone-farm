import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Play, Loader2 } from 'lucide-react';
import { DeviceEntity } from '../types';
import { DeviceControlModal } from '../components/devices/DeviceControlModal';
import { deviceService } from '../services/device-service';

const DeviceGridTile: React.FC<{ device: DeviceEntity; onOpenControl: (dev: DeviceEntity) => void }> = ({
  device,
  onOpenControl,
}) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let animId: number;

    const renderTile = () => {
      const w = canvas.width || 180;
      const h = canvas.height || 320;

      ctx.fillStyle = '#0f172a';
      ctx.fillRect(0, 0, w, h);

      // Header status bar
      ctx.fillStyle = 'rgba(255, 255, 255, 0.15)';
      ctx.fillRect(0, 0, w, 20);
      ctx.fillStyle = '#ffffff';
      ctx.font = '9px sans-serif';
      ctx.fillText(`${device.telemetry?.battery ?? 98}%`, w - 30, 14);

      // Grid wallpaper simulation
      ctx.fillStyle = '#1e293b';
      ctx.fillRect(0, 20, w, h - 20);

      // Draw device name on screen
      ctx.fillStyle = '#38bdf8';
      ctx.font = 'bold 12px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText(device.display_name || device.name || 'Device', w / 2, h / 2 - 10);

      ctx.fillStyle = '#94a3b8';
      ctx.font = '10px sans-serif';
      ctx.fillText(`${device.model} • Android ${device.platform_version || device.android_version}`, w / 2, h / 2 + 10);

      // FPS badge
      ctx.fillStyle = 'rgba(34, 197, 94, 0.2)';
      ctx.fillRect(6, 26, 65, 16);
      ctx.fillStyle = '#4ade80';
      ctx.font = '9px monospace';
      ctx.textAlign = 'left';
      ctx.fillText(`30 FPS`, 10, 38);

      animId = requestAnimationFrame(renderTile);
    };

    renderTile();

    return () => {
      cancelAnimationFrame(animId);
    };
  }, [device]);

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-2xl p-3 flex flex-col items-center shadow-lg hover:border-blue-500/50 transition-all group relative overflow-hidden">
      {/* Tile Header */}
      <div className="w-full flex items-center justify-between mb-2">
        <span className="text-xs font-bold text-slate-200 truncate max-w-[110px]">
          {device.display_name || device.name}
        </span>
        <span
          className={`w-2 h-2 rounded-full ${
            device.status === 'online' ? 'bg-emerald-500 shadow-sm shadow-emerald-500' : 'bg-slate-500'
          }`}
        />
      </div>

      {/* Screen Canvas Container */}
      <div className="relative w-full aspect-[9/16] bg-slate-950 rounded-xl overflow-hidden border border-slate-800/80">
        <canvas ref={canvasRef} width={180} height={320} className="w-full h-full object-contain" />

        {/* Hover Overlay Actions */}
        <div className="absolute inset-0 bg-slate-950/70 backdrop-blur-xs opacity-0 group-hover:opacity-100 flex flex-col items-center justify-center gap-2 transition-all">
          <button
            onClick={() => onOpenControl(device)}
            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-bold shadow-md flex items-center gap-1.5"
          >
            <Play className="w-3.5 h-3.5" />
            <span>Điều khiển</span>
          </button>
        </div>
      </div>
    </div>
  );
};

export const DeviceGridPage: React.FC = () => {
  const { t } = useTranslation();
  const [activeControlDevice, setActiveControlDevice] = useState<DeviceEntity | null>(null);
  const [devices, setDevices] = useState<DeviceEntity[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;

    deviceService
      .list()
      .then((res) => {
        if (isMounted) {
          setDevices(res.items);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (isMounted) {
          setError(err.message || 'Failed to load devices');
          setLoading(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <div className="space-y-6">
      {/* Title Bar */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('deviceGrid.title')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Xem trực tiếp ma trận màn hình nhiều thiết bị trong một chế độ quan sát duy nhất
          </p>
        </div>
      </div>

      {/* Grid Container */}
      {loading ? (
        <div className="p-16 text-center text-slate-400 font-medium flex items-center justify-center gap-2 bg-white rounded-2xl border border-slate-200">
          <Loader2 className="w-5 h-5 animate-spin text-blue-600" />
          <span>Đang tải màn hình ma trận thiết bị...</span>
        </div>
      ) : error ? (
        <div className="p-16 text-center text-rose-500 font-medium bg-white rounded-2xl border border-slate-200">
          Lỗi tải dữ liệu: {error}
        </div>
      ) : devices.length === 0 ? (
        <div className="p-16 text-center text-slate-400 font-medium bg-white rounded-2xl border border-slate-200">
          Không có thiết bị nào khả dụng.
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
          {devices.map((dev) => (
            <DeviceGridTile key={dev.device_id} device={dev} onOpenControl={setActiveControlDevice} />
          ))}
        </div>
      )}

      {/* Control Modal */}
      {activeControlDevice && (
        <DeviceControlModal device={activeControlDevice} onClose={() => setActiveControlDevice(null)} />
      )}
    </div>
  );
};
