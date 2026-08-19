import React from 'react';
import { Construction } from 'lucide-react';

interface AdminEmptyShellProps {
  title: string;
  description: string;
  icon?: React.ReactNode;
}

export const AdminEmptyShell: React.FC<AdminEmptyShellProps> = ({ 
  title, 
  description,
  icon = <Construction className="w-12 h-12 text-slate-300" />
}) => {
  return (
    <div className="p-4 md:p-8 max-w-7xl mx-auto space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-black text-slate-900 tracking-tight">{title}</h1>
        <p className="text-sm text-slate-500 font-medium mt-1">{description}</p>
      </div>

      {/* Empty State */}
      <div className="bg-white border border-slate-200/60 rounded-[20px] p-16 shadow-sm flex flex-col items-center justify-center text-center mt-8">
        <div className="bg-slate-50 p-6 rounded-full mb-6 ring-8 ring-slate-50/50">
          {icon}
        </div>
        <h3 className="text-xl font-extrabold text-slate-900 mb-2">Đang phát triển</h3>
        <p className="text-slate-500 max-w-sm mx-auto text-sm leading-relaxed">
          Phân hệ <strong>{title}</strong> hiện đang được xây dựng và sẽ sẵn sàng trong các phase tiếp theo.
        </p>
      </div>
    </div>
  );
};
