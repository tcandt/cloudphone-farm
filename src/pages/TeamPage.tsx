import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { UserPlus, CheckCircle2 } from 'lucide-react';
import { UserRole } from '../types';
import { PermissionGuard } from '../components/common/PermissionGuard';

interface MemberItem {
  id: string;
  name: string;
  email: string;
  role: UserRole;
  status: 'active' | 'invited';
}

const initialMembers: MemberItem[] = [
  { id: '1', name: 'Minh Tuấn', email: 'admin@phonecontrol.io', role: 'owner', status: 'active' },
  { id: '2', name: 'Hoàng Nam', email: 'nam.hoang@phonecontrol.io', role: 'admin', status: 'active' },
  { id: '3', name: 'Thanh Hương', email: 'huong.thanh@phonecontrol.io', role: 'operator', status: 'active' },
];

export const TeamPage: React.FC = () => {
  const { t } = useTranslation();
  const [members, setMembers] = useState<MemberItem[]>(initialMembers);
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<UserRole>('operator');

  const handleInvite = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inviteEmail.trim()) return;

    const newMember: MemberItem = {
      id: Math.random().toString(),
      name: inviteEmail.split('@')[0],
      email: inviteEmail,
      role: inviteRole,
      status: 'invited',
    };

    setMembers([...members, newMember]);
    setInviteEmail('');
    setShowInviteModal(false);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('nav.team')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quản lý thành viên tổ chức và phân gán vai trò RBAC (Owner, Admin, Manager, Operator, Viewer)
          </p>
        </div>

        <PermissionGuard permission="member.invite">
          <button
            onClick={() => setShowInviteModal(true)}
            className="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all flex items-center gap-2 active:scale-95"
          >
            <UserPlus size={16} /> Mời thành viên mới
          </button>
        </PermissionGuard>
      </div>

      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50/70 border-b border-slate-100 text-[11px] font-extrabold uppercase text-slate-400 tracking-wider">
                <th className="p-4">Thành viên</th>
                <th className="p-4">Email</th>
                <th className="p-4">Vai trò (Role)</th>
                <th className="p-4">Trạng thái</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {members.map((mem) => (
                <tr key={mem.id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="p-4 font-bold text-slate-900">{mem.name}</td>
                  <td className="p-4 text-slate-600 font-mono">{mem.email}</td>
                  <td className="p-4">
                    <span
                      className={`px-2.5 py-1 rounded-full font-bold text-[10px] uppercase ${
                        mem.role === 'owner'
                          ? 'bg-amber-100 text-amber-800 border border-amber-300'
                          : mem.role === 'admin'
                          ? 'bg-purple-100 text-purple-800'
                          : 'bg-blue-100 text-blue-800'
                      }`}
                    >
                      {mem.role}
                    </span>
                  </td>
                  <td className="p-4">
                    <span
                      className={`inline-flex items-center gap-1 font-bold text-[11px] ${
                        mem.status === 'active' ? 'text-emerald-600' : 'text-amber-600'
                      }`}
                    >
                      <CheckCircle2 size={14} /> {mem.status === 'active' ? 'Hoạt động' : 'Đã mời'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {showInviteModal && (
        <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white border border-slate-100 shadow-2xl rounded-3xl w-full max-w-md p-6 space-y-5 animate-fadeIn">
            <h2 className="text-lg font-extrabold text-slate-900">Mời Thành Viên Vào Organization</h2>

            <form onSubmit={handleInvite} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Email người nhận</label>
                <input
                  type="email"
                  required
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder="colleague@organization.com"
                  className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Gán vai trò RBAC</label>
                <select
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value as UserRole)}
                  className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-semibold text-slate-800 outline-none"
                >
                  <option value="admin">Admin</option>
                  <option value="manager">Manager</option>
                  <option value="operator">Operator (Điều khiển)</option>
                  <option value="viewer">Viewer (Chỉ xem)</option>
                </select>
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowInviteModal(false)}
                  className="px-4 py-2 bg-slate-100 text-slate-700 font-bold text-xs rounded-xl hover:bg-slate-200 transition-colors"
                >
                  Hủy
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white font-bold text-xs rounded-xl shadow-md hover:bg-blue-700 transition-colors"
                >
                  Gửi lời mời
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
