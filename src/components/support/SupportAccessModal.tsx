import React, { useState } from 'react';
import { ShieldCheck, Clock, Ticket, AlertTriangle, X, Check } from 'lucide-react';
import { defaultWsSimulator } from '../../services/websocket-simulator';
import { mockCurrentUserSession } from '../../data/mockData';

interface SupportAccessModalProps {
  onClose: () => void;
}

export const SupportAccessModal: React.FC<SupportAccessModalProps> = ({ onClose }) => {
  const [ticketId, setTicketId] = useState('TCK-2026-8910');
  const [durationHours, setDurationHours] = useState(1);
  const [granted, setGranted] = useState(false);

  const handleGrantAccess = (e: React.FormEvent) => {
    e.preventDefault();
    if (!ticketId.trim()) return;

    // Dispatch audit & WS event according to Blueprint Section 15.4
    defaultWsSimulator.publish({
      event_id: `evt_supp_${Math.random().toString(36).substring(2, 8)}`,
      event_type: 'command.updated',
      organization_id: mockCurrentUserSession.organization_id,
      timestamp: new Date().toISOString(),
      data: {
        action: 'support.access.granted',
        ticket_id: ticketId,
        duration_hours: durationHours,
        granted_by: mockCurrentUserSession.email,
        expires_at: new Date(Date.now() + durationHours * 3600 * 1000).toISOString(),
      },
    });

    setGranted(true);
    setTimeout(() => {
      onClose();
    }, 2000);
  };

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-white border border-slate-100 shadow-2xl rounded-3xl w-full max-w-md p-6 space-y-5 animate-fadeIn">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-extrabold text-slate-900 flex items-center gap-2">
            <ShieldCheck size={18} className="text-blue-600" /> Ủy quyền truy cập Support có thời hạn
          </h2>
          <button onClick={onClose} className="p-1 text-slate-400 hover:text-slate-700">
            <X size={18} />
          </button>
        </div>

        {granted ? (
          <div className="p-4 bg-emerald-50 border border-emerald-200 rounded-2xl text-center space-y-2">
            <Check size={32} className="mx-auto text-emerald-600" />
            <p className="text-xs font-bold text-emerald-900">Ủy quyền Support đã được cấp thành công!</p>
            <p className="text-[11px] text-emerald-700">
              Quyền truy cập có hiệu lực trong {durationHours} giờ và được ghi Audit Log chi tiết.
            </p>
          </div>
        ) : (
          <form onSubmit={handleGrantAccess} className="space-y-4">
            <div className="bg-amber-50/80 border border-amber-200/80 p-3.5 rounded-2xl text-xs text-amber-900 leading-relaxed space-y-1">
              <div className="flex items-center gap-1.5 font-bold text-amber-800">
                <AlertTriangle size={15} className="text-amber-600" /> Quyền truy cập an toàn được kiểm soát
              </div>
              <p className="text-[11px] text-amber-800/90">
                Support Agent chỉ có thể truy cập hệ thống theo đúng Mã Ticket và thời gian ủy quyền. Mọi thao tác đều ghi Audit Log.
              </p>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">Mã Ticket Hỗ trợ</label>
              <div className="relative">
                <Ticket size={16} className="absolute left-3.5 top-3 text-slate-400" />
                <input
                  type="text"
                  required
                  value={ticketId}
                  onChange={(e) => setTicketId(e.target.value)}
                  placeholder="TCK-2026-XXXX"
                  className="w-full pl-10 pr-4 py-2.5 bg-slate-50 border border-slate-200 rounded-xl text-xs font-bold font-mono outline-none"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">Thời hạn truy cập (TTL)</label>
              <div className="grid grid-cols-4 gap-2">
                {[1, 2, 4, 8].map((h) => (
                  <button
                    key={h}
                    type="button"
                    onClick={() => setDurationHours(h)}
                    className={`py-2 rounded-xl text-xs font-bold border transition-all ${
                      durationHours === h
                        ? 'bg-blue-600 text-white border-blue-600 shadow-md'
                        : 'bg-slate-50 text-slate-700 border-slate-200 hover:bg-slate-100'
                    }`}
                  >
                    {h} Giờ
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 bg-slate-100 text-slate-700 font-bold text-xs rounded-xl hover:bg-slate-200"
              >
                Hủy
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-blue-600 text-white font-bold text-xs rounded-xl shadow-md hover:bg-blue-700"
              >
                Xác nhận cấp quyền
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
