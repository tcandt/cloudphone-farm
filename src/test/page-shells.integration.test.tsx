import React from 'react';
import { DeviceEntity } from '../types';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { RentalStorePage } from '../pages/RentalStorePage';
import { DeviceListPage } from '../pages/DeviceListPage';
import { WalletPage } from '../pages/WalletPage';
import { DocsPage } from '../pages/DocsPage';
import { routes } from '../router';
import { deviceService } from '../services/device-service';

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
    list: vi.fn(),
  },
}));

const mockAddToast = vi.fn();
vi.mock('@ui/toast/Toast', () => ({
  useToastStore: () => mockAddToast,
}));

describe('Slice 1.3 Client Page Shells Contracts', () => {
  let alertSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
  });

  afterEach(() => {
    alertSpy.mockRestore();
  });

  describe('1. Store Page', () => {
    it('renders canonical identity and prevents real rental mutation (no alert)', () => {
      render(
        <MemoryRouter>
          <RentalStorePage />
        </MemoryRouter>
      );
      
      expect(screen.getByText('Cửa hàng cho thuê')).toBeInTheDocument();
      
      const rentButton = screen.getAllByText('Thuê ngay')[0];
      expect(rentButton).toBeInTheDocument();
      
      fireEvent.click(rentButton);
      
      expect(mockAddToast).toHaveBeenCalledWith(expect.objectContaining({
        type: 'info',
        title: 'Tính năng giới hạn'
      }));
      
      // Explicitly assert alert is not called
      expect(alertSpy).not.toHaveBeenCalled();
    });
  });

  describe('2. Devices Page', () => {
    const mockRealDevice = {
      device_id: 'dev-1',
      name: 'Phone-A',
      display_name: 'Galaxy S23',
      model: 'SM-S911B',
      android_version: '13',
      status: 'online',
      group_id: 'group-1'
    };

    it('renders realistic mocked device and hides operator telemetry', async () => {
      vi.mocked(deviceService.list).mockResolvedValue({ 
        items: [mockRealDevice as unknown as DeviceEntity], 
        total: 1,
        page: 1,
        limit: 10
      });

      render(
        <MemoryRouter>
          <DeviceListPage />
        </MemoryRouter>
      );
      
      // Verify loading resolves and real device fields are rendered
      await waitFor(() => {
        expect(screen.getAllByText('Galaxy S23')[0]).toBeInTheDocument();
      });
      expect(screen.getAllByText('SM-S911B')[0]).toBeInTheDocument();
      expect(screen.getAllByText('Android 13')[0]).toBeInTheDocument();
      
      // Prohibited operator labels must be absent
      const prohibitedWords = ['Gán Proxy', 'NodeID', 'Fencing', 'CPU', 'Agent ID', 'Restart'];
      prohibitedWords.forEach(word => {
        expect(screen.queryByText(word)).not.toBeInTheDocument();
      });

      // Assert deviceService.list was NOT called with invented values
      expect(deviceService.list).toHaveBeenCalledWith(
        expect.not.objectContaining({ status: expect.anything() })
      );
    });

    it('renders ERROR state (not empty state) on API failure', async () => {
      vi.mocked(deviceService.list).mockRejectedValue(new Error('API failure'));

      render(
        <MemoryRouter>
          <DeviceListPage />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Không thể tải dữ liệu')).toBeInTheDocument();
        expect(screen.getByText('Không thể tải danh sách thiết bị lúc này.')).toBeInTheDocument();
      });
      // Ensure "Không tìm thấy thiết bị" (empty state) is not shown
      expect(screen.queryByText('Không tìm thấy thiết bị')).not.toBeInTheDocument();
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
      
      const depositButton = screen.getByText('Tiến hành thanh toán $50');
      fireEvent.click(depositButton);
      
      expect(mockAddToast).toHaveBeenCalledWith(expect.objectContaining({
        type: 'info',
        message: expect.stringContaining('giai đoạn thương mại')
      }));
    });
  });

  describe('4. Docs Page', () => {
    it('renders canonical categories and clearly marks Agent as unavailable', () => {
      render(
        <MemoryRouter>
          <DocsPage />
        </MemoryRouter>
      );
      
      // Category click test
      const firstCategory = screen.getByText('Getting Started');
      fireEvent.click(firstCategory);
      expect(mockAddToast).toHaveBeenCalledWith(expect.objectContaining({
        title: 'Tài liệu chi tiết'
      }));

      // Assert Agent has no active download/href and is marked as coming soon
      expect(screen.getByText('Android Agent')).toBeInTheDocument();
      expect(screen.getByText('Sắp có')).toBeInTheDocument();
      expect(screen.getByText('Sẽ khả dụng trong giai đoạn Agent')).toBeInTheDocument();
      expect(screen.queryByText('Tải APK')).not.toBeInTheDocument();
    });
  });

  describe('Routing Contracts', () => {
    it('contains all four canonical paths inside the app route', () => {
      // Find the /app route
      const appRoute = routes.find(r => r.path === '/app');
      expect(appRoute).toBeDefined();
      expect(appRoute?.children).toBeDefined();

      const childPaths = appRoute!.children!.map(child => child.path);
      
      // Ensure it contains the canonical slice 1.3 paths
      expect(childPaths).toContain('store');
      expect(childPaths).toContain('devices');
      expect(childPaths).toContain('wallet');
      expect(childPaths).toContain('docs');
    });
  });
});
