import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { QRCodeSVG } from 'qrcode.react';
import { ShieldCheck, Plus, Copy, Check, QrCode } from 'lucide-react';
import { EnrollmentToken } from '../types';
import { enrollmentService } from '../services/enrollment-service';
import { agentService, AgentItem } from '../services/agent-service';
import { PermissionGuard } from '../components/common/PermissionGuard';

export const AgentsPage: React.FC = () => {
  const { t } = useTranslation();
  const [_tokens, setTokens] = useState<EnrollmentToken[]>([]);
  const [agents, setAgents] = useState<AgentItem[]>([]);
  const [activeTokenModal, setActiveTokenModal] = useState<EnrollmentToken | null>(null);
  const [copied, setCopied] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState<number>(600);

  useEffect(() => {
    enrollmentService.listTokens().then(setTokens).catch(() => {});
    agentService.listAgents().then(setAgents).catch(() => {});
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
            className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-xl text-xs shadow-sm transition-all"
          >
            <Plus size={16} />
            <span>Tạo Mã Đăng Ký Agent</span>
          </button>
        </PermissionGuard>
      </div>

      {/* Agents Table */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        <div className="p-4 border-b border-slate-100 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <ShieldCheck className="w-5 h-5 text-blue-600" />
            <h2 className="text-sm font-bold text-slate-900">Danh sách APK Agents đã Đăng ký</h2>
          </div>
          <span className="text-xs font-semibold text-slate-500">Tổng số: {agents.length}</span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50 border-b border-slate-200 text-[11px] font-bold text-slate-500 uppercase tracking-wider">
                <th className="py-3 px-4">Agent ID</th>
                <th className="py-3 px-4">Device ID</th>
                <th className="py-3 px-4">Phiên bản APK</th>
                <th className="py-3 px-4">Trạng thái</th>
                <th className="py-3 px-4">Thời gian xác thực gần nhất</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs">
              {agents.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-slate-400">
                    Chưa có Agent nào được đăng ký.
                  </td>
                </tr>
              ) : (
                agents.map((agent) => (
                  <tr key={agent.agent_id} className="hover:bg-slate-50/80 transition-colors">
                    <td className="py-3.5 px-4 font-mono font-bold text-slate-800">{agent.agent_id}</td>
                    <td className="py-3.5 px-4 font-mono text-slate-600">{agent.device_id}</td>
                    <td className="py-3.5 px-4 font-semibold text-slate-700">{agent.apk_version}</td>
                    <td className="py-3.5 px-4">
                      <span
                        className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase ${
                          agent.status === 'active'
                            ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                            : 'bg-slate-100 text-slate-600 border border-slate-200'
                        }`}
                      >
                        <span
                          className={`w-1.5 h-1.5 rounded-full ${
                            agent.status === 'active' ? 'bg-emerald-500' : 'bg-slate-400'
                          }`}
                        />
                        {agent.status}
                      </span>
                    </td>
                    <td className="py-3.5 px-4 text-slate-500">
                      {agent.last_authenticated_at
                        ? new Date(agent.last_authenticated_at).toLocaleString('vi-VN')
                        : 'Vừa xong'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Modal QR Token */}
      {activeTokenModal && (
        <div className="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50 animate-fadeIn">
          <div className="bg-white rounded-3xl p-6 max-w-md w-full shadow-2xl space-y-5 border border-slate-100">
            <div className="flex items-center justify-between border-b border-slate-100 pb-3">
              <div className="flex items-center gap-2">
                <QrCode className="w-5 h-5 text-blue-600" />
                <h3 className="text-base font-extrabold text-slate-900">Mã Đăng Ký Agent Một Lần</h3>
              </div>
              <button
                onClick={() => setActiveTokenModal(null)}
                className="text-slate-400 hover:text-slate-600 text-xs font-bold"
              >
                Đóng
              </button>
            </div>

            <div className="flex flex-col items-center justify-center space-y-4 py-2">
              <div className="p-4 bg-slate-50 border border-slate-200 rounded-2xl shadow-inner">
                <QRCodeSVG value={activeTokenModal.token_code} size={180} />
              </div>

              <div className="w-full space-y-1 text-center">
                <p className="text-xs text-slate-500 font-medium">Mã Token (Nhập trên APK/Agent):</p>
                <div className="flex items-center justify-center gap-2 bg-slate-100 p-2.5 rounded-xl border border-slate-200">
                  <span className="font-mono text-sm font-extrabold text-blue-900 tracking-wider">
                    {activeTokenModal.token_code}
                  </span>
                  <button
                    onClick={() => copyToClipboard(activeTokenModal.token_code)}
                    className="p-1 hover:bg-slate-200 rounded-lg text-slate-600 transition-colors"
                  >
                    {copied ? <Check size={16} className="text-emerald-600" /> : <Copy size={16} />}
                  </button>
                </div>
              </div>

              <div className="text-xs font-semibold text-amber-600 bg-amber-50 px-3 py-1.5 rounded-xl border border-amber-200">
                Mã sẽ hết hạn sau {Math.floor(secondsLeft / 60)}m {secondsLeft % 60}s
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
