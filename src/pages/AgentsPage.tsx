import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { QRCodeSVG } from 'qrcode.react';
import {
  ShieldCheck,
  PlusCircle,
  Copy,
  Clock,
  Check,
  Smartphone,
  Key,
  X,
  Sparkles,
} from 'lucide-react';
import { mockAgents, mockEnrollmentTokens } from '../data/mockData';
import { EnrollmentToken } from '../types';
import { PermissionGuard } from '../components/common/PermissionGuard';

export const AgentsPage: React.FC = () => {
  const { t } = useTranslation();
  const [tokens, setTokens] = useState<EnrollmentToken[]>(mockEnrollmentTokens);
  const [activeTokenModal, setActiveTokenModal] = useState<EnrollmentToken | null>(null);
  const [copied, setCopied] = useState(false);

  const handleGenerateToken = async () => {
    // Contract mapping: POST /api/v1/agent-enrollments
    const newTokenCode = `PCP-ENROLL-${Math.floor(1000 + Math.random() * 9000)}-${Math.random()
      .toString(36)
      .substring(2, 6)
      .toUpperCase()}`;

    const newToken: EnrollmentToken = {
      token_id: `tok_${Math.random().toString(36).substring(2, 8)}`,
      organization_id: 'org_pcp_enterprise_01',
      token_code: newTokenCode,
      created_by: 'usr_owner_01',
      expires_at: new Date(Date.now() + 600 * 1000).toISOString(), // 10 mins TTL
      used: false,
    };

    setTokens([newToken, ...tokens]);
    setActiveTokenModal(newToken);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('agents.title')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quản lý ứng dụng PCP Agent APK trên các thiết bị và phát hành mã Enroll 1 lần
          </p>
        </div>

        <PermissionGuard permission="agent.enroll">
          <button
            onClick={handleGenerateToken}
            className="px-4 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 hover:opacity-95 transition-all flex items-center gap-2 active:scale-95"
          >
            <PlusCircle size={16} /> {t('agents.createToken')} (POST /agent-enrollments)
          </button>
        </PermissionGuard>
      </div>

      {/* Agents Table */}
      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="p-4 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between">
          <h2 className="text-sm font-extrabold text-slate-900 flex items-center gap-2">
            <ShieldCheck size={18} className="text-blue-600" /> Danh sách Android Agent đang kết nối
          </h2>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50/70 border-b border-slate-100 text-[11px] font-extrabold uppercase text-slate-400 tracking-wider">
                <th className="p-4">Agent ID</th>
                <th className="p-4">Device ID</th>
                <th className="p-4">Phiên bản APK</th>
                <th className="p-4">Fingerprint Keystore</th>
                <th className="p-4">Trạng thái</th>
                <th className="p-4">Heartbeat gần nhất</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {mockAgents.map((agt) => (
                <tr key={agt.agent_id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="p-4 font-mono font-bold text-blue-600">{agt.agent_id}</td>
                  <td className="p-4 font-mono text-slate-800">{agt.device_id}</td>
                  <td className="p-4 font-bold text-slate-900">{agt.app_version}</td>
                  <td className="p-4 font-mono text-[11px] text-slate-500 truncate max-w-[200px]">
                    {agt.public_key_fingerprint}
                  </td>
                  <td className="p-4">
                    <span className="px-2.5 py-1 rounded-full bg-emerald-50 text-emerald-700 font-bold text-[11px] border border-emerald-200 inline-block">
                      ACTIVE
                    </span>
                  </td>
                  <td className="p-4 text-slate-500">{new Date(agt.last_heartbeat_at).toLocaleTimeString('vi-VN')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Token Modal */}
      {activeTokenModal && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white border border-slate-100 shadow-2xl rounded-3xl w-full max-w-md p-6 space-y-5 animate-fadeIn">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
                <Sparkles size={18} className="text-amber-500" /> {t('agents.tokenModalTitle')}
              </h2>
              <button onClick={() => setActiveTokenModal(null)} className="p-1.5 text-slate-400 hover:text-slate-700">
                <X size={18} />
              </button>
            </div>

            <p className="text-xs text-slate-600 leading-relaxed bg-blue-50/60 border border-blue-100 p-3 rounded-2xl">
              {t('agents.tokenNotice')}
            </p>

            {/* QR Code Container */}
            <div className="p-4 bg-slate-50 border border-slate-200 rounded-2xl flex flex-col items-center justify-center space-y-3">
              <QRCodeSVG value={activeTokenModal.token_code} size={180} level="H" includeMargin />
              <div className="flex items-center gap-1.5 text-xs font-bold text-amber-600">
                <Clock size={14} /> <span>Hết hạn trong 10 phút (TTL)</span>
              </div>
            </div>

            {/* String Code */}
            <div className="space-y-1">
              <label className="block text-[11px] font-extrabold uppercase text-slate-400">
                {t('agents.tokenCodeLabel')}
              </label>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  readOnly
                  value={activeTokenModal.token_code}
                  className="flex-1 px-3.5 py-2.5 bg-slate-100 border border-slate-200 rounded-xl font-mono font-bold text-sm text-slate-900 outline-none select-all"
                />
                <button
                  onClick={() => copyToClipboard(activeTokenModal.token_code)}
                  className="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-md transition-colors flex items-center gap-1.5"
                >
                  {copied ? <Check size={16} /> : <Copy size={16} />}
                  <span>{copied ? 'Đã chép' : 'Sao chép'}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
