import React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Smartphone,
  Wifi,
  Activity,
  ShieldAlert,
  Zap,
  Grid,
  PlusCircle,
  Clock,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  ArrowRight,
  TrendingUp,
} from 'lucide-react';
import { mockAuditLogs, mockDevices } from '../data/mockData';
import { useNavigate } from 'react-router-dom';
import { PermissionGuard } from '../components/common/PermissionGuard';

export const DashboardPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const totalDevices = mockDevices.length;
  const onlineDevices = mockDevices.filter((d) => d.status === 'online').length;
  const degradedDevices = mockDevices.filter((d) => d.status === 'degraded').length;
  const offlineDevices = mockDevices.filter((d) => d.status === 'offline').length;

  return (
    <div className="space-y-6">
      {/* Hero Welcome & Title */}
      <div className="bg-gradient-to-r from-blue-600 via-indigo-600 to-purple-600 rounded-3xl p-6 md:p-8 text-white shadow-xl shadow-blue-500/10 flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
        <div className="space-y-2 max-w-xl">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/10 backdrop-blur-md text-amber-300 text-xs font-bold uppercase tracking-wider">
            <Zap size={14} /> System Operational Baseline v1.0
          </div>
          <h1 className="text-2xl md:text-3xl font-extrabold tracking-tight">{t('dashboard.title')}</h1>
          <p className="text-xs md:text-sm text-blue-100/90 leading-relaxed">
            {t('dashboard.subtitle')}
          </p>
        </div>

        <div className="flex items-center gap-3 flex-wrap">
          <button
            onClick={() => navigate('/app/devices/grid')}
            className="px-5 py-2.5 rounded-xl bg-white text-blue-700 font-bold text-xs shadow-lg hover:bg-slate-50 transition-all flex items-center gap-2 active:scale-95"
          >
            <Grid size={16} /> Xem Lưới Device Grid
          </button>
          <PermissionGuard permission="agent.enroll">
            <button
              onClick={() => navigate('/app/agents')}
              className="px-5 py-2.5 rounded-xl bg-amber-500 text-white font-bold text-xs shadow-lg hover:bg-amber-600 transition-all flex items-center gap-2 active:scale-95"
            >
              <PlusCircle size={16} /> {t('dashboard.generateEnrollment')}
            </button>
          </PermissionGuard>
        </div>
      </div>

      {/* Metric Cards Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Total Devices */}
        <div className="bg-white border border-slate-100 shadow-pcp-card rounded-2xl p-5 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold uppercase tracking-wider text-slate-400">
              {t('dashboard.totalDevices')}
            </span>
            <div className="p-2 bg-blue-50 text-blue-600 rounded-xl">
              <Smartphone size={20} />
            </div>
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-3xl font-black text-slate-900">{totalDevices}</span>
            <span className="text-xs font-semibold text-emerald-600 flex items-center gap-1">
              <TrendingUp size={12} /> 100% Registered
            </span>
          </div>
        </div>

        {/* Online Devices */}
        <div className="bg-white border border-slate-100 shadow-pcp-card rounded-2xl p-5 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold uppercase tracking-wider text-slate-400">
              {t('dashboard.onlineDevices')}
            </span>
            <div className="p-2 bg-emerald-50 text-emerald-600 rounded-xl">
              <CheckCircle2 size={20} />
            </div>
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-3xl font-black text-emerald-600">{onlineDevices}</span>
            <span className="text-xs font-semibold text-slate-500">
              {Math.round((onlineDevices / totalDevices) * 100)}% Active
            </span>
          </div>
        </div>

        {/* Active Leases */}
        <div className="bg-white border border-slate-100 shadow-pcp-card rounded-2xl p-5 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold uppercase tracking-wider text-slate-400">
              {t('dashboard.activeLeases')}
            </span>
            <div className="p-2 bg-purple-50 text-purple-600 rounded-xl">
              <Activity size={20} />
            </div>
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-3xl font-black text-purple-600">1</span>
            <span className="text-xs font-semibold text-purple-600">60s TTL Lease</span>
          </div>
        </div>

        {/* Latency */}
        <div className="bg-white border border-slate-100 shadow-pcp-card rounded-2xl p-5 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold uppercase tracking-wider text-slate-400">
              {t('dashboard.avgLatency')}
            </span>
            <div className="p-2 bg-amber-50 text-amber-600 rounded-xl">
              <Wifi size={20} />
            </div>
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-3xl font-black text-slate-900">18 <span className="text-sm font-normal text-slate-500">ms</span></span>
            <span className="text-xs font-semibold text-emerald-600">WebRTC P2P</span>
          </div>
        </div>
      </div>

      {/* Main Grid: Status Distribution & Recent Audit */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Device Status Breakdown */}
        <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-5">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-extrabold text-slate-900">{t('dashboard.statusDistribution')}</h2>
            <button
              onClick={() => navigate('/app/devices')}
              className="text-xs font-bold text-blue-600 hover:underline flex items-center gap-1"
            >
              Chi tiết <ArrowRight size={14} />
            </button>
          </div>

          <div className="space-y-3">
            {/* Online Bar */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs font-bold">
                <span className="flex items-center gap-2 text-slate-700">
                  <span className="w-2.5 h-2.5 rounded-full bg-emerald-500" /> Online
                </span>
                <span className="text-slate-900">{onlineDevices} máy</span>
              </div>
              <div className="w-full h-2.5 bg-slate-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-emerald-500 rounded-full transition-all duration-500"
                  style={{ width: `${(onlineDevices / totalDevices) * 100}%` }}
                />
              </div>
            </div>

            {/* Degraded Bar */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs font-bold">
                <span className="flex items-center gap-2 text-slate-700">
                  <span className="w-2.5 h-2.5 rounded-full bg-amber-500" /> Degraded / Lag
                </span>
                <span className="text-slate-900">{degradedDevices} máy</span>
              </div>
              <div className="w-full h-2.5 bg-slate-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-amber-500 rounded-full transition-all duration-500"
                  style={{ width: `${(degradedDevices / totalDevices) * 100}%` }}
                />
              </div>
            </div>

            {/* Offline Bar */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs font-bold">
                <span className="flex items-center gap-2 text-slate-700">
                  <span className="w-2.5 h-2.5 rounded-full bg-rose-500" /> Offline
                </span>
                <span className="text-slate-900">{offlineDevices} máy</span>
              </div>
              <div className="w-full h-2.5 bg-slate-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-rose-500 rounded-full transition-all duration-500"
                  style={{ width: `${(offlineDevices / totalDevices) * 100}%` }}
                />
              </div>
            </div>
          </div>

          <div className="p-4 bg-slate-50 border border-slate-100 rounded-2xl space-y-2">
            <div className="flex items-center gap-2 text-xs font-bold text-slate-800">
              <ShieldAlert size={16} className="text-amber-500" /> Ghi chú phân quyền & Security
            </div>
            <p className="text-[11px] text-slate-600 leading-relaxed">
              Mọi hành động tác động tới thiết bị được ghi nhật ký Audit Log độc lập. Quyền điều khiển remote lease có thời gian hết hạn cố định 60s để tránh chiếm dụng.
            </p>
          </div>
        </div>

        {/* Recent Audit Feed */}
        <div className="lg:col-span-2 bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-5">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-extrabold text-slate-900">{t('dashboard.recentAudit')}</h2>
            <button
              onClick={() => navigate('/app/audit')}
              className="text-xs font-bold text-blue-600 hover:underline flex items-center gap-1"
            >
              Xem toàn bộ <ArrowRight size={14} />
            </button>
          </div>

          <div className="space-y-3">
            {mockAuditLogs.map((log) => (
              <div
                key={log.audit_id}
                className="p-3.5 bg-slate-50/70 hover:bg-slate-100/80 rounded-2xl border border-slate-100 transition-colors flex items-start gap-3"
              >
                <div className="p-2 bg-blue-50 text-blue-600 rounded-xl mt-0.5">
                  <Clock size={16} />
                </div>
                <div className="flex-1 min-w-0 space-y-1">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-bold text-slate-900 truncate">{log.action_code}</span>
                    <span className="text-[10px] font-medium text-slate-400">
                      {new Date(log.created_at).toLocaleTimeString('vi-VN')}
                    </span>
                  </div>
                  <p className="text-xs text-slate-600">{log.details}</p>
                  <div className="flex items-center gap-3 text-[10px] text-slate-400 font-medium">
                    <span>Actor: {log.actor_email}</span>
                    <span>IP: {log.ip_address}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};
