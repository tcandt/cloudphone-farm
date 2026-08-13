import { describe, it, expect } from 'vitest';
import { defaultMediaRegistry } from '../services/media-client';
import { defaultCommandEngine } from '../services/command-engine';
import { defaultWsSimulator } from '../services/websocket-simulator';
import { mockDevices, mockCurrentUserSession } from '../data/mockData';

describe('Phone Control Platform — Workflow Integration Test', () => {
  it('Executes full remote device control lease & command dispatch lifecycle', async () => {
    // 1. Target device selection
    const targetDevice = mockDevices[0];
    expect(targetDevice.status).toBe('online');

    // 2. Media Client stream session initiation via Registry
    const mediaClient = defaultMediaRegistry.acquire(`str_${targetDevice.device_id}`);
    const streamSession = await mediaClient.startSession(targetDevice.device_id, {
      resolution: '720p',
      fps: 30,
      bitrate_kbps: 2000,
    });
    expect(streamSession.status).toBe('connected');

    // 3. Acquire Control Lease
    const leaseId = `lease_test_${Math.random().toString(36).substring(2, 8)}`;
    const now = new Date();

    // 4. Command execution dispatch with strict DispatchCommandRequest contract
    const command = await defaultCommandEngine.dispatchCommand({
      deviceId: targetDevice.device_id,
      type: 'global.home',
      payload: { action: 'home' },
      controlLeaseId: leaseId,
      idempotencyKey: `idemp_test_${Math.random().toString(36).substring(2, 8)}`,
      issuedAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + 10000).toISOString(),
    });

    expect(command.command_id).toMatch(/^cmd_/);
    expect(command.device_id).toBe(targetDevice.device_id);

    // 5. WebSocket presence event publishing
    let eventReceived = false;
    const unsubscribe = defaultWsSimulator.subscribe('command.updated', (evt) => {
      if (evt.data.command?.command_id === command.command_id) {
        eventReceived = true;
      }
    });

    defaultWsSimulator.publish({
      event_id: 'evt_test_01',
      event_type: 'command.updated',
      organization_id: mockCurrentUserSession.organization_id,
      timestamp: new Date().toISOString(),
      data: { command },
    });

    expect(eventReceived).toBe(true);
    unsubscribe();

    // 6. Cleanup session via Registry
    await defaultMediaRegistry.release(`str_${targetDevice.device_id}`);
  });

  it('Rejects interactive command when control lease is missing', async () => {
    const targetDevice = mockDevices[0];
    const now = new Date();

    await expect(
      defaultCommandEngine.dispatchCommand({
        deviceId: targetDevice.device_id,
        type: 'gesture.touch',
        payload: { x: 0.5, y: 0.5 },
        // controlLeaseId omitted intentionally
        idempotencyKey: `idemp_fail_${Math.random().toString(36).substring(2, 8)}`,
        issuedAt: now.toISOString(),
        expiresAt: new Date(now.getTime() + 10000).toISOString(),
      })
    ).rejects.toThrow('CONTROL_LEASE_REQUIRED');
  });
});
