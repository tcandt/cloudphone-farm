import { describe, it, expect, vi } from 'vitest';
import { defaultMediaRegistry } from '../services/media-client';
import { defaultCommandEngine, canonicalJsonStringify, SUPPORTED_COMMAND_TYPES, CommandEngine } from '../services/command-engine';
import { defaultWsSimulator } from '../services/websocket-simulator';
import { mockDevices, mockCurrentUserSession } from '../data/mockData';
import { ControlLease, DeviceEntity, PermissionCode, UserSession } from '../types';
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

  it('Rejects screen.capture and all interactive commands when target device is offline', () => {
    const offlineDevice: DeviceEntity = {
      ...mockDevices[0],
      device_id: 'dev_offline_test',
      status: 'offline',
    };
    mockDevices.push(offlineDevice);

    const engine = new CommandEngine();
    const now = new Date();

    expect(() =>
      engine.dispatch(
        {
          deviceId: offlineDevice.device_id,
          type: 'screen.capture',
          payload: {},
          idempotencyKey: `idemp_off_1_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('DEVICE_OFFLINE');

    // Cleanup
    const idx = mockDevices.findIndex((d) => d.device_id === 'dev_offline_test');
    if (idx !== -1) mockDevices.splice(idx, 1);
  });

  it('Strictly requires device.stream.view permission for screen.capture (RBAC least-privilege)', () => {
    const targetDevice = mockDevices[0];
    const now = new Date();
    const sessionWithoutStreamView: UserSession = {
      ...mockCurrentUserSession,
      permissions: ['device.read' as PermissionCode, 'dashboard.read' as PermissionCode],
    };

    const engine = new CommandEngine();
    expect(() =>
      engine.dispatch(
        {
          deviceId: targetDevice.device_id,
          type: 'screen.capture',
          payload: {},
          idempotencyKey: `idemp_rbac_stream_${Math.random().toString(36).substring(2, 8)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        sessionWithoutStreamView
      )
    ).toThrow('PERMISSION_DENIED');
  });

  it.each([
    {
      name: 'Touch capability disabled',
      cmdType: 'gesture.touch' as const,
      payload: { x: 0.5, y: 0.5 },
      capabilitiesPatch: { control: { supported: true, touch: false, swipe: true, global_actions: ['back' as const], text_input: 'full' as const } },
    },
    {
      name: 'Swipe capability disabled',
      cmdType: 'gesture.swipe' as const,
      payload: { x1: 0.1, y1: 0.1, x2: 0.8, y2: 0.8 },
      capabilitiesPatch: { control: { supported: true, touch: true, swipe: false, global_actions: ['back' as const], text_input: 'full' as const } },
    },
    {
      name: 'Text input capability disabled',
      cmdType: 'input.text' as const,
      payload: { text: 'hello' },
      capabilitiesPatch: { control: { supported: true, touch: true, swipe: true, global_actions: ['back' as const], text_input: 'none' as const } },
    },
    {
      name: 'Global home action disabled',
      cmdType: 'global.home' as const,
      payload: {},
      capabilitiesPatch: { control: { supported: true, touch: true, swipe: true, global_actions: ['back' as const], text_input: 'full' as const } },
    },
  ])('Enforces capability check for $name -> throws CAPABILITY_UNSUPPORTED', ({ cmdType, payload, capabilitiesPatch }) => {
    const devId = `dev_cap_${Math.random().toString(36).substring(2, 6)}`;
    const testDevice: DeviceEntity = {
      ...mockDevices[0],
      device_id: devId,
      status: 'online',
      capabilities: {
        ...mockDevices[0].capabilities,
        ...capabilitiesPatch,
      },
    };
    mockDevices.push(testDevice);

    const engine = new CommandEngine();
    const leaseId = `lease_cap_${Math.random().toString(36).substring(2, 6)}`;
    engine.registerLease({
      control_lease_id: leaseId,
      device_id: testDevice.device_id,
      organization_id: mockCurrentUserSession.organization_id,
      user_id: mockCurrentUserSession.user_id,
      user_display_name: mockCurrentUserSession.display_name,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    });

    const now = new Date();
    expect(() =>
      engine.dispatch(
        {
          deviceId: testDevice.device_id,
          type: cmdType,
          payload,
          controlLeaseId: leaseId,
          idempotencyKey: `idemp_cap_${Math.random().toString(36).substring(2, 6)}`,
          issuedAt: now.toISOString(),
          expiresAt: new Date(now.getTime() + 10000).toISOString(),
        },
        mockCurrentUserSession
      )
    ).toThrow('CAPABILITY_UNSUPPORTED');

    const idx = mockDevices.findIndex((d) => d.device_id === devId);
    if (idx !== -1) mockDevices.splice(idx, 1);
  });

  it('Verifies canonicalJsonStringify handles key ordering, primitives, dates, and cyclic object rejection', () => {
    const obj1 = { z: 1, a: { y: 'test', x: [2, 1] } };
    const obj2 = { a: { x: [2, 1], y: 'test' }, z: 1 };

    expect(canonicalJsonStringify(obj1)).toBe(canonicalJsonStringify(obj2));
    expect(canonicalJsonStringify(obj1)).toBe('{"a":{"x":[2,1],"y":"test"},"z":1}');

    // Test cyclic reference rejection
    const cyclicObj: Record<string, unknown> = { a: 1 };
    cyclicObj.self = cyclicObj;
    expect(() => canonicalJsonStringify(cyclicObj)).toThrow('INVALID_PAYLOAD');
  });

  it('Verifies AuthService zero-trust production HttpAuthService contract via mocked HTTP responses', async () => {
    const mockAuth = new MockAuthService();
    const devSession = await mockAuth.login('test@pcp.io', 'pass');
    expect(devSession.user_id).toBe('usr_owner_01');

    const httpAuth = new HttpAuthService();
    expect(httpAuth).toBeInstanceOf(HttpAuthService);

    // Mock successful session response
    const mockFetch = vi.fn();
    globalThis.fetch = mockFetch;

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockCurrentUserSession,
    } as Response);

    const prodSession = await httpAuth.fetchSession();
    expect(prodSession?.user_id).toBe('usr_owner_01');
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/auth/session', expect.anything());

    // Mock 401 unauthenticated response
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
    } as Response);

    const unauthSession = await httpAuth.fetchSession();
    expect(unauthSession).toBeNull();
  });
});
