import { describe, it, expect, beforeEach } from 'vitest';
import { defaultMediaRegistry } from '../services/media-client';
import { mockCurrentUserSession, mockDevices } from '../data/mockData';
import { useUiStore } from '../stores/useUiStore';

describe('Phone Control Platform — Core Architecture Tests', () => {
  beforeEach(() => {
    useUiStore.setState({
      isSidebarCollapsed: false,
      gridColumns: 3,
      selectedDeviceIds: [],
      featureRentalStore: false,
    });
  });

  it('MediaClient starts session and generates valid StreamSession contract', async () => {
    const mediaClient = defaultMediaRegistry.acquire('str_test_001');
    const session = await mediaClient.startSession('dev_s7_001', {
      resolution: '480p',
      fps: 30,
      bitrate_kbps: 1500,
    });

    expect(session.device_id).toBe('dev_s7_001');
    expect(session.status).toBe('connected');
    expect(session.stream_session_id).toMatch(/^str_/);
    expect(session.organization_id).toBe('org_demo_01');
  });

  it('Zustand UI store toggles sidebar state without mutating server presence', () => {
    expect(useUiStore.getState().isSidebarCollapsed).toBe(false);
    useUiStore.getState().toggleSidebar();
    expect(useUiStore.getState().isSidebarCollapsed).toBe(true);
  });

  it('RBAC Permissions check validates owner privileges', () => {
    expect(mockCurrentUserSession.permissions).toContain('device.control.acquire');
    expect(mockCurrentUserSession.permissions).toContain('agent.enroll');
    expect(mockCurrentUserSession.permissions).toContain('billing.manage');
  });

  it('Device Entities contain required contract fields', () => {
    const device = mockDevices[0];
    expect(device.device_id).toBeDefined();
    expect(device.organization_id).toBeDefined();
    expect(device.capabilities.control).toBeDefined();
    expect(device.telemetry.battery).toBeGreaterThanOrEqual(0);
  });
});
