import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Play, Maximize2 } from 'lucide-react';
import { mockDevices } from '../data/mockData';
import { DeviceEntity } from '../types';
import { useUiStore } from '../stores/useUiStore';
import { DeviceControlModal } from '../components/devices/DeviceControlModal';

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
      ctx.fillText(`98%`, w - 30, 14);

      // Grid wallpaper simulation
      ctx.fillStyle = '#1e293b';
      ctx.fillRect(0, 20, w, h - 20);

      // Draw device name on screen
      ctx.fillStyle = '#38bdf8';
      ctx.font = 'bold 12px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText(device.display_name, w / 2, h / 2 - 10);

      ctx.fillStyle = '#94a3b8';
      ctx.font = '10px sans-serif';
      ctx.fillText(`${device.model} • Android ${device.android_version}`, w / 2, h / 2 + 10);

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
    <div className="bg-white border border-slate-100 shadow-pcp-card rounded-2xl p-3 flex flex-col space-y-2 hover:shadow-lg transition-all">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 truncate">
          <span
            className={`w-2 h-2 rounded-full ${
              device.status === 'online' ? 'bg-emerald-500' : device.status === 'degraded' ? 'bg-amber-500' : 'bg-rose-500'
            }`}
          />
          <span className="font-bold text-xs text-slate-900 truncate">{device.display_name}</span>
        </div>
        <button
          onClick={() => onOpenControl(device)}
          className="p-1 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
          title="Mở bảng điều khiển chi tiết"
        >
          <Maximize2 size={14} />
        </button>
      </div>

      {/* Screen Tile Canvas */}
      <div className="relative rounded-xl overflow-hidden bg-slate-950 aspect-[9/16] flex items-center justify-center">
        <canvas ref={canvasRef} width={180} height={320} className="w-full h-full object-cover" />

        {/* Hover overlay button */}
        <div className="absolute inset-0 bg-slate-900/40 opacity-0 hover:opacity-100 transition-opacity flex items-center justify-center p-2">
          <button
            onClick={() => onOpenControl(device)}
            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-lg flex items-center gap-1.5 active:scale-95"
          >
            <Play size={14} /> Điều khiển
          </button>
        </div>
      </div>

      {/* Footer Specs */}
      <div className="flex items-center justify-between text-[10px] text-slate-500 font-semibold pt-1 border-t border-slate-100">
        <span>{device.telemetry.battery}% ⚡</span>
        <span className="uppercase">{device.telemetry.network}</span>
        <span className="text-emerald-600 font-bold">18ms</span>
      </div>
    </div>
  );
};

export const DeviceGridPage: React.FC = () => {
  const { t } = useTranslation();
  const { gridColumns, setGridColumns } = useUiStore();
  const [activeControlDevice, setActiveControlDevice] = useState<DeviceEntity | null>(null);

  const getGridClass = () => {
    switch (gridColumns) {
      case 2:
        return 'grid-cols-1 sm:grid-cols-2';
      case 4:
        return 'grid-cols-2 sm:grid-cols-4';
      case 5:
        return 'grid-cols-2 sm:grid-cols-5';
      case 3:
      default:
        return 'grid-cols-1 sm:grid-cols-3';
    }
  };

  return (
    <div className="space-y-6">
      {/* Title & Grid Selector */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('deviceGrid.title')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quan sát đồng thời nhiều màn hình thiết bị Android với độ trễ thấp
          </p>
        </div>

        {/* Grid Column Selector */}
        <div className="flex items-center gap-1 bg-white border border-slate-100 p-1.5 rounded-2xl shadow-sm">
          {[2, 3, 4, 5].map((cols) => (
            <button
              key={cols}
              onClick={() => setGridColumns(cols)}
              className={`px-3 py-1.5 rounded-xl text-xs font-bold transition-all ${
                gridColumns === cols
                  ? 'bg-blue-600 text-white shadow-md'
                  : 'text-slate-600 hover:bg-slate-100'
              }`}
            >
              {cols}x{cols}
            </button>
          ))}
        </div>
      </div>

      {/* Grid Canvas Tiles */}
      <div className={`grid gap-4 ${getGridClass()}`}>
        {mockDevices.map((dev) => (
          <DeviceGridTile key={dev.device_id} device={dev} onOpenControl={(d) => setActiveControlDevice(d)} />
        ))}
      </div>

      {/* Control Modal */}
      {activeControlDevice && (
        <DeviceControlModal
          device={activeControlDevice}
          isOpen={!!activeControlDevice}
          onClose={() => setActiveControlDevice(null)}
        />
      )}
    </div>
  );
};
