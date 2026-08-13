import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { StopCircle } from 'lucide-react';
import { StreamSession } from '../types';

const mockActiveSessions: StreamSession[] = [
  {
    stream_session_id: 'str_sess_9001',
    device_id: 'dev_s7_001',
    organization_id: 'org_pcp_enterprise_01',
    user_id: 'usr_owner_01',
    profile: { resolution: '480p', fps: 30, bitrate_kbps: 1500 },
    status: 'connected',
    started_at: new Date(Date.now() - 600000).toISOString(),
    expires_at: new Date(Date.now() + 3000000).toISOString(),
  },
  {
    stream_session_id: 'str_sess_9002',
    device_id: 'dev_note8_002',
    organization_id: 'org_pcp_enterprise_01',
    user_id: 'usr_owner_01',
    profile: { resolution: '720p', fps: 30, bitrate_kbps: 2500 },
    status: 'connected',
    started_at: new Date(Date.now() - 1200000).toISOString(),
    expires_at: new Date(Date.now() + 2400000).toISOString(),
  },
];

export const ActiveSessionsPage: React.FC = () => {
  const { t } = useTranslation();
  const [sessions, setSessions] = useState<StreamSession[]>(mockActiveSessions);

  const handleStopSession = (sessionId: string) => {
    setSessions(sessions.filter((s) => s.stream_session_id !== sessionId));
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('nav.activeSessions')}</h1>
        <p className="text-xs text-slate-500 font-medium">
          Theo dõi toàn bộ các phiên WebRTC stream đang hoạt động trong Organization và giải phóng băng thông khi cần
        </p>
      </div>

      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50/70 border-b border-slate-100 text-[11px] font-extrabold uppercase text-slate-400 tracking-wider">
                <th className="p-4">Session ID</th>
                <th className="p-4">Device ID</th>
                <th className="p-4">Profile Quality</th>
                <th className="p-4">Trạng thái</th>
                <th className="p-4">Thời gian bắt đầu</th>
                <th className="p-4 text-right">Thao tác</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {sessions.map((sess) => (
                <tr key={sess.stream_session_id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="p-4 font-mono font-bold text-purple-600">{sess.stream_session_id}</td>
                  <td className="p-4 font-mono text-slate-900">{sess.device_id}</td>
                  <td className="p-4">
                    <span className="font-extrabold text-slate-900">{sess.profile.resolution}</span> @ {sess.profile.fps}fps ({sess.profile.bitrate_kbps} kbps)
                  </td>
                  <td className="p-4">
                    <span className="px-2.5 py-1 rounded-full bg-emerald-50 text-emerald-700 font-bold text-[11px] border border-emerald-200 inline-block">
                      CONNECTED
                    </span>
                  </td>
                  <td className="p-4 text-slate-500">{new Date(sess.started_at).toLocaleTimeString('vi-VN')}</td>
                  <td className="p-4 text-right">
                    <button
                      onClick={() => handleStopSession(sess.stream_session_id)}
                      className="px-3 py-1.5 bg-rose-50 hover:bg-rose-100 text-rose-700 font-bold text-xs rounded-xl transition-colors inline-flex items-center gap-1"
                    >
                      <StopCircle size={14} /> Dừng Session
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
