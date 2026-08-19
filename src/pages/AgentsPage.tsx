import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { ShieldCheck, Plus, Key } from 'lucide-react';
import { agentService, AgentItem } from '../services/agent-service';
import { PermissionGuard } from '../components/common/PermissionGuard';
import { TokenKeysTable } from '../components/agents/TokenKeysTable';
import { CreateTokenModal } from '../components/agents/CreateTokenModal';

export const AgentsPage: React.FC = () => {
  const { t } = useTranslation();
  const [agents, setAgents] = useState<AgentItem[]>([]);
  const [activeTab, setActiveTab] = useState<'tokens' | 'agents'>('tokens');
  
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  const [agentsLoading, setAgentsLoading] = useState(false);
  const [agentsError, setAgentsError] = useState<string | null>(null);

  const fetchAgents = () => {
    setAgentsLoading(true);
    setAgentsError(null);
    agentService.listAgents()
      .then(setAgents)
      .catch(err => {
        setAgentsError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => setAgentsLoading(false));
  };

  useEffect(() => {
    if (activeTab === 'agents') {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      fetchAgents();
    }
  }, [activeTab]);

  const handleTokenCreated = () => {
    setRefreshTrigger(prev => prev + 1);
  };

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('agents.title')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quản lý Token Keys đa dụng (V2) và danh sách thiết bị đã đăng ký (Registered Agents)
          </p>
        </div>

        {activeTab === 'tokens' && (
          <PermissionGuard permission="agent.enroll">
            <button
              onClick={() => setShowCreateModal(true)}
              className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-xl text-xs shadow-sm transition-all"
            >
              <Plus size={16} />
              <span>Tạo Token Key Mới</span>
            </button>
          </PermissionGuard>
        )}
      </div>

      {/* Tabs */}
      <div className="flex space-x-1 bg-slate-100/50 p-1 rounded-xl w-fit border border-slate-200">
        <button
          onClick={() => setActiveTab('tokens')}
          className={`flex items-center gap-2 px-4 py-2 text-sm font-bold rounded-lg transition-all ${
            activeTab === 'tokens'
              ? 'bg-white text-blue-700 shadow-sm border border-slate-200/60'
              : 'text-slate-600 hover:text-slate-900 hover:bg-slate-200/50'
          }`}
        >
          <Key size={16} />
          Token Keys
        </button>
        <button
          onClick={() => setActiveTab('agents')}
          className={`flex items-center gap-2 px-4 py-2 text-sm font-bold rounded-lg transition-all ${
            activeTab === 'agents'
              ? 'bg-white text-blue-700 shadow-sm border border-slate-200/60'
              : 'text-slate-600 hover:text-slate-900 hover:bg-slate-200/50'
          }`}
        >
          <ShieldCheck size={16} />
          Registered Agents
        </button>
      </div>

      {/* Tab Content */}
      <div className="animate-fadeIn">
        {activeTab === 'tokens' && (
          <TokenKeysTable refreshTrigger={refreshTrigger} />
        )}

        {activeTab === 'agents' && (
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
                    <th className="py-3 px-4">Thời gian kết nối gần nhất</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 text-xs">
                  {agentsLoading ? (
                    <tr>
                      <td colSpan={5} className="py-8 text-center text-slate-500 font-medium">
                        Đang tải danh sách...
                      </td>
                    </tr>
                  ) : agentsError ? (
                    <tr>
                      <td colSpan={5} className="py-8 text-center text-red-600 font-medium">
                        <div className="flex flex-col items-center gap-2">
                          <span>{agentsError}</span>
                          <button onClick={fetchAgents} className="px-4 py-1.5 bg-red-100 hover:bg-red-200 text-red-700 rounded-lg text-xs font-bold transition-colors">
                            Thử lại
                          </button>
                        </div>
                      </td>
                    </tr>
                  ) : agents.length === 0 ? (
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
        )}
      </div>

      {showCreateModal && (
        <CreateTokenModal
          onClose={() => setShowCreateModal(false)}
          onSuccess={handleTokenCreated}
        />
      )}
    </div>
  );
};
