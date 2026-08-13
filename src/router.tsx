import React from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';
import { Layout } from './components/layout/Layout';
import { ErrorBoundary } from './components/common/ErrorBoundary';

import { LoginPage } from './pages/auth/LoginPage';
import { RegisterPage } from './pages/auth/RegisterPage';
import { VerifyEmailPage } from './pages/auth/VerifyEmailPage';
import { ForgotPasswordPage } from './pages/auth/ForgotPasswordPage';
import { ResetPasswordPage } from './pages/auth/ResetPasswordPage';

import { DashboardPage } from './pages/DashboardPage';
import { DeviceListPage } from './pages/DeviceListPage';
import { DeviceGridPage } from './pages/DeviceGridPage';
import { DeviceDetailPage } from './pages/DeviceDetailPage';
import { GroupsPage } from './pages/GroupsPage';
import { AgentsPage } from './pages/AgentsPage';
import { ActiveSessionsPage } from './pages/ActiveSessionsPage';
import { ProxyProfilesPage } from './pages/ProxyProfilesPage';
import { TeamPage } from './pages/TeamPage';
import { AuditPage } from './pages/AuditPage';
import { SettingsPage } from './pages/SettingsPage';
import { BillingPage } from './pages/BillingPage';
import { RentalStorePage } from './pages/RentalStorePage';
import { DiagnosticsPage } from './pages/DiagnosticsPage';
import { LiveMonitorPage } from './pages/LiveMonitorPage';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Navigate to="/app" replace />,
  },
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/register',
    element: <RegisterPage />,
  },
  {
    path: '/verify-email',
    element: <VerifyEmailPage />,
  },
  {
    path: '/forgot-password',
    element: <ForgotPasswordPage />,
  },
  {
    path: '/reset-password',
    element: <ResetPasswordPage />,
  },
  {
    path: '/app',
    element: (
      <ErrorBoundary>
        <Layout />
      </ErrorBoundary>
    ),
    children: [
      {
        index: true,
        element: <DashboardPage />,
      },
      {
        path: 'devices',
        element: <DeviceListPage />,
      },
      {
        path: 'devices/grid',
        element: <DeviceGridPage />,
      },
      {
        path: 'devices/:id',
        element: <DeviceDetailPage />,
      },
      {
        path: 'groups',
        element: <GroupsPage />,
      },
      {
        path: 'agents',
        element: <AgentsPage />,
      },
      {
        path: 'sessions',
        element: <ActiveSessionsPage />,
      },
      {
        path: 'proxy',
        element: <ProxyProfilesPage />,
      },
      {
        path: 'team',
        element: <TeamPage />,
      },
      {
        path: 'audit',
        element: <AuditPage />,
      },
      {
        path: 'settings',
        element: <SettingsPage />,
      },
      {
        path: 'billing',
        element: <BillingPage />,
      },
      {
        path: 'rental',
        element: <RentalStorePage />,
      },
      {
        path: 'diagnostics',
        element: <DiagnosticsPage />,
      },
      {
        path: 'live-monitor',
        element: <LiveMonitorPage />,
      },
    ],
  },
  {
    path: '*',
    element: <Navigate to="/app" replace />,
  },
]);
