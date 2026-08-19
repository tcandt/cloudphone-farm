import { 
  LayoutDashboard, 
  Users, 
  Smartphone, 
  Layers, 
  MonitorPlay, 
  Server, 
  Key, 
  Package, 
  Bot, 
  Workflow, 
  Activity, 
  ShoppingCart, 
  Tags, 
  Wallet, 
  ArrowRightLeft, 
  BellRing, 
  FileSearch, 
  UserCog, 
  ShieldCheck, 
  Settings 
} from 'lucide-react';
import { ElementType } from 'react';

export interface AdminNavItem {
  title: string;
  href: string;
  icon: ElementType;
}

export interface AdminNavGroup {
  groupName: string;
  items: AdminNavItem[];
}

export const adminNavGroups: AdminNavGroup[] = [
  {
    groupName: 'Tổng quan',
    items: [
      { title: 'Overview', href: '/admin/overview', icon: LayoutDashboard },
    ],
  },
  {
    groupName: 'Vận hành',
    items: [
      { title: 'Khách hàng', href: '/admin/customers', icon: Users },
      { title: 'Thiết bị', href: '/admin/devices', icon: Smartphone },
      { title: 'Nhóm thiết bị', href: '/admin/device-groups', icon: Layers },
      { title: 'Wall Monitor', href: '/admin/wall-monitor', icon: MonitorPlay },
    ],
  },
  {
    groupName: 'Hạ tầng Agent',
    items: [
      { title: 'Agent', href: '/admin/agents', icon: Server },
      { title: 'Token Keys', href: '/admin/agent-keys', icon: Key },
      { title: 'APK Releases', href: '/admin/agent-releases', icon: Package },
    ],
  },
  {
    groupName: 'Automation',
    items: [
      { title: 'Tổng quan Automation', href: '/admin/automation', icon: Bot },
      { title: 'Workflows', href: '/admin/workflows', icon: Workflow },
      { title: 'Runs', href: '/admin/automation-runs', icon: Activity },
    ],
  },
  {
    groupName: 'Kinh doanh',
    items: [
      { title: 'Cho thuê', href: '/admin/rentals', icon: ShoppingCart },
      { title: 'Gói dịch vụ', href: '/admin/plans', icon: Tags },
    ],
  },
  {
    groupName: 'Tài chính',
    items: [
      { title: 'Ví khách hàng', href: '/admin/wallets', icon: Wallet },
      { title: 'Giao dịch', href: '/admin/transactions', icon: ArrowRightLeft },
    ],
  },
  {
    groupName: 'Giám sát',
    items: [
      { title: 'Cảnh báo', href: '/admin/alerts', icon: BellRing },
      { title: 'Audit Logs', href: '/admin/audit', icon: FileSearch },
    ],
  },
  {
    groupName: 'Hệ thống',
    items: [
      { title: 'Admin Users', href: '/admin/admin-users', icon: UserCog },
      { title: 'Roles', href: '/admin/roles', icon: ShieldCheck },
      { title: 'Settings', href: '/admin/settings', icon: Settings },
    ],
  },
];
