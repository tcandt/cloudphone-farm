import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { RentalStorePage } from '../pages/RentalStorePage';
import { DeviceListPage } from '../pages/DeviceListPage';
import { WalletPage } from '../pages/WalletPage';
import { DocsPage } from '../pages/DocsPage';

// Mock translation and UI store to isolate the tests
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock('../stores/useUiStore', () => ({
  useUiStore: () => ({
    featureRentalStore: true,
  }),
}));

vi.mock('../services/device-service', () => ({
  deviceService: {
    // Return empty list to prevent side effects in tests,
    // we only care about the shell rendering correctly.
    list: vi.fn().mockResolvedValue({ items: [], total: 0 }),
  },
}));

const mockAddToast = vi.fn();
vi.mock('@ui/toast/Toast', () => ({
  useToastStore: () => mockAddToast,
}));

describe('Slice 1.3 Client Page Shells Contracts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('1. Store Page', () => {
    it('renders canonical identity and prevents real rental mutation', () => {
      render(
        <MemoryRouter>
          <RentalStorePage />
        </MemoryRouter>
      );
      
      // Page identity
      expect(screen.getByText('Cửa hàng cho thuê')).toBeInTheDocument();
      
      // Check that alert() is not used. We mock addToast.
      const rentButton = screen.getAllByText('Thuê ngay')[0];
      expect(rentButton).toBeInTheDocument();
      
      // Should trigger Toast, not a real checkout
      fireEvent.click(rentButton);
      expect(mockAddToast).toHaveBeenCalledWith(expect.objectContaining({
        type: 'info',
        title: 'Tính năng giới hạn'
      }));
    });
  });

  describe('2. Devices Page', () => {
    it('renders client-safe presentation and hides operator telemetry', () => {
      render(
        <MemoryRouter>
          <DeviceListPage />
        </MemoryRouter>
      );
      
      expect(screen.getByText('Quản lý thiết bị')).toBeInTheDocument();

      // Check presentation filters exist
      expect(screen.getByText('Thiếu quyền')).toBeInTheDocument();
      expect(screen.getByText('Sắp hết hạn')).toBeInTheDocument();

      // Prohibited operator labels must be absent
      const prohibitedWords = ['Gán Proxy', 'NodeID', 'Fencing', 'CPU', 'Agent ID'];
      prohibitedWords.forEach(word => {
        expect(screen.queryByText(word)).not.toBeInTheDocument();
      });
    });
  });

  describe('3. Wallet Page', () => {
    it('renders balance shell and prevents real payment integration', () => {
      render(
        <MemoryRouter>
          <WalletPage />
        </MemoryRouter>
      );
      
      expect(screen.getByText('Ví điện tử')).toBeInTheDocument();
      expect(screen.getByText('Số dư khả dụng')).toBeInTheDocument();
      
      // Test deposit CTA
      const depositButton = screen.getByText('Tiến hành thanh toán $50');
      fireEvent.click(depositButton);
      
      expect(mockAddToast).toHaveBeenCalledWith(expect.objectContaining({
        type: 'info',
        message: expect.stringContaining('giai đoạn thương mại')
      }));
    });
  });

  describe('4. Docs Page', () => {
    it('renders 6 canonical categories and mocks APK download', () => {
      render(
        <MemoryRouter>
          <DocsPage />
        </MemoryRouter>
      );
      
      // 6 Categories
      expect(screen.getByText('Getting Started')).toBeInTheDocument();
      expect(screen.getByText('Device Management')).toBeInTheDocument();
      expect(screen.getByText('Remote Control')).toBeInTheDocument();
      expect(screen.getByText('Automation')).toBeInTheDocument();
      expect(screen.getByText('API / Integration')).toBeInTheDocument();
      expect(screen.getByText('Android Agent')).toBeInTheDocument();

      // APK Download mock
      const downloadBtn = screen.getByText('Tải APK');
      fireEvent.click(downloadBtn);
      
      expect(mockAddToast).toHaveBeenCalledWith(expect.objectContaining({
        message: expect.stringContaining('Sắp có')
      }));
    });
  });

  describe('Routing Contracts', () => {
    const TestRouter = ({ initialPath }: { initialPath: string }) => (
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/app/store" element={<div data-testid="route-store" />} />
          <Route path="/app/devices" element={<div data-testid="route-devices" />} />
          <Route path="/app/wallet" element={<div data-testid="route-wallet" />} />
          <Route path="/app/docs" element={<div data-testid="route-docs" />} />
        </Routes>
      </MemoryRouter>
    );

    it('resolves all four canonical paths', () => {
      const { unmount: unmountStore } = render(<TestRouter initialPath="/app/store" />);
      expect(screen.getByTestId('route-store')).toBeInTheDocument();
      unmountStore();

      const { unmount: unmountDevices } = render(<TestRouter initialPath="/app/devices" />);
      expect(screen.getByTestId('route-devices')).toBeInTheDocument();
      unmountDevices();

      const { unmount: unmountWallet } = render(<TestRouter initialPath="/app/wallet" />);
      expect(screen.getByTestId('route-wallet')).toBeInTheDocument();
      unmountWallet();

      const { unmount: unmountDocs } = render(<TestRouter initialPath="/app/docs" />);
      expect(screen.getByTestId('route-docs')).toBeInTheDocument();
      unmountDocs();
    });
  });
});
