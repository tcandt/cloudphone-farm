import React, { useState, useEffect } from 'react';
import { agentKeyService } from '../../services/agent-key-service';
import { AgentKeyBinding } from '../../types';
import { X, Smartphone, Clock } from 'lucide-react';

interface BindingsDrawerProps {
  keyId: string;
  onClose: () => void;
}

export const BindingsDrawer: React.FC<BindingsDrawerProps> = ({ keyId, onClose }) => {
  const [bindings, setBindings] = useState<AgentKeyBinding[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    agentKeyService.getBindings(keyId)
      .then(setBindings)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [keyId]);

  return (
    <>
      <div className="fixed inset-0 bg-slate-900/20 backdrop-blur-sm z-40" onClick={onClose} />
      <div className="fixed inset-y-0 right-0 w-80 sm:w-96 bg-white shadow-2xl z-50 flex flex-col border-l border-slate-100 transform transition-transform">
        <div className="p-4 border-b border-slate-100 flex items-center justify-between bg-slate-50">
          <div className="flex items-center gap-2 text-slate-800">
            <Smartphone className="w-5 h-5" />
            <h3 className="font-bold">Lịch sử thiết bị</h3>
          </div>
          <button onClick={onClose} className="p-1.5 hover:bg-slate-200 rounded-lg text-slate-500 transition-colors">
            <X size={20} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 space-y-3 bg-slate-50/50">
          {loading ? (
            <div className="text-center text-slate-500 text-sm py-8 animate-pulse">Đang tải lịch sử...</div>
          ) : bindings.length === 0 ? (
            <div className="text-center text-slate-400 text-sm py-8">Chưa có thiết bị nào sử dụng token này</div>
          ) : (
            bindings.map(b => (
              <div key={b.binding_id} className="bg-white p-3 rounded-xl border border-slate-200 shadow-sm transition-all hover:shadow-md">
                <div className="flex items-center justify-between mb-2">
                  <span className="font-mono text-sm font-bold text-slate-700 truncate" title={b.device_id}>
                    {b.device_id.substring(0, 8)}...
                  </span>
                  {b.released_at ? (
                    <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-slate-100 text-slate-500 uppercase">
                      Đã ngắt
                    </span>
                  ) : (
                    <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-700 uppercase flex items-center gap-1 border border-emerald-200">
                      <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" /> Active
                    </span>
                  )}
                </div>
                <div className="text-[11px] text-slate-500 space-y-1.5">
                  <div className="flex items-center gap-1.5">
                    <Clock size={12} className="text-slate-400" />
                    <span>Kết nối: {new Date(b.bound_at).toLocaleString('vi-VN')}</span>
                  </div>
                  {b.released_at && (
                    <div className="flex items-center gap-1.5">
                      <Clock size={12} className="text-slate-400" />
                      <span>Ngắt: {new Date(b.released_at).toLocaleString('vi-VN')}</span>
                    </div>
                  )}
                  {b.release_reason && (
                    <div className="mt-1.5 pt-1.5 border-t border-slate-100 text-amber-600 font-medium">
                      Lý do ngắt: {b.release_reason}
                    </div>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </>
  );
};
