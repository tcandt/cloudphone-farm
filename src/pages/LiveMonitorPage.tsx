import React from 'react';
import { useTranslation } from 'react-i18next';
import { Eye, ShieldCheck, AlertCircle, Grid, Activity } from 'lucide-react';
import { mockDevices } from '../data/mockData';
import { defaultMediaClient } from '../services/media-client';

export const LiveMonitorPage: React.FC = () => {
  const { t } = useTranslation();

  return (
    <div className="space-y-6">
      {/* Title */}
      <div>
        <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">Authorized LIVE Monitor Console</h1>
        <p className="text-xs text-slate-500 font-medium">
          Quan sát thời gian thực nhiều luồng màn hình thiết bị dành cho mục đích QA, Lab và Hỗ trợ được ủy quyền
        </p>
      </div>

      {/* Authorized Consent Banner */}
      <div className="p-4 bg-blue-50 border border-blue-200 rounded-3xl flex items-start gap-3 text-xs text-blue-900 leading-relaxed">
        <ShieldCheck size={20} className="text-blue-600 flex-shrink-0 mt-0.5" />
        <div>
          <p className="font-extrabold text-blue-950">Chế độ Quan sát được Ủy quyền (Authorized Compliance)</p>
          <p className="text-[11px] text-blue-800/90 mt-0.5">
            Tính năng LIVE Monitor tuân thủ nghiêm ngặt quy định bảo mật của Phone Control Platform. Mọi phiên xem màn hình đều được ghi nhận Audit Log theo đúng consent của Organization Owner.
          </p>
        </div>
      </div>

      {/* Stream Grid Preview */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {mockDevices.slice(0, 3).map((device) => (
          <div
            key={device.device_id}
            className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-5 space-y-3"
          >
            <div className="flex items-center justify-between">
              <span className="font-extrabold text-xs text-slate-900">{device.display_name}</span>
              <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 font-bold text-[10px]">
                LIVE 30fps
              </span>
            </div>

            <div className="bg-slate-950 rounded-2xl aspect-[9/16] flex items-center justify-center p-2">
              <div className="text-center text-slate-400 space-y-1">
                <Eye size={24} className="mx-auto text-blue-400" />
                <p className="text-xs font-bold text-slate-200">{device.model}</p>
                <p className="text-[10px] font-mono">Stream Active (480p)</p>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
