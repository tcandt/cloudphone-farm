import React, { useState } from 'react';
import { agentKeyService } from '../../services/agent-key-service';
import { Key, Copy, Check, AlertCircle, X } from 'lucide-react';

interface CreateTokenModalProps {
  onClose: () => void;
  onSuccess: () => void;
}

export const CreateTokenModal: React.FC<CreateTokenModalProps> = ({ onClose, onSuccess }) => {
  const [name, setName] = useState('');
  const [maxBindingsStr, setMaxBindingsStr] = useState('');
  const [expiresDaysStr, setExpiresDaysStr] = useState('');
  
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const [rawSecret, setRawSecret] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!name.trim()) {
      setError('Tên không được để trống');
      return;
    }

    let maxBindings: number | null = null;
    if (maxBindingsStr.trim()) {
      maxBindings = parseInt(maxBindingsStr, 10);
      if (isNaN(maxBindings) || maxBindings <= 0) {
        setError('Số thiết bị tối đa phải lớn hơn 0');
        return;
      }
    }

    let expiresAt: string | null = null;
    if (expiresDaysStr.trim()) {
      const days = parseInt(expiresDaysStr, 10);
      if (isNaN(days) || days <= 0) {
        setError('Số ngày hết hạn phải lớn hơn 0');
        return;
      }
      expiresAt = new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString();
    }

    try {
      setLoading(true);
      const res = await agentKeyService.createKey(name, maxBindings, expiresAt);
      setRawSecret(res.raw_secret);
      onSuccess();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message || 'Lỗi khi tạo Token Key');
    } finally {
      setLoading(false);
    }
  };

  const copySecret = () => {
    if (rawSecret) {
      navigator.clipboard.writeText(rawSecret);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleClose = () => {
    setRawSecret(null); // Clear ephemeral raw secret
    onClose();
  };

  if (rawSecret) {
    return (
      <div className="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50 animate-fadeIn">
        <div className="bg-white rounded-3xl p-6 max-w-md w-full shadow-2xl space-y-5 border border-slate-100">
          <div className="flex items-center gap-2 border-b border-slate-100 pb-3">
            <Check className="w-6 h-6 text-emerald-600" />
            <h3 className="text-lg font-extrabold text-slate-900">Tạo Token Thành Công</h3>
          </div>

          <div className="space-y-4">
            <div className="p-3 bg-amber-50 border border-amber-200 rounded-xl flex gap-3 text-amber-800 text-sm">
              <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
              <div>
                <strong className="block mb-1">Cảnh báo bảo mật:</strong>
                Mã bí mật dưới đây chỉ được hiển thị <strong>một lần duy nhất</strong>. Vui lòng copy và lưu trữ an toàn.
              </div>
            </div>

            <div className="space-y-1">
              <label className="text-xs font-bold text-slate-500">Mã Token Bí Mật</label>
              <div className="flex items-center gap-2 bg-slate-50 p-3 rounded-xl border border-slate-200">
                <input
                  type="text"
                  readOnly
                  value={rawSecret}
                  className="flex-1 bg-transparent border-none font-mono text-sm text-slate-800 focus:outline-none"
                />
                <button
                  onClick={copySecret}
                  className="p-2 bg-white border border-slate-200 hover:bg-slate-50 hover:border-slate-300 rounded-lg text-slate-600 transition-colors shadow-sm"
                >
                  {copied ? <Check size={16} className="text-emerald-600" /> : <Copy size={16} />}
                </button>
              </div>
            </div>
          </div>

          <div className="pt-4 flex justify-end">
            <button
              onClick={handleClose}
              className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-xl text-sm transition-colors"
            >
              Đã lưu & Đóng
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50 animate-fadeIn">
      <div className="bg-white rounded-3xl p-6 max-w-md w-full shadow-2xl space-y-5 border border-slate-100">
        <div className="flex items-center justify-between border-b border-slate-100 pb-3">
          <div className="flex items-center gap-2">
            <Key className="w-5 h-5 text-blue-600" />
            <h3 className="text-lg font-extrabold text-slate-900">Tạo Token Key Mới</h3>
          </div>
          <button
            onClick={handleClose}
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
              placeholder="Ví dụ: Farm Server 01"
              className="w-full px-4 py-2.5 bg-slate-50 border border-slate-200 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 rounded-xl outline-none transition-all text-sm font-medium"
              required
              maxLength={128}
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-bold text-slate-700">Số thiết bị tối đa (Capacity)</label>
            <input
              type="number"
              value={maxBindingsStr}
              onChange={e => setMaxBindingsStr(e.target.value)}
              placeholder="Để trống nếu không giới hạn"
              className="w-full px-4 py-2.5 bg-slate-50 border border-slate-200 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 rounded-xl outline-none transition-all text-sm font-medium"
              min="1"
            />
            <p className="text-xs text-slate-500">Số lượng thiết bị tối đa có thể đồng thời sử dụng token này.</p>
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-bold text-slate-700">Thời hạn sử dụng (Ngày)</label>
            <input
              type="number"
              value={expiresDaysStr}
              onChange={e => setExpiresDaysStr(e.target.value)}
              placeholder="Để trống nếu vĩnh viễn"
              className="w-full px-4 py-2.5 bg-slate-50 border border-slate-200 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 rounded-xl outline-none transition-all text-sm font-medium"
              min="1"
            />
            <p className="text-xs text-slate-500">Sau thời gian này token sẽ hết hạn, không thể dùng để đăng ký mới.</p>
          </div>

          <div className="pt-4 flex gap-3 justify-end">
            <button
              type="button"
              onClick={handleClose}
              className="px-5 py-2.5 bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 font-bold rounded-xl text-sm transition-colors"
            >
              Hủy
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white font-bold rounded-xl text-sm transition-colors flex items-center gap-2"
            >
              {loading ? 'Đang tạo...' : 'Tạo Token'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
