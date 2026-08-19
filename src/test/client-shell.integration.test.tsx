import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { Sidebar } from '../components/layout/Sidebar';
import { BottomNav } from '../components/layout/BottomNav';
import { Layout } from '../components/layout/Layout';
import { useUiStore } from '../stores/useUiStore';
import { routes } from '../router';

// Mock the BrandLogo since it might load assets or have complex internal state we don't care about here
vi.mock('@brand/index', () => ({
  BrandLogo: () => <div data-testid="mock-brand-logo">BrandLogo</div>,
}));

describe('Slice 1.2 Client AppShell Contracts', () => {
  beforeEach(() => {
    // Reset Zustand store
    useUiStore.setState({ isSidebarCollapsed: false });
  });

  describe('A. Desktop Sidebar Contents', () => {
    it('contains exactly the 4 canonical navigation items', () => {
      render(
        <MemoryRouter>
          <Sidebar />
        </MemoryRouter>
      );
      
      const navItems = screen.getAllByRole('link');
      expect(navItems).toHaveLength(4);
      
      expect(screen.getByText('Cửa hàng cho thuê')).toBeInTheDocument();
      expect(screen.getByText('Quản lý thiết bị')).toBeInTheDocument();
      expect(screen.getByText('Nạp tiền')).toBeInTheDocument();
      expect(screen.getByText('Document')).toBeInTheDocument();
    });
  });

  describe('B. Primary Sidebar Exclusions', () => {
    it('does NOT contain legacy operational items', () => {
      render(
        <MemoryRouter>
          <Sidebar />
        </MemoryRouter>
      );
      
      expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
      expect(screen.queryByText('Agents')).not.toBeInTheDocument();
      expect(screen.queryByText('Settings')).not.toBeInTheDocument();
    });
  });

  describe('C. Collapsed Sidebar Behavior', () => {
    it('remains rendered but hides text labels (icon rail)', () => {
      useUiStore.setState({ isSidebarCollapsed: true });
      
      render(
        <MemoryRouter>
          <Sidebar />
        </MemoryRouter>
      );
      
      const sidebar = screen.getByRole('complementary'); // aside element
      expect(sidebar).toBeInTheDocument();
      expect(sidebar.className).toContain('w-[72px]'); // Icon rail width check
      
      // Text labels should be hidden
      expect(screen.queryByText('Cửa hàng cho thuê')).not.toBeInTheDocument();
      expect(screen.queryByText('Quản lý thiết bị')).not.toBeInTheDocument();
      
      // Links should still be present
      const navItems = screen.getAllByRole('link');
      expect(navItems).toHaveLength(4);
      
      // Should have titles for tooltips
      expect(navItems[0]).toHaveAttribute('title', 'Cửa hàng cho thuê');
    });
  });

  describe('D. BottomNav Destinations', () => {
    it('contains exactly 4 canonical destinations', () => {
      render(
        <MemoryRouter>
          <BottomNav />
        </MemoryRouter>
      );
      
      const navItems = screen.getAllByRole('link');
      expect(navItems).toHaveLength(4);
      
      expect(navItems[0]).toHaveAttribute('href', '/app/store');
      expect(navItems[1]).toHaveAttribute('href', '/app/devices');
      expect(navItems[2]).toHaveAttribute('href', '/app/wallet');
      expect(navItems[3]).toHaveAttribute('href', '/app/docs');
    });
  });

  describe('E. Router Redirects', () => {
    it('redirects /app to /app/store', () => {
      const appRoute = routes.find(r => r.path === '/app');
      expect(appRoute).toBeDefined();
      
      const indexRoute = appRoute?.children?.find(c => c.index === true);
      expect(indexRoute).toBeDefined();
      
      // Since it's a Navigate component, we can check its props via React element inspection
      const element = indexRoute?.element as React.ReactElement;
      expect(element.props.to).toBe('/app/store');
      expect(element.props.replace).toBe(true);
    });
  });

  describe('F. BrandLogo Rendering', () => {
    it('is rendered by the Client Shell (Header and Sidebar)', () => {
      render(
        <MemoryRouter>
          <Layout />
        </MemoryRouter>
      );
      
      const brandLogos = screen.getAllByTestId('mock-brand-logo');
      // Should be in Sidebar (Desktop) and Header (Mobile)
      expect(brandLogos.length).toBeGreaterThanOrEqual(1);
    });
  });
});
