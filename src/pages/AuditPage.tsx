import React from 'react';
import { useTranslation } from 'react-i18next';
import { History, Shield, Clock, FileText } from 'lucide-react';
import { mockAuditLogs } from '../data/mockData';

export const AuditPage: React.FC = () => {
  const { t } = useTranslation();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('nav.auditLogs')}</h1>
        <p className="text-xs text-slate-500 font-medium">
          Ghi chép toàn bộ sự kiện hệ thống, xác thực, thay đổi quyền và lệnh điều khiển từ xa
        </p>
      </div>

      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50/70 border-b border-slate-100 text-[11px] font-extrabold uppercase text-slate-400 tracking-wider">
                <th className="p-4">Audit ID</th>
                <th className="p-4">Mã hành động</th>
                <th className="p-4">Actor</th>
                <th className="p-4">Resource</th>
                <th className="p-4">Chi tiết</th>
                <th className="p-4">IP Address</th>
                <th className="p-4">Thời gian (UTC)</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {mockAuditLogs.map((log) => (
                <tr key={log.audit_id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="p-4 font-mono text-[11px] text-slate-400">{log.audit_id}</td>
                  <td className="p-4 font-mono font-bold text-blue-600">{log.action_code}</td>
                  <td className="p-4 font-semibold text-slate-800">{log.actor_email}</td>
                  <td className="p-4 font-mono text-slate-600">
                    {log.resource_type}:{log.resource_id}
                  </td>
                  <td className="p-4 text-slate-600 max-w-xs">{log.details}</td>
                  <td className="p-4 font-mono text-slate-500">{log.ip_address}</td>
                  <td className="p-4 text-slate-500">{new Date(log.created_at).toLocaleString('vi-VN')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
