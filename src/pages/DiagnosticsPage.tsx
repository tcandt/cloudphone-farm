import React, { useState } from 'react';
import { Server, Database, CheckCircle2, RefreshCw, FileText } from 'lucide-react';

export const DiagnosticsPage: React.FC = () => {
  const [refreshing, setRefreshing] = useState(false);

  const handleRefresh = () => {
    setRefreshing(true);
    setTimeout(() => setRefreshing(false), 800);
  };

  const logs = [
    { time: '17:28:01', level: 'INFO', module: 'presence', msg: 'Device dev_s7_001 heartbeat acked (TTL 30s)' },
    { time: '17:27:45', level: 'INFO', module: 'stream', msg: 'WebRTC SDP exchange completed for session str_sess_9001' },
    { time: '17:27:12', level: 'WARN', module: 'agent', msg: 'Device dev_redmi_004 network jitter detected (lag 45ms)' },
    { time: '17:26:30', level: 'INFO', module: 'audit', msg: 'Support access grant support.access.granted emitted' },
  ];

  return (
    <div className="space-y-6">
      {/* Title & Refresh Action */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">System Diagnostics & Infra Health</h1>
          <p className="text-xs text-slate-500 font-medium">
            Quan sát chỉ số hạ tầng server, PostgreSQL connection pool, Redis cache và coturn TURN server
          </p>
        </div>

        <button
          onClick={handleRefresh}
          className="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-lg shadow-blue-500/20 transition-all flex items-center gap-2"
        >
          <RefreshCw size={15} className={refreshing ? 'animate-spin' : ''} /> Refresh Diagnostics
        </button>
      </div>

      {/* Health Status Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-white border border-slate-100 p-5 rounded-2xl shadow-pcp-card space-y-2">
          <div className="flex items-center justify-between text-xs font-bold text-slate-400">
            <span>/health/live</span>
            <CheckCircle2 size={18} className="text-emerald-500" />
          </div>
          <p className="text-xl font-black text-emerald-600">PASSING</p>
          <p className="text-[10px] text-slate-500">Process alive & healthy</p>
        </div>

        <div className="bg-white border border-slate-100 p-5 rounded-2xl shadow-pcp-card space-y-2">
          <div className="flex items-center justify-between text-xs font-bold text-slate-400">
            <span>/health/ready</span>
            <CheckCircle2 size={18} className="text-emerald-500" />
          </div>
          <p className="text-xl font-black text-emerald-600">READY</p>
          <p className="text-[10px] text-slate-500">DB & Redis connected</p>
        </div>

        <div className="bg-white border border-slate-100 p-5 rounded-2xl shadow-pcp-card space-y-2">
          <div className="flex items-center justify-between text-xs font-bold text-slate-400">
            <span>PostgreSQL Pool</span>
            <Database size={18} className="text-blue-500" />
          </div>
          <p className="text-xl font-black text-slate-900">14 / 50</p>
          <p className="text-[10px] text-slate-500">Active connections</p>
        </div>

        <div className="bg-white border border-slate-100 p-5 rounded-2xl shadow-pcp-card space-y-2">
          <div className="flex items-center justify-between text-xs font-bold text-slate-400">
            <span>coturn TURN Relay</span>
            <Server size={18} className="text-purple-500" />
          </div>
          <p className="text-xl font-black text-slate-900">4.2 Mbps</p>
          <p className="text-[10px] text-emerald-600 font-bold">ICE Candidates OK</p>
        </div>
      </div>

      {/* System Diagnostic Log Stream */}
      <div className="bg-slate-900 rounded-3xl p-6 text-white space-y-4 shadow-xl">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-extrabold flex items-center gap-2 text-slate-200">
            <FileText size={18} className="text-amber-400" /> System Diagnostic Log Stream (Redacted Payload)
          </h2>
          <span className="text-[10px] font-mono text-emerald-400 bg-slate-800 px-2.5 py-1 rounded-md">
            Auto-Redacted Secrets
          </span>
        </div>

        <div className="space-y-2 font-mono text-xs max-h-60 overflow-y-auto custom-scrollbar">
          {logs.map((log, idx) => (
            <div key={idx} className="p-2.5 bg-slate-950/80 rounded-xl border border-slate-800 flex items-start gap-3">
              <span className="text-slate-500 font-bold">[{log.time}]</span>
              <span
                className={`font-bold px-1.5 py-0.5 rounded text-[10px] ${
                  log.level === 'WARN' ? 'bg-amber-500/20 text-amber-400' : 'bg-blue-500/20 text-blue-400'
                }`}
              >
                {log.level}
              </span>
              <span className="text-slate-400">[{log.module}]</span>
              <span className="text-slate-200 flex-1">{log.msg}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
