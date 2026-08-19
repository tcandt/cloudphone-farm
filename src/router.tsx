import React, { lazy, Suspense } from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';
import { Layout } from './components/layout/Layout';
import { ErrorBoundary } from './components/common/ErrorBoundary';
import { RouteGuard } from './components/common/RouteGuard';

// Lazy loading route modules for optimal bundle splitting
const LoginPage = lazy(() => import('./pages/auth/LoginPage').then(m => ({ default: m.LoginPage })));
const RegisterPage = lazy(() => import('./pages/auth/RegisterPage').then(m => ({ default: m.RegisterPage })));
const VerifyEmailPage = lazy(() => import('./pages/auth/VerifyEmailPage').then(m => ({ default: m.VerifyEmailPage })));
const ForgotPasswordPage = lazy(() => import('./pages/auth/ForgotPasswordPage').then(m => ({ default: m.ForgotPasswordPage })));
const ResetPasswordPage = lazy(() => import('./pages/auth/ResetPasswordPage').then(m => ({ default: m.ResetPasswordPage })));


const DeviceListPage = lazy(() => import('./pages/DeviceListPage').then(m => ({ default: m.DeviceListPage })));
const DeviceGridPage = lazy(() => import('./pages/DeviceGridPage').then(m => ({ default: m.DeviceGridPage })));
const DeviceDetailPage = lazy(() => import('./pages/DeviceDetailPage').then(m => ({ default: m.DeviceDetailPage })));
const GroupsPage = lazy(() => import('./pages/GroupsPage').then(m => ({ default: m.GroupsPage })));
const AgentsPage = lazy(() => import('./pages/AgentsPage').then(m => ({ default: m.AgentsPage })));
const ActiveSessionsPage = lazy(() => import('./pages/ActiveSessionsPage').then(m => ({ default: m.ActiveSessionsPage })));
const ProxyProfilesPage = lazy(() => import('./pages/ProxyProfilesPage').then(m => ({ default: m.ProxyProfilesPage })));
const TeamPage = lazy(() => import('./pages/TeamPage').then(m => ({ default: m.TeamPage })));
const AuditPage = lazy(() => import('./pages/AuditPage').then(m => ({ default: m.AuditPage })));
const SettingsPage = lazy(() => import('./pages/SettingsPage').then(m => ({ default: m.SettingsPage })));
const BillingPage = lazy(() => import('./pages/BillingPage').then(m => ({ default: m.BillingPage })));
const RentalStorePage = lazy(() => import('./pages/RentalStorePage').then(m => ({ default: m.RentalStorePage })));
const DiagnosticsPage = lazy(() => import('./pages/DiagnosticsPage').then(m => ({ default: m.DiagnosticsPage })));
const LiveMonitorPage = lazy(() => import('./pages/LiveMonitorPage').then(m => ({ default: m.LiveMonitorPage })));
const WalletPage = lazy(() => import('./pages/WalletPage').then(m => ({ default: m.WalletPage })));
const DocsPage = lazy(() => import('./pages/DocsPage').then(m => ({ default: m.DocsPage })));

const SuspenseLoader: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <Suspense
    fallback={
      <div className="flex items-center justify-center p-12 text-slate-400 font-medium text-sm animate-pulse">
        Loading module...
      </div>
    }
  >
    {children}
  </Suspense>
);

import type { RouteObject } from 'react-router-dom';

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <Navigate to="/app" replace />,
  },
  {
    path: '/login',
    element: <SuspenseLoader><LoginPage /></SuspenseLoader>,
  },
  {
    path: '/register',
    element: <SuspenseLoader><RegisterPage /></SuspenseLoader>,
  },
  {
    path: '/verify-email',
    element: <SuspenseLoader><VerifyEmailPage /></SuspenseLoader>,
  },
  {
    path: '/forgot-password',
    element: <SuspenseLoader><ForgotPasswordPage /></SuspenseLoader>,
  },
  {
    path: '/reset-password',
    element: <SuspenseLoader><ResetPasswordPage /></SuspenseLoader>,
  },
  {
    path: '/app',
    element: (
      <RouteGuard>
        <ErrorBoundary>
          <Layout />
        </ErrorBoundary>
      </RouteGuard>
    ),
    children: [
      {
        index: true,
        element: <Navigate to="/app/store" replace />,
      },
      {
        path: 'store',
        element: <SuspenseLoader><RentalStorePage /></SuspenseLoader>,
      },
      {
        path: 'wallet',
        element: <SuspenseLoader><WalletPage /></SuspenseLoader>,
      },
      {
        path: 'docs',
        element: <SuspenseLoader><DocsPage /></SuspenseLoader>,
      },
      {
        path: 'devices',
        element: <SuspenseLoader><DeviceListPage /></SuspenseLoader>,
      },
      {
        path: 'devices/grid',
        element: <SuspenseLoader><DeviceGridPage /></SuspenseLoader>,
      },
      {
        path: 'devices/:id',
        element: <SuspenseLoader><DeviceDetailPage /></SuspenseLoader>,
      },
      {
        path: 'groups',
        element: <SuspenseLoader><GroupsPage /></SuspenseLoader>,
      },
      {
        path: 'agents',
        element: <SuspenseLoader><AgentsPage /></SuspenseLoader>,
      },
      {
        path: 'sessions',
        element: <SuspenseLoader><ActiveSessionsPage /></SuspenseLoader>,
      },
      {
        path: 'proxy',
        element: <SuspenseLoader><ProxyProfilesPage /></SuspenseLoader>,
      },
      {
        path: 'team',
        element: <SuspenseLoader><TeamPage /></SuspenseLoader>,
      },
      {
        path: 'audit',
        element: <SuspenseLoader><AuditPage /></SuspenseLoader>,
      },
      {
        path: 'settings',
        element: <SuspenseLoader><SettingsPage /></SuspenseLoader>,
      },
      {
        path: 'billing',
        element: <SuspenseLoader><BillingPage /></SuspenseLoader>,
      },
      {
        path: 'rental',
        element: <SuspenseLoader><RentalStorePage /></SuspenseLoader>,
      },
      {
        path: 'diagnostics',
        element: <SuspenseLoader><DiagnosticsPage /></SuspenseLoader>,
      },
      {
        path: 'live-monitor',
        element: <SuspenseLoader><LiveMonitorPage /></SuspenseLoader>,
      },
    ],
  },
  {
    path: '*',
    element: <Navigate to="/app" replace />,
  },
];

if (import.meta.env.DEV) {
  const DesignSystemPreview = lazy(() => import('./dev/DesignSystemPreview').then(m => ({ default: m.DesignSystemPreview })));
  routes.unshift({
    path: '/dev/design-system',
    element: <SuspenseLoader><DesignSystemPreview /></SuspenseLoader>,
  });
}

export const router = createBrowserRouter(routes);
