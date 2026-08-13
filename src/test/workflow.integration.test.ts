import { describe, it, expect } from 'vitest';
import { defaultMediaRegistry } from '../services/media-client';
import { defaultCommandEngine, canonicalJsonStringify, SUPPORTED_COMMAND_TYPES } from '../services/command-engine';
import { defaultWsSimulator } from '../services/websocket-simulator';
import { mockDevices, mockCurrentUserSession } from '../data/mockData';
import { ControlLease, DeviceEntity } from '../types';
import { MockAuthService, HttpAuthService } from '../services/auth-service';

describe('Phone Control Platform — Workflow & Contract Hardening Integration Tests', () => {
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

    // 3. Register Control Lease
    const leaseId = `lease_test_${Math.random().toString(36).substring(2, 8)}`;
    const lease: ControlLease = {
      control_lease_id: leaseId,
      device_id: targetDevice.device_id,
      organization_id: mockCurrentUserSession.organization_id,
      user_id: mockCurrentUserSession.user_id,
      user_display_name: mockCurrentUserSession.display_name,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    };
    defaultCommandEngine.registerLease(lease);

    const now = new Date();

    // 4. Command execution dispatch with strict DispatchCommandRequest contract
    const command = defaultCommandEngine.dispatch(
      {
        deviceId: targetDevice.device_id,
        type: 'global.home',
        payload: { action: 'home' },
        controlLeaseId: leaseId,
        idempotencyKey: `idemp_test_${Math.random().toString(36).substring(2, 8)}`,
        issuedAt: now.toISOString(),
        expiresAt: new Date(now.getTime() + 10000).toISOString(),
      },
      mockCurrentUserSession
    );

    expect(command.command_id).toMatch(/^cmd_/);
    expect(command.device_id).toBe(targetDevice.device_id);

    // 5. WebSocket presence event publishing
    let eventReceived = false;
    const unsubscribe = defaultWsSimulator.subscribe('command.updated', (evt) => {
      if (evt.payload?.command_id === command.command_id) {
        eventReceived = true;
      }
    });

    defaultWsSimulator.publish({
      event_id: 'evt_test_01',
      event_type: 'command.updated',
      organization_id: mockCurrentUserSession.organization_id,
      timestamp: new Date().toISOString(),
      payload: { command_id: command.command_id },
    });

    expect(eventReceived).toBe(true);
    unsubscribe();

    // 6. Cleanup session via Registry
    await defaultMediaRegistry.release(`str_${targetDevice.device_id}`);
  });

  it('Rejects interactive command when control lease is missing', () => {
    const targetDevice = mockDevices[0];
    const now = new Date();

    expect(() =>
      defaultCommandEngine.dispatch(
        {
          deviceId: targetDevice.device_id,
          type: 'gesture.touch',
          payload: { x: 0.5, y: 0.5 },
          idempotencyKey: `idemp_fail_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('CONTROL_LEASE_REQUIRED');
  });

  it('Reuses cached command response when same idempotency key and identical payload are sent', () => {
    const targetDevice = mockDevices[0];
    const leaseId = `lease_idemp_${Math.random().toString(36).substring(2, 8)}`;
    defaultCommandEngine.registerLease({
      control_lease_id: leaseId,
      device_id: targetDevice.device_id,
      organization_id: mockCurrentUserSession.organization_id,
      user_id: mockCurrentUserSession.user_id,
      user_display_name: mockCurrentUserSession.display_name,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    });

    const now = new Date();
    const idempKey = `idemp_reuse_${Math.random().toString(36).substring(2, 8)}`;

    const req1 = {
      deviceId: targetDevice.device_id,
      type: 'global.back' as const,
      payload: { b: 2, a: 1 },
      controlLeaseId: leaseId,
      idempotencyKey: idempKey,
      issuedAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + 10000).toISOString(),
    };

    const cmd1 = defaultCommandEngine.dispatch(req1, mockCurrentUserSession);

    // Re-send with key order swapped in payload object (canonical JSON test)
    const req2 = {
      ...req1,
      payload: { a: 1, b: 2 },
    };

    const cmd2 = defaultCommandEngine.dispatch(req2, mockCurrentUserSession);
    expect(cmd2.command_id).toBe(cmd1.command_id);
  });

  it('Throws IDEMPOTENCY_CONFLICT when idempotency key is reused with different request payload', () => {
    const targetDevice = mockDevices[0];
    const leaseId = `lease_idemp_conflict_${Math.random().toString(36).substring(2, 8)}`;
    defaultCommandEngine.registerLease({
      control_lease_id: leaseId,
      device_id: targetDevice.device_id,
      organization_id: mockCurrentUserSession.organization_id,
      user_id: mockCurrentUserSession.user_id,
      user_display_name: mockCurrentUserSession.display_name,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    });

    const now = new Date();
    const idempKey = `idemp_conflict_${Math.random().toString(36).substring(2, 8)}`;

    defaultCommandEngine.dispatch(
      {
        deviceId: targetDevice.device_id,
        type: 'global.back',
        payload: { action: 'back_v1' },
        controlLeaseId: leaseId,
        idempotencyKey: idempKey,
        issuedAt: now.toISOString(),
        expiresAt: new Date(now.getTime() + 10000).toISOString(),
      },
      mockCurrentUserSession
    );

    expect(() =>
      defaultCommandEngine.dispatch(
        {
          deviceId: targetDevice.device_id,
          type: 'global.back',
          payload: { action: 'back_v2_altered' },
          controlLeaseId: leaseId,
          idempotencyKey: idempKey,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('IDEMPOTENCY_CONFLICT');
  });

  it('Rejects unknown/unsupported command types with COMMAND_TYPE_UNSUPPORTED', () => {
    const targetDevice = mockDevices[0];
    const now = new Date();

    expect(() =>
      defaultCommandEngine.dispatch(
        {
          deviceId: targetDevice.device_id,
          type: 'unsupported.custom.action' as unknown as typeof SUPPORTED_COMMAND_TYPES[number],
          payload: {},
          idempotencyKey: `idemp_unsupp_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('COMMAND_TYPE_UNSUPPORTED');
  });

  it('Enforces granular capability checking per command type', () => {
    // Create mock device with disabled capabilities
    const limitedDevice: DeviceEntity = {
      ...mockDevices[0],
      device_id: 'dev_limited_01',
      organization_id: mockCurrentUserSession.organization_id,
      status: 'online',
      capabilities: {
        capture: { supported: false, codecs: [], max_width: 0, max_height: 0, max_fps: 0 },
        control: {
          supported: true,
          touch: false,
          swipe: false,
          global_actions: [],
          text_input: 'none',
          sensitive_actions: false,
        },
        telemetry: [],
        transport: [],
      },
    };

    mockDevices.push(limitedDevice);

    const leaseId = `lease_limited_${Math.random().toString(36).substring(2, 8)}`;
    defaultCommandEngine.registerLease({
      control_lease_id: leaseId,
      device_id: limitedDevice.device_id,
      organization_id: mockCurrentUserSession.organization_id,
      user_id: mockCurrentUserSession.user_id,
      user_display_name: mockCurrentUserSession.display_name,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    });

    const now = new Date();

    // 1. Touch unsupported
    expect(() =>
      defaultCommandEngine.dispatch(
        {
          deviceId: limitedDevice.device_id,
          type: 'gesture.touch',
          payload: { x: 0.5, y: 0.5 },
          controlLeaseId: leaseId,
          idempotencyKey: `idemp_lim_1_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('CAPABILITY_UNSUPPORTED');

    // 2. Text input unsupported
    expect(() =>
      defaultCommandEngine.dispatch(
        {
          deviceId: limitedDevice.device_id,
          type: 'input.text',
          payload: { text: 'hello' },
          controlLeaseId: leaseId,
          idempotencyKey: `idemp_lim_2_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('CAPABILITY_UNSUPPORTED');

    // 3. Screen capture unsupported
    expect(() =>
      defaultCommandEngine.dispatch(
        {
          deviceId: limitedDevice.device_id,
          type: 'screen.capture',
          payload: {},
          idempotencyKey: `idemp_lim_3_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('CAPABILITY_UNSUPPORTED');

    // Cleanup mock device array
    const idx = mockDevices.findIndex((d) => d.device_id === 'dev_limited_01');
    if (idx !== -1) mockDevices.splice(idx, 1);
  });

  it('Verifies canonicalJsonStringify handles key ordering deterministically', () => {
    const obj1 = { z: 1, a: { y: 'test', x: [2, 1] } };
    const obj2 = { a: { x: [2, 1], y: 'test' }, z: 1 };

    expect(canonicalJsonStringify(obj1)).toBe(canonicalJsonStringify(obj2));
    expect(canonicalJsonStringify(obj1)).toBe('{"a":{"x":[2,1],"y":"test"},"z":1}');
  });

  it('Verifies AuthService implementations (Mock vs Http zero-trust)', async () => {
    const mockAuth = new MockAuthService();
    const session = await mockAuth.login('test@pcp.io', 'pass');
    expect(session.user_id).toBe('usr_owner_01');

    const httpAuth = new HttpAuthService();
    expect(httpAuth).toBeInstanceOf(HttpAuthService);
  });
});
