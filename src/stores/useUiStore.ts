import { create } from 'zustand';

interface UiState {
  isSidebarCollapsed: boolean;
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;

  gridColumns: number;
  setGridColumns: (cols: number) => void;

  selectedDeviceIds: string[];
  toggleSelectDevice: (deviceId: string) => void;
  selectAllDevices: (deviceIds: string[]) => void;
  clearDeviceSelection: () => void;

  featureRentalStore: boolean;
  setFeatureRentalStore: (enabled: boolean) => void;
}

export const useUiStore = create<UiState>((set) => ({
  isSidebarCollapsed: false,
  toggleSidebar: () => set((state) => ({ isSidebarCollapsed: !state.isSidebarCollapsed })),
  setSidebarCollapsed: (collapsed) => set({ isSidebarCollapsed: collapsed }),

  gridColumns: 3,
  setGridColumns: (cols) => set({ gridColumns: cols }),

  selectedDeviceIds: [],
  toggleSelectDevice: (deviceId) =>
    set((state) => ({
      selectedDeviceIds: state.selectedDeviceIds.includes(deviceId)
        ? state.selectedDeviceIds.filter((id) => id !== deviceId)
        : [...state.selectedDeviceIds, deviceId],
    })),
  selectAllDevices: (deviceIds) => set({ selectedDeviceIds: deviceIds }),
  clearDeviceSelection: () => set({ selectedDeviceIds: [] }),

  featureRentalStore: import.meta.env.VITE_FEATURE_RENTAL_STORE === 'true',
  setFeatureRentalStore: (enabled) => set({ featureRentalStore: enabled }),
}));
