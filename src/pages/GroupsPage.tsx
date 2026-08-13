import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Plus } from 'lucide-react';
import { mockGroups, mockDevices } from '../data/mockData';
import { DeviceGroup } from '../types';
import { PermissionGuard } from '../components/common/PermissionGuard';

export const GroupsPage: React.FC = () => {
  const { t } = useTranslation();
  const [groups, setGroups] = useState<DeviceGroup[]>(mockGroups);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [newGroupDesc, setNewGroupDesc] = useState('');

  const handleCreateGroup = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newGroupName.trim()) return;

    const newGroup: DeviceGroup = {
      group_id: `grp_${Math.random().toString(36).substring(2, 8)}`,
      organization_id: 'org_pcp_enterprise_01',
      name: newGroupName,
      description: newGroupDesc || 'Nhóm mới tạo',
      color: '#2563eb',
      device_count: 0,
      created_at: new Date().toISOString(),
    };

    setGroups([...groups, newGroup]);
    setNewGroupName('');
    setNewGroupDesc('');
    setShowCreateModal(false);
  };

  return (
    <div className="space-y-6">
      {/* Title Bar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('nav.groupsTags')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Phân nhóm thiết bị theo lab, chức năng hoặc dự án để cấp quyền và quản lý
          </p>
        </div>

        <PermissionGuard permission="group.manage">
          <button
            onClick={() => setShowCreateModal(true)}
            className="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all flex items-center gap-2 active:scale-95"
          >
            <Plus size={16} /> Tạo Nhóm Mới
          </button>
        </PermissionGuard>
      </div>

      {/* Groups Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {groups.map((group) => {
          const groupDevices = mockDevices.filter((d) => d.groups.includes(group.group_id));

          return (
            <div
              key={group.group_id}
              className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl p-6 space-y-4 hover:shadow-lg transition-all flex flex-col justify-between"
            >
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <span
                      className="w-3.5 h-3.5 rounded-full shadow-sm"
                      style={{ backgroundColor: group.color }}
                    />
                    <h2 className="text-base font-extrabold text-slate-900">{group.name}</h2>
                  </div>
                  <span className="text-xs font-bold px-2.5 py-1 rounded-full bg-slate-100 text-slate-700">
                    {groupDevices.length} máy
                  </span>
                </div>

                <p className="text-xs text-slate-600 leading-relaxed">{group.description}</p>
              </div>

              {/* Group Devices List */}
              <div className="pt-3 border-t border-slate-100 space-y-2">
                <span className="text-[10px] font-extrabold text-slate-400 uppercase tracking-wider block">
                  Danh sách máy thuộc nhóm:
                </span>
                <div className="flex flex-wrap gap-1.5">
                  {groupDevices.length === 0 ? (
                    <span className="text-xs text-slate-400 italic">Chưa có máy nào</span>
                  ) : (
                    groupDevices.map((dev) => (
                      <span
                        key={dev.device_id}
                        className="px-2.5 py-1 rounded-lg bg-slate-50 border border-slate-100 text-slate-700 text-[11px] font-bold"
                      >
                        {dev.display_name}
                      </span>
                    ))
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Create Group Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white border border-slate-100 shadow-2xl rounded-3xl w-full max-w-md p-6 space-y-5 animate-fadeIn">
            <h2 className="text-lg font-extrabold text-slate-900">Tạo Nhóm Thiết Bị Mới</h2>

            <form onSubmit={handleCreateGroup} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Tên nhóm</label>
                <input
                  type="text"
                  required
                  value={newGroupName}
                  onChange={(e) => setNewGroupName(e.target.value)}
                  placeholder="Ví dụ: Cluster Galaxy S7 Lab 01"
                  className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Mô tả</label>
                <textarea
                  rows={3}
                  value={newGroupDesc}
                  onChange={(e) => setNewGroupDesc(e.target.value)}
                  placeholder="Mô tả mục đích nhóm..."
                  className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium focus:ring-2 focus:ring-blue-500 outline-none resize-none"
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="px-4 py-2 bg-slate-100 text-slate-700 font-bold text-xs rounded-xl hover:bg-slate-200 transition-colors"
                >
                  Hủy
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white font-bold text-xs rounded-xl shadow-md hover:bg-blue-700 transition-colors"
                >
                  Lưu nhóm
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
