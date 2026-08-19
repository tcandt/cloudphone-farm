import React, { useState } from 'react';
import { agentKeyService } from '../../services/agent-key-service';
import { Edit2, X } from 'lucide-react';
import { AgentKey } from '../../types';

interface EditTokenKeyModalProps {
  token: AgentKey;
  onClose: () => void;
  onSuccess: () => void;
}

export const EditTokenKeyModal: React.FC<EditTokenKeyModalProps> = ({ token, onClose, onSuccess }) => {
  const [name, setName] = useState(token.name);
  const [maxBindingsStr, setMaxBindingsStr] = useState(token.max_bindings?.toString() || '');
  const [maxBindingsUnlimited, setMaxBindingsUnlimited] = useState(token.max_bindings === null);
  
  const [expiresDaysStr, setExpiresDaysStr] = useState('');
  const [expiresAtForever, setExpiresAtForever] = useState(token.expires_at === null);
  
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!name.trim()) {
      setError('Tên không được để trống');
      return;
    }

    let maxBindings: number | null | undefined;
    if (maxBindingsUnlimited) {
      maxBindings = null;
    } else if (maxBindingsStr.trim() !== '') {
      maxBindings = parseInt(maxBindingsStr, 10);
      if (isNaN(maxBindings) || maxBindings <= 0) {
        setError('Số thiết bị tối đa phải lớn hơn 0');
        return;
      }
    }

    let expiresAt: string | null | undefined;
    if (expiresAtForever) {
      expiresAt = null;
    } else if (expiresDaysStr.trim() !== '') {
      const days = parseInt(expiresDaysStr, 10);
      if (isNaN(days) || days <= 0) {
        setError('Số ngày gia hạn phải lớn hơn 0');
        return;
      }
      expiresAt = new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString();
    }

    try {
      setLoading(true);
      const payload: { name?: string; max_bindings?: number | null; expires_at?: string | null } = { name };
      if (maxBindings !== undefined) payload.max_bindings = maxBindings;
      if (expiresAt !== undefined) payload.expires_at = expiresAt;

      await agentKeyService.updateKey(token.key_id, payload);
      onSuccess();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message || 'Lỗi khi cập nhật Token Key');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50 animate-fadeIn">
      <div className="bg-white rounded-3xl p-6 max-w-md w-full shadow-2xl space-y-5 border border-slate-100">
        <div className="flex items-center justify-between border-b border-slate-100 pb-3">
          <div className="flex items-center gap-2">
            <Edit2 className="w-5 h-5 text-blue-600" />
            <h3 className="text-lg font-extrabold text-slate-900">Sửa Token Key</h3>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-600 transition-colors"
          >
            <X size={20} />
          </button>
        </div>

        {error && (
          <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded-xl text-sm font-medium">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <label className="text-sm font-bold text-slate-700">Tên Token <span className="text-red-500">*</span></label>
            <input
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              className="w-full px-4 py-2.5 bg-slate-50 border border-slate-200 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 rounded-xl outline-none transition-all text-sm font-medium"
              required
              maxLength={128}
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-bold text-slate-700">Số thiết bị tối đa (Capacity)</label>
            <div className="flex flex-col gap-2">
              <input
                type="number"
                value={maxBindingsStr}
                onChange={e => { setMaxBindingsStr(e.target.value); setMaxBindingsUnlimited(false); }}
                disabled={maxBindingsUnlimited}
                placeholder={maxBindingsUnlimited ? "Không giới hạn" : "Để trống nếu không đổi"}
                className="w-full px-4 py-2.5 bg-slate-50 border border-slate-200 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 rounded-xl outline-none transition-all text-sm font-medium disabled:opacity-50"
                min="1"
              />
              <label className="flex items-center gap-2 text-sm text-slate-700 cursor-pointer">
                <input
                  type="checkbox"
                  checked={maxBindingsUnlimited}
                  onChange={e => {
                    setMaxBindingsUnlimited(e.target.checked);
                    if (e.target.checked) setMaxBindingsStr('');
                  }}
                  className="rounded text-blue-600 focus:ring-blue-500"
                />
                Không giới hạn
              </label>
            </div>
            <p className="text-xs text-slate-500">Số lượng thiết bị tối đa có thể đồng thời sử dụng token này.</p>
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-bold text-slate-700">Gia hạn thêm (Ngày)</label>
            <div className="flex flex-col gap-2">
              <input
                type="number"
                value={expiresDaysStr}
                onChange={e => { setExpiresDaysStr(e.target.value); setExpiresAtForever(false); }}
                disabled={expiresAtForever}
                placeholder={expiresAtForever ? "Không hết hạn" : "Bỏ trống để giữ nguyên"}
                className="w-full px-4 py-2.5 bg-slate-50 border border-slate-200 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 rounded-xl outline-none transition-all text-sm font-medium disabled:opacity-50"
                min="1"
              />
              <label className="flex items-center gap-2 text-sm text-slate-700 cursor-pointer">
                <input
                  type="checkbox"
                  checked={expiresAtForever}
                  onChange={e => {
                    setExpiresAtForever(e.target.checked);
                    if (e.target.checked) setExpiresDaysStr('');
                  }}
                  className="rounded text-blue-600 focus:ring-blue-500"
                />
                Không hết hạn / Forever
              </label>
            </div>
            <p className="text-xs text-slate-500">Nhập số ngày để gia hạn tính từ bây giờ.</p>
          </div>

          <div className="pt-4 flex gap-3 justify-end">
            <button
              type="button"
              onClick={onClose}
              className="px-5 py-2.5 bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 font-bold rounded-xl text-sm transition-colors"
            >
              Hủy
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white font-bold rounded-xl text-sm transition-colors flex items-center gap-2"
            >
              {loading ? 'Đang lưu...' : 'Lưu thay đổi'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
