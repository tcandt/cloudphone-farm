import React from 'react';
import { NavLink } from 'react-router-dom';
import { ShoppingBag, Smartphone, Wallet, BookOpen } from 'lucide-react';

export const BottomNav: React.FC = () => {
  const items = [
    { to: '/app/store', icon: ShoppingBag, label: 'Cửa hàng' },
    { to: '/app/devices', icon: Smartphone, label: 'Thiết bị' },
    { to: '/app/wallet', icon: Wallet, label: 'Nạp tiền' },
    { to: '/app/docs', icon: BookOpen, label: 'Document' },
  ];

  return (
    <nav className="md:hidden fixed bottom-0 left-0 right-0 bg-white border-t border-slate-200 px-2 pb-4 pt-1 z-40 shadow-[0_-4px_12px_rgba(0,0,0,0.02)]">
      <div className="flex items-center justify-around max-w-md mx-auto">
        {items.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              `flex flex-col items-center justify-center py-2 px-3 min-w-[72px] rounded-xl transition-all duration-200 ${
                isActive
                  ? 'text-emerald-600'
                  : 'text-slate-400 hover:text-slate-600 hover:bg-slate-50'
              }`
            }
          >
            {({ isActive }) => (
              <>
                <div className={`transition-transform duration-200 ${isActive ? '-translate-y-0.5' : ''}`}>
                  <item.icon size={22} className={isActive ? 'stroke-[2.5px]' : 'stroke-2'} />
                </div>
                <span className={`text-[10px] mt-1 transition-all duration-200 ${isActive ? 'font-bold' : 'font-medium'}`}>
                  {item.label}
                </span>
              </>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  );
};
