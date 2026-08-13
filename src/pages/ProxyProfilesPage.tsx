import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Globe, Plus, Wifi, Check, Trash2, ShieldCheck, Activity } from 'lucide-react';

interface ProxyProfile {
  id: string;
  name: string;
  type: 'HTTP' | 'SOCKS5';
  host: string;
  port: number;
  assigned_group: string;
  status: 'active' | 'testing' | 'offline';
  ping_ms: number;
}

const mockProxyProfiles: ProxyProfile[] = [
  {
    id: 'prx_01',
    name: 'Corporate Routing VN-01',
    type: 'SOCKS5',
    host: '118.69.182.10',
    port: 1080,
    assigned_group: 'Lab Alpha - Galaxy S7 Cluster',
    status: 'active',
    ping_ms: 22,
  },
  {
    id: 'prx_02',
    name: 'QA Testing US-East',
    type: 'HTTP',
    host: '54.210.12.99',
    port: 8080,
    assigned_group: 'Automation Pool - Pixel 6',
    status: 'active',
    ping_ms: 180,
  },
];

export const ProxyProfilesPage: React.FC = () => {
  const { t } = useTranslation();
  const [proxies, setProxies] = useState<ProxyProfile[]>(mockProxyProfiles);
  const [showModal, setShowModal] = useState(false);
  const [name, setName] = useState('');
  const [host, setHost] = useState('');
  const [port, setPort] = useState(8080);
  const [type, setType] = useState<'HTTP' | 'SOCKS5'>('SOCKS5');

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name || !host) return;

    const newPrx: ProxyProfile = {
      id: `prx_${Math.random().toString(36).substring(2, 6)}`,
      name,
      type,
      host,
      port,
      assigned_group: 'Tất cả nhóm',
      status: 'active',
      ping_ms: Math.floor(15 + Math.random() * 40),
    };

    setProxies([...proxies, newPrx]);
    setName('');
    setHost('');
    setShowModal(false);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{t('header.networkProfiles')}</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quản lý cấu hình định tuyến Proxy doanh nghiệp (HTTP/SOCKS5) cho các nhóm thiết bị
          </p>
        </div>

        <button
          onClick={() => setShowModal(true)}
          className="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all flex items-center gap-2"
        >
          <Plus size={16} /> Thêm Cấu hình Proxy Mới
        </button>
      </div>

      <div className="bg-white border border-slate-100 shadow-pcp-card rounded-3xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-slate-50/70 border-b border-slate-100 text-[11px] font-extrabold uppercase text-slate-400 tracking-wider">
                <th className="p-4">Tên Cấu hình</th>
                <th className="p-4">Loại Protocol</th>
                <th className="p-4">Host IP & Port</th>
                <th className="p-4">Nhóm áp dụng</th>
                <th className="p-4">Ping Latency</th>
                <th className="p-4 text-right">Thao tác</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-xs font-medium text-slate-700">
              {proxies.map((prx) => (
                <tr key={prx.id} className="hover:bg-slate-50/80 transition-colors">
                  <td className="p-4 font-bold text-slate-900">{prx.name}</td>
                  <td className="p-4">
                    <span className="px-2.5 py-1 rounded-md bg-blue-50 text-blue-800 font-bold text-[10px]">
                      {prx.type}
                    </span>
                  </td>
                  <td className="p-4 font-mono text-slate-800">
                    {prx.host}:{prx.port}
                  </td>
                  <td className="p-4 font-semibold text-slate-700">{prx.assigned_group}</td>
                  <td className="p-4 text-emerald-600 font-extrabold">{prx.ping_ms} ms</td>
                  <td className="p-4 text-right">
                    <button
                      onClick={() => setProxies(proxies.filter((p) => p.id !== prx.id))}
                      className="p-1.5 text-rose-500 hover:bg-rose-50 rounded-lg transition-colors"
                      title="Xóa Proxy"
                    >
                      <Trash2 size={16} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {showModal && (
        <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white border border-slate-100 shadow-2xl rounded-3xl w-full max-w-md p-6 space-y-5 animate-fadeIn">
            <h2 className="text-lg font-extrabold text-slate-900">Thêm Proxy Profile Mới</h2>

            <form onSubmit={handleCreate} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Tên cấu hình</label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Ví dụ: Enterprise SOCKS5 Node 01"
                  className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium outline-none"
                />
              </div>

              <div className="grid grid-cols-3 gap-2">
                <div className="col-span-2">
                  <label className="block text-xs font-semibold text-slate-700 mb-1">Host IP</label>
                  <input
                    type="text"
                    required
                    value={host}
                    onChange={(e) => setHost(e.target.value)}
                    placeholder="192.168.1.100"
                    className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium outline-none"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">Port</label>
                  <input
                    type="number"
                    required
                    value={port}
                    onChange={(e) => setPort(Number(e.target.value))}
                    className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-medium outline-none"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Loại Proxy Protocol</label>
                <select
                  value={type}
                  onChange={(e) => setType(e.target.value as 'HTTP' | 'SOCKS5')}
                  className="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-semibold text-slate-800 outline-none"
                >
                  <option value="SOCKS5">SOCKS5</option>
                  <option value="HTTP">HTTP / HTTPS</option>
                </select>
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="px-4 py-2 bg-slate-100 text-slate-700 font-bold text-xs rounded-xl hover:bg-slate-200 transition-colors"
                >
                  Hủy
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white font-bold text-xs rounded-xl shadow-md hover:bg-blue-700 transition-colors"
                >
                  Lưu cấu hình
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
