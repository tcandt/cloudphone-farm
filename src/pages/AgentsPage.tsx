import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { QRCodeSVG } from 'qrcode.react';
import { ShieldCheck, Plus, Copy, Check, QrCode } from 'lucide-react';
import { mockAgents } from '../data/mockData';
import { EnrollmentToken } from '../types';
import { enrollmentService } from '../services/enrollment-service';
import { PermissionGuard } from '../components/common/PermissionGuard';

export const AgentsPage: React.FC = () => {
  const { t } = useTranslation();
  const [tokens, setTokens] = useState<EnrollmentToken[]>([]);
  const [activeTokenModal, setActiveTokenModal] = useState<EnrollmentToken | null>(null);
  const [copied, setCopied] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState<number>(600);

  useEffect(() => {
    enrollmentService.listTokens().then(setTokens);
  }, []);

  useEffect(() => {
    if (!activeTokenModal) return;

    const timer = setInterval(() => {
      const remaining = Math.max(
        0,
        Math.floor((new Date(activeTokenModal.expires_at).getTime() - Date.now()) / 1000)
      );
      setSecondsLeft(remaining);
      if (remaining <= 0) {
        setActiveTokenModal(null);
      }
    }, 1000);

    return () => clearInterval(timer);
  }, [activeTokenModal]);

  const handleGenerateToken = async () => {
    const newToken = await enrollmentService.createToken();
    setTokens((prev) => [newToken, ...prev]);
    setActiveTokenModal(newToken);
    setSecondsLeft(600);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('agents.title')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quản lý mã đăng ký Android APK Agent 1 lần và giám sát phiên kết nối
          </p>
        </div>

        <PermissionGuard permission="agent.enroll">
          <button
            onClick={handleGenerateToken}
            className="flex items-center gap-2 px-4 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all active:scale-95"
          >
            <Plus size={16} /> {t('agents.createToken')}
          </button>
        </PermissionGuard>
      </div>

      {/* Connected Agents Table */}
      <div className="bg-white border border-slate-200/80 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
          <h3 className="font-extrabold text-slate-900 text-sm flex items-center gap-2">
            <ShieldCheck size={16} className="text-emerald-500" /> Danh sách APK Agent đang hoạt động ({mockAgents.length})
          </h3>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50 border-b border-slate-100 text-[11px] font-bold text-slate-500 uppercase tracking-wider">
                <th className="px-6 py-3.5">Agent ID</th>
                <th className="px-6 py-3.5">Thiết bị Bound</th>
                <th className="px-6 py-3.5">Phiên bản APK</th>
                <th className="px-6 py-3.5">Fingerprint Key</th>
                <th className="px-6 py-3.5">Trạng thái</th>
                <th className="px-6 py-3.5">Heartbeat Cuối</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {mockAgents.map((agent) => (
                <tr key={agent.agent_id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="px-6 py-4 font-mono font-bold text-slate-900">{agent.agent_id}</td>
                  <td className="px-6 py-4 font-mono text-blue-600 font-semibold">{agent.device_id}</td>
                  <td className="px-6 py-4">
                    <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-700 font-mono text-[11px]">
                      v{agent.app_version}
                    </span>
                  </td>
                  <td className="px-6 py-4 font-mono text-slate-500 text-[11px]">
                    {agent.public_key_fingerprint.substring(0, 16)}...
                  </td>
                  <td className="px-6 py-4">
                    <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold bg-emerald-50 text-emerald-700 border border-emerald-200">
                      <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span> Active
                    </span>
                  </td>
                  <td className="px-6 py-4 text-slate-500 font-mono">
                    {new Date(agent.last_heartbeat_at).toLocaleTimeString('vi-VN')}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* One-Time Token Modal */}
      {activeTokenModal && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-3xl shadow-2xl border border-slate-100 w-full max-w-md p-6 space-y-6 animate-fadeIn">
            <div className="text-center space-y-2">
              <div className="w-12 h-12 mx-auto rounded-2xl bg-blue-50 text-blue-600 border border-blue-100 flex items-center justify-center">
                <QrCode size={24} />
              </div>
              <h3 className="font-extrabold text-slate-900 text-lg">{t('agents.tokenModalTitle')}</h3>
              <p className="text-xs text-slate-500">{t('agents.tokenNotice')}</p>
            </div>

            {/* QR Code Container */}
            <div className="flex flex-col items-center justify-center p-4 bg-slate-50 rounded-2xl border border-slate-100">
              <div className="p-3 bg-white rounded-2xl shadow-md border border-slate-200">
                <QRCodeSVG value={activeTokenModal.token_code} size={160} level="H" />
              </div>
              <p className="text-[11px] font-bold text-amber-600 mt-3 flex items-center gap-1">
                ⏱ {t('agents.expiresIn')}: {Math.floor(secondsLeft / 60)}m {secondsLeft % 60}s
              </p>
            </div>

            {/* Token String Box */}
            <div className="space-y-1.5">
              <label className="block text-xs font-bold text-slate-700">{t('agents.tokenCodeLabel')}</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  readOnly
                  value={activeTokenModal.token_code}
                  className="flex-1 bg-slate-100 border border-slate-200 rounded-xl px-3 py-2 text-xs font-mono font-bold text-slate-800 focus:outline-none"
                />
                <button
                  onClick={() => copyToClipboard(activeTokenModal.token_code)}
                  className="px-3.5 py-2 bg-slate-900 hover:bg-slate-800 text-white text-xs font-bold rounded-xl flex items-center gap-1.5 transition-colors"
                >
                  {copied ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                  <span>{copied ? 'Copied' : t('agents.copyToken')}</span>
                </button>
              </div>
            </div>

            <button
              onClick={() => setActiveTokenModal(null)}
              className="w-full py-2.5 bg-slate-100 hover:bg-slate-200 text-slate-700 font-bold text-xs rounded-xl transition-colors"
            >
              Đóng cửa sổ
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
