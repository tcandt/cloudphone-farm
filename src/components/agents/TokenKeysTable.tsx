import React, { useState, useEffect } from 'react';
import { AgentKey } from '../../types';
import { agentKeyService } from '../../services/agent-key-service';
import { Key, Trash2, Smartphone, Edit2, History } from 'lucide-react';
import { PermissionGuard } from '../common/PermissionGuard';
import { EditTokenKeyModal } from './EditTokenKeyModal';
import { BindingsDrawer } from './BindingsDrawer';

export const TokenKeysTable: React.FC<{ refreshTrigger: number }> = ({ refreshTrigger }) => {
  const [keys, setKeys] = useState<AgentKey[]>([]);
  const [loading, setLoading] = useState(true);
  
  const [editingToken, setEditingToken] = useState<AgentKey | null>(null);
  const [viewingBindingsFor, setViewingBindingsFor] = useState<string | null>(null);

  const fetchKeys = async () => {
    try {
      setLoading(true);
      const data = await agentKeyService.listKeys();
      setKeys(data);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchKeys();
  }, [refreshTrigger]);

  const handleRevoke = async (keyId: string) => {
    if (!window.confirm('Bạn có chắc chắn muốn thu hồi Token này? Việc này sẽ ngăn các agent mới dùng token này để đăng ký, nhưng không ảnh hưởng tới các agent đã đăng ký.')) {
      return;
    }
    try {
      await agentKeyService.revokeKey(keyId);
      fetchKeys();
    } catch (e) {
      alert('Lỗi khi thu hồi: ' + e);
    }
  };

  if (loading) {
    return <div className="p-8 text-center text-slate-500 font-medium text-sm animate-pulse">Đang tải Token Keys...</div>;
  }

  return (
    <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
      <div className="p-4 border-b border-slate-100 flex items-center justify-between bg-slate-50">
        <div className="flex items-center gap-2">
          <Key className="w-5 h-5 text-blue-600" />
          <h2 className="text-sm font-bold text-slate-900">Danh sách Token Keys</h2>
        </div>
        <span className="text-xs font-semibold text-slate-500">Tổng số: {keys.length}</span>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="bg-slate-50 border-b border-slate-200 text-[11px] font-bold text-slate-500 uppercase tracking-wider">
              <th className="py-3 px-4">Tên / Prefix</th>
              <th className="py-3 px-4">Trạng thái</th>
              <th className="py-3 px-4">Đã sử dụng / Quota</th>
              <th className="py-3 px-4">Ngày hết hạn</th>
              <th className="py-3 px-4 text-right">Thao tác</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-xs">
            {keys.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-8 text-center text-slate-400">
                  Chưa có Token Key nào được tạo.
                </td>
              </tr>
            ) : (
              keys.map((k) => {
                // eslint-disable-next-line react-hooks/purity
                const now = Date.now();
                const isRevoked = !!k.revoked_at;
                const isExpired = k.expires_at ? new Date(k.expires_at).getTime() < now : false;
                const isActive = !isRevoked && !isExpired;

                return (
                  <tr key={k.key_id} className="hover:bg-slate-50/80 transition-colors">
                    <td className="py-3.5 px-4">
                      <div className="font-bold text-slate-800">{k.name}</div>
                      <div className="font-mono text-[10px] text-slate-500 mt-0.5">{k.token_prefix}...</div>
                    </td>
                    <td className="py-3.5 px-4">
                      {isActive ? (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase bg-emerald-50 text-emerald-700 border border-emerald-200">
                          <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                          Hoạt động
                        </span>
                      ) : isRevoked ? (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase bg-red-50 text-red-700 border border-red-200">
                          <span className="w-1.5 h-1.5 rounded-full bg-red-500" />
                          Đã thu hồi
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase bg-amber-50 text-amber-700 border border-amber-200">
                          <span className="w-1.5 h-1.5 rounded-full bg-amber-500" />
                          Hết hạn
                        </span>
                      )}
                    </td>
                    <td className="py-3.5 px-4">
                      <div className="flex items-center gap-1.5">
                        <Smartphone size={14} className="text-slate-400" />
                        <span className="font-mono font-bold text-slate-700">
                          {k.active_bindings} / {k.max_bindings ?? '∞'}
                        </span>
                      </div>
                    </td>
                    <td className="py-3.5 px-4 text-slate-600">
                      {k.expires_at ? new Date(k.expires_at).toLocaleString('vi-VN') : 'Không bao giờ'}
                    </td>
                    <td className="py-3.5 px-4 text-right space-x-1">
                      <button
                        onClick={() => setViewingBindingsFor(k.key_id)}
                        className="p-1.5 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                        title="Xem thiết bị đã kết nối"
                      >
                        <History size={16} />
                      </button>
                      
                      {isActive && (
                        <PermissionGuard permission="agent.enroll">
                          <button
                            onClick={() => setEditingToken(k)}
                            className="p-1.5 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                            title="Chỉnh sửa Token"
                          >
                            <Edit2 size={16} />
                          </button>
                        </PermissionGuard>
                      )}
                      
                      {isActive && (
                        <PermissionGuard permission="agent.revoke">
                          <button
                            onClick={() => handleRevoke(k.key_id)}
                            className="p-1.5 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                            title="Thu hồi Token"
                          >
                            <Trash2 size={16} />
                          </button>
                        </PermissionGuard>
                      )}
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {editingToken && (
        <EditTokenKeyModal
          token={editingToken}
          onClose={() => setEditingToken(null)}
          onSuccess={() => {
            setEditingToken(null);
            fetchKeys();
          }}
        />
      )}

      {viewingBindingsFor && (
        <BindingsDrawer
          keyId={viewingBindingsFor}
          onClose={() => setViewingBindingsFor(null)}
        />
      )}
    </div>
  );
};
