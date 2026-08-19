import React from 'react';
import { AdminEmptyShell } from './AdminEmptyShell';
import { 
  Users, Smartphone, Layers, MonitorPlay, 
  Server, Key, Package, Bot, Workflow, 
  Activity, ShoppingCart, Tags, Wallet, 
  ArrowRightLeft, BellRing, FileSearch, 
  UserCog, ShieldCheck, Settings 
} from 'lucide-react';

export const CustomersPage = () => <AdminEmptyShell title="Khách hàng" description="Quản lý khách hàng và tài khoản." icon={<Users className="w-12 h-12 text-blue-300" />} />;
export const DevicesPage = () => <AdminEmptyShell title="Kho thiết bị" description="Quản lý toàn bộ thiết bị vật lý trên hệ thống." icon={<Smartphone className="w-12 h-12 text-slate-300" />} />;
export const DeviceGroupsPage = () => <AdminEmptyShell title="Nhóm thiết bị" description="Cấu hình nhóm và phân bổ thiết bị." icon={<Layers className="w-12 h-12 text-emerald-300" />} />;
export const WallMonitorPage = () => <AdminEmptyShell title="Wall Monitor" description="Wall Monitor will become available in the media scaling phase." icon={<MonitorPlay className="w-12 h-12 text-indigo-300" />} />;

export const AgentsPage = () => <AdminEmptyShell title="Agents" description="Quản lý các Node/Worker agent." icon={<Server className="w-12 h-12 text-slate-300" />} />;
export const AgentKeysPage = () => <AdminEmptyShell title="Token Keys" description="Token Key management will be enabled in the enrollment phase." icon={<Key className="w-12 h-12 text-amber-300" />} />;
export const AgentReleasesPage = () => <AdminEmptyShell title="APK Releases" description="Quản lý các phiên bản APK của Agent." icon={<Package className="w-12 h-12 text-emerald-300" />} />;

export const AutomationPage = () => <AdminEmptyShell title="Tổng quan Automation" description="Automation features will be enabled in later phases." icon={<Bot className="w-12 h-12 text-purple-300" />} />;
export const WorkflowsPage = () => <AdminEmptyShell title="Automation Workflows" description="Workflow builder and management." icon={<Workflow className="w-12 h-12 text-purple-300" />} />;
export const AutomationRunsPage = () => <AdminEmptyShell title="Automation Runs" description="Lịch sử thực thi tự động hóa." icon={<Activity className="w-12 h-12 text-purple-300" />} />;

export const RentalsPage = () => <AdminEmptyShell title="Cho thuê" description="Quản lý các hợp đồng và phiên thuê thiết bị." icon={<ShoppingCart className="w-12 h-12 text-orange-300" />} />;
export const PlansPage = () => <AdminEmptyShell title="Gói dịch vụ" description="Quản lý các gói cước và bảng giá." icon={<Tags className="w-12 h-12 text-orange-300" />} />;

export const WalletsPage = () => <AdminEmptyShell title="Ví khách hàng" description="Quản lý số dư và ví của người dùng." icon={<Wallet className="w-12 h-12 text-emerald-300" />} />;
export const TransactionsPage = () => <AdminEmptyShell title="Giao dịch" description="Lịch sử nạp/trừ tiền và hóa đơn." icon={<ArrowRightLeft className="w-12 h-12 text-emerald-300" />} />;

export const AlertsPage = () => <AdminEmptyShell title="Cảnh báo" description="Theo dõi sự cố và cảnh báo hệ thống." icon={<BellRing className="w-12 h-12 text-rose-300" />} />;
export const AuditPage = () => <AdminEmptyShell title="Nhật ký Audit" description="Lưu vết mọi hành động của Admin và User." icon={<FileSearch className="w-12 h-12 text-slate-300" />} />;

export const AdminUsersPage = () => <AdminEmptyShell title="Admin Users" description="Quản lý tài khoản quản trị viên." icon={<UserCog className="w-12 h-12 text-slate-300" />} />;
export const RolesPage = () => <AdminEmptyShell title="Vai trò & Quyền" description="Phân quyền chi tiết cho Admin." icon={<ShieldCheck className="w-12 h-12 text-slate-300" />} />;
export const SettingsPage = () => <AdminEmptyShell title="Cài đặt hệ thống" description="Cấu hình tham số lõi của hệ thống." icon={<Settings className="w-12 h-12 text-slate-300" />} />;
