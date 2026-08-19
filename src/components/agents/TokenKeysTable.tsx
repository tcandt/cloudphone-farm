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
  const [error, setError] = useState<string | null>(null);
  
  const [editingToken, setEditingToken] = useState<AgentKey | null>(null);
  const [viewingBindingsFor, setViewingBindingsFor] = useState<string | null>(null);
  const [revokingToken, setRevokingToken] = useState<AgentKey | null>(null);

  const fetchKeys = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await agentKeyService.listKeys();
      setKeys(data);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchKeys();
  }, [refreshTrigger]);

  const handleRevoke = async () => {
    if (!revokingToken) return;
    try {
      await agentKeyService.revokeKey(revokingToken.key_id);
      fetchKeys();
      setRevokingToken(null);
    } catch (e: unknown) {
      // In a real app we'd show a toast, but we can set error or just let modal handle it
      setError('Lỗi khi thu hồi: ' + (e instanceof Error ? e.message : String(e)));
      setRevokingToken(null);
    }
  };

  if (loading && keys.length === 0) {
    return (
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-8 flex justify-center">
        <div className="text-slate-500 font-medium text-sm animate-pulse">Đang tải Token Keys...</div>
      </div>
    );
  }

  if (error && keys.length === 0) {
    return (
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-8 flex flex-col items-center gap-3">
        <div className="text-red-600 font-medium text-sm">{error}</div>
        <button onClick={fetchKeys} className="px-4 py-2 bg-slate-100 hover:bg-slate-200 text-slate-700 font-bold rounded-lg text-sm transition-colors">
          Thử lại
        </button>
      </div>
    );
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
              <th className="py-3 px-4">Lần sử dụng cuối</th>
              <th className="py-3 px-4">Ngày tạo</th>
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
                          {k.active_bindings} / {k.max_bindings === null ? 'Không giới hạn' : k.max_bindings}
                        </span>
                      </div>
                    </td>
                    <td className="py-3.5 px-4 text-slate-600">
                      {k.expires_at ? new Date(k.expires_at).toLocaleString('vi-VN') : 'Không hết hạn'}
                    </td>
                    <td className="py-3.5 px-4 text-slate-600">
                      {/* TODO: If we had last_used_at we'd display it. Currently we only have active_bindings and created_at. Let's fallback gracefully if API doesn't have it yet. */}
                      Chưa rõ
                    </td>
                    <td className="py-3.5 px-4 text-slate-600">
                      {k.created_at ? new Date(k.created_at).toLocaleString('vi-VN') : ''}
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
                        <PermissionGuard permission="agent.enroll">
                          <button
                            onClick={() => setRevokingToken(k)}
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

      {revokingToken && (
        <div className="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50 animate-fadeIn">
          <div className="bg-white rounded-3xl p-6 max-w-md w-full shadow-2xl space-y-5 border border-slate-100">
            <h3 className="text-lg font-extrabold text-slate-900">Thu hồi Token Key</h3>
            <p className="text-sm text-slate-600">
              Bạn đang thu hồi Token Key <strong>{revokingToken.name}</strong>.
            </p>
            <ul className="text-sm text-slate-600 list-disc list-inside space-y-1">
              <li>Việc đăng ký thiết bị mới với Token này sẽ bị chặn.</li>
              <li>Các Agents và Thiết bị đã đăng ký vẫn tiếp tục hoạt động.</li>
              <li>Các bindings hiện tại sẽ được giữ nguyên.</li>
              <li className="text-red-600 font-bold">Hành động này KHÔNG THỂ HOÀN TÁC.</li>
            </ul>
            <div className="pt-4 flex gap-3 justify-end">
              <button onClick={() => setRevokingToken(null)} className="px-5 py-2.5 bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 font-bold rounded-xl text-sm transition-colors">
                Hủy
              </button>
              <button onClick={handleRevoke} className="px-6 py-2.5 bg-red-600 hover:bg-red-700 text-white font-bold rounded-xl text-sm transition-colors">
                Đồng ý thu hồi
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
