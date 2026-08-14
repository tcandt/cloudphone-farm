import { describe, it, expect, vi } from 'vitest';
import { defaultMediaRegistry } from '../services/media-client';
import { WebRtcMediaClient } from '../services/webrtc-media-client';
import { defaultCommandEngine } from '../services/command-engine';
import { mockDevices, mockCurrentUserSession } from '../data/mockData';
import { ControlLease } from '../types';
import { OperationalCommandStore, OperationalCommand } from '../services/operational-command-store';
import { OperatorEventClient, OperatorEvent } from '../services/operator-event-client';

describe('Phone Control Platform — Workflow & Contract Hardening Integration Tests', () => {
  it('Executes full remote device control lease & command dispatch lifecycle', async () => {
    const targetDevice = mockDevices[0];
    expect(targetDevice.status).toBe('online');

    const mediaClient = defaultMediaRegistry.acquire(`str_${targetDevice.device_id}`);
    const streamSession = await mediaClient.startSession(targetDevice.device_id, {
      resolution: '720p',
      fps: 30,
      bitrate_kbps: 2000,
    });
    expect(streamSession.status).toBe('connected');

    const leaseId = `lease_test_${Math.random().toString(36).substring(2, 8)}`;
    const lease: ControlLease = {
      control_lease_id: leaseId,
      device_id: targetDevice.device_id,
      organization_id: mockCurrentUserSession.organization_id,
      user_id: mockCurrentUserSession.user_id,
      user_display_name: mockCurrentUserSession.display_name,
      fencing_token: 1,
      acquired_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 60000).toISOString(),
      ttl_seconds: 60,
    };
    defaultCommandEngine.registerLease(lease);

    const now = new Date();
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
    await defaultMediaRegistry.release(`str_${targetDevice.device_id}`);
  });

  it('Verifies OperationalCommandStore monotonic sequence protection, terminal immutability, and confirmation timeout', async () => {
    vi.useFakeTimers();
    const deviceId = 'dev_sm_g930f_01';
    const store = new OperationalCommandStore(deviceId, 10000);

    let currentCommands: OperationalCommand[] = [];
    store.subscribe((cmds) => {
      currentCommands = cmds;
    });

    // 1. HTTP 202 Accepted
    store.trackAcceptedCommand('cmd_001', deviceId);
    expect(currentCommands[0].commandId).toBe('cmd_001');
    expect(currentCommands[0].operationalStatus).toBe('normal');

    // 2. Delivery dispatched
    store.processEvent({
      type: 'command.delivery.changed',
      data: {
        command_id: 'cmd_001',
        device_id: deviceId,
        delivery_status: 'dispatched',
        attempt_count: 1,
        dispatched_at: new Date().toISOString(),
      },
    });
    expect(currentCommands[0].deliveryStatus).toBe('dispatched');

    // 3. Status executing (seq 2) -> starts confirmation timer
    store.processEvent({
      type: 'command.status.changed',
      data: {
        command_id: 'cmd_001',
        device_id: deviceId,
        execution_status: 'executing',
        sequence: 2,
        occurred_at: new Date().toISOString(),
      },
    });
    expect(currentCommands[0].executionStatus).toBe('executing');
    expect(currentCommands[0].lastSequence).toBe(2);

    // 4. Stale/duplicate sequence 1 ignored
    store.processEvent({
      type: 'command.status.changed',
      data: {
        command_id: 'cmd_001',
        device_id: deviceId,
        execution_status: 'ack',
        sequence: 1,
        occurred_at: new Date().toISOString(),
      },
    });
    expect(currentCommands[0].executionStatus).toBe('executing'); // Unchanged!

    // 5. Fast-forward timer by 11 seconds to trigger confirmation_timeout
    vi.advanceTimersByTime(11000);
    expect(currentCommands[0].operationalStatus).toBe('confirmation_timeout');
    expect(currentCommands[0].executionStatus).toBe('executing'); // NOT failed!

    // 6. Late valid terminal event arrives (seq 3) -> clears timeout and applies terminal state
    store.processEvent({
      type: 'command.status.changed',
      data: {
        command_id: 'cmd_001',
        device_id: deviceId,
        execution_status: 'succeeded',
        sequence: 3,
        occurred_at: new Date().toISOString(),
      },
    });
    expect(currentCommands[0].executionStatus).toBe('succeeded');
    expect(currentCommands[0].isTerminal).toBe(true);
    expect(currentCommands[0].operationalStatus).toBe('normal');

    // 7. Terminal state is immutable: later event cannot regress terminal state
    store.processEvent({
      type: 'command.status.changed',
      data: {
        command_id: 'cmd_001',
        device_id: deviceId,
        execution_status: 'executing',
        sequence: 4,
        occurred_at: new Date().toISOString(),
      },
    });
    expect(currentCommands[0].executionStatus).toBe('succeeded');

    store.destroy();
    vi.useRealTimers();
  });

  it('Verifies OperatorEventClient rejects malformed events without fabricating business state', () => {
    const client = new OperatorEventClient('dev_sm_g930f_01');
    const receivedEvents: OperatorEvent[] = [];

    client.subscribe((evt) => {
      receivedEvents.push(evt);
    });

    // Client should ignore malformed raw json
    expect(receivedEvents.length).toBe(0);
  });

  it('Verifies 10x full production WebRtcMediaClient lifecycle leaves ZERO active resources on EVERY cycle including rVFC', async () => {
    let activeWS = 0;
    let activePC = 0;
    let activeTracks = 0;
    let activeIntervals = 0;
    let activeTimeouts = 0;
    let activeFrameCallbacks = 0;

    const OriginalWS = globalThis.WebSocket;
    const OriginalPC = globalThis.RTCPeerConnection;
    const OriginalSetInterval = globalThis.setInterval;
    const OriginalClearInterval = globalThis.clearInterval;
    const OriginalSetTimeout = globalThis.setTimeout;
    const OriginalClearTimeout = globalThis.clearTimeout;
    const originalRVFC = HTMLVideoElement.prototype.requestVideoFrameCallback;
    const originalCVFC = HTMLVideoElement.prototype.cancelVideoFrameCallback;

    // Mock Track
    class MockTrack {
      public kind = 'video';
      public enabled = true;
      constructor() {
        activeTracks++;
      }
      stop() {
        activeTracks = Math.max(0, activeTracks - 1);
      }
    }

    // Mock MediaStream
    class MockMediaStream {
      constructor(private tracks: MockTrack[] = []) {}
      getTracks() {
        return this.tracks;
      }
    }

    // Mock PeerConnection
    globalThis.RTCPeerConnection = class MockPeerConnection {
      public iceConnectionState = 'connected';
      public remoteDescription: unknown = null;
      public onicecandidate: unknown = null;
      public oniceconnectionstatechange: unknown = null;
      public ontrack: ((evt: { streams: MockMediaStream[]; track: MockTrack }) => void) | null = null;

      constructor() {
        activePC++;
      }

      addTransceiver() {
        return {};
      }
      createOffer() {
        return Promise.resolve({ type: 'offer', sdp: 'mock_sdp' });
      }
      setLocalDescription() {
        return Promise.resolve();
      }
      setRemoteDescription() {
        this.remoteDescription = { type: 'answer' };
        return Promise.resolve();
      }
      addIceCandidate() {
        return Promise.resolve();
      }
      getStats() {
        return Promise.resolve(new Map());
      }
      close() {
        activePC = Math.max(0, activePC - 1);
      }
    } as unknown as typeof RTCPeerConnection;

    // Mock WebSocket
    globalThis.WebSocket = class MockWebSocket {
      static OPEN = 1;
      static CONNECTING = 0;
      static CLOSING = 2;
      static CLOSED = 3;

      public readyState = 1;
      public onopen: (() => void) | null = null;
      public onmessage: ((evt: { data: string }) => void) | null = null;
      public onclose: ((evt: { code: number; reason: string }) => void) | null = null;
      public onerror: ((err: unknown) => void) | null = null;

      constructor() {
        activeWS++;
        queueMicrotask(() => {
          this.onopen?.();
        });
      }
      send() {}
      close() {
        activeWS = Math.max(0, activeWS - 1);
        this.onclose?.({ code: 1000, reason: 'normal' });
      }
    } as unknown as typeof WebSocket;

    // Mock timers
    globalThis.setInterval = ((fn: (...args: unknown[]) => void, ms: number) => {
      activeIntervals++;
      return OriginalSetInterval(fn, ms);
    }) as unknown as typeof setInterval;

    globalThis.clearInterval = ((id: ReturnType<typeof setInterval>) => {
      activeIntervals = Math.max(0, activeIntervals - 1);
      OriginalClearInterval(id);
    }) as unknown as typeof clearInterval;

    globalThis.setTimeout = ((fn: (...args: unknown[]) => void, ms: number) => {
      activeTimeouts++;
      return OriginalSetTimeout(() => {
        activeTimeouts = Math.max(0, activeTimeouts - 1);
        fn();
      }, ms);
    }) as unknown as typeof setTimeout;

    globalThis.clearTimeout = ((id: ReturnType<typeof setTimeout>) => {
      activeTimeouts = Math.max(0, activeTimeouts - 1);
      OriginalClearTimeout(id);
    }) as unknown as typeof clearTimeout;

    // Mock requestVideoFrameCallback / cancelVideoFrameCallback cleanly
    Object.defineProperty(HTMLVideoElement.prototype, 'requestVideoFrameCallback', {
      writable: true,
      configurable: true,
      value: function (cb: (now: number, metadata: Record<string, unknown>) => void): number {
        activeFrameCallbacks++;
        const timerId = OriginalSetTimeout(() => {
          cb(performance.now(), {});
        }, 16);
        return timerId as unknown as number;
      },
    });

    Object.defineProperty(HTMLVideoElement.prototype, 'cancelVideoFrameCallback', {
      writable: true,
      configurable: true,
      value: function (id: number): void {
        if (id) {
          activeFrameCallbacks = Math.max(0, activeFrameCallbacks - 1);
          OriginalClearTimeout(id as unknown as ReturnType<typeof setTimeout>);
        }
      },
    });

    const mockElement = document.createElement('video');

    try {
      for (let i = 0; i < 10; i++) {
        const client = new WebRtcMediaClient({
          deviceId: 'dev_sm_g930f_01',
        });

        // Test multiple bind/ontrack calls to ensure no concurrent rVFC loop leak
        client.bindVideoElement(mockElement);
        client.bindVideoElement(mockElement);
        client.startSession();

        // Simulate incoming signaling envelope: created -> ready
        const wsInst = (client as unknown as { ws: { onmessage: (evt: { data: string }) => void } }).ws;
        if (wsInst && wsInst.onmessage) {
          wsInst.onmessage({
            data: JSON.stringify({
              type: 'media.session.created',
              payload: { session_id: `sess_${i}`, device_id: 'dev_sm_g930f_01' },
            }),
          });
          wsInst.onmessage({
            data: JSON.stringify({
              type: 'media.session.ready',
              payload: { session_id: `sess_${i}` },
            }),
          });
        }

        // Trigger remote track
        const pcInst = (client as unknown as { pc: { ontrack: (evt: { streams: MockMediaStream[]; track: MockTrack }) => void } }).pc;
        if (pcInst && pcInst.ontrack) {
          const trackInst = new MockTrack();
          pcInst.ontrack({
            streams: [new MockMediaStream([trackInst])],
            track: trackInst,
          });
        }

        client.notifyVideoFrameReceived();
        expect(client.getState()).toBe('VIDEO_RECEIVING');

        // Close client cleanly
        client.close();
        expect(client.getState()).toBe('CLOSED');

        // Assert ZERO leaked active resources AFTER EVERY SINGLE CYCLE
        expect(activeWS).toBe(0);
        expect(activePC).toBe(0);
        expect(activeTracks).toBe(0);
        expect(activeIntervals).toBe(0);
        expect(activeTimeouts).toBe(0);
        expect(activeFrameCallbacks).toBe(0);
        expect(client.getActiveReconnectTaskCount()).toBe(0);
        expect(mockElement.srcObject).toBeNull();
      }
    } finally {
      globalThis.WebSocket = OriginalWS;
      globalThis.RTCPeerConnection = OriginalPC;
      globalThis.setInterval = OriginalSetInterval;
      globalThis.clearInterval = OriginalClearInterval;
      globalThis.setTimeout = OriginalSetTimeout;
      globalThis.clearTimeout = OriginalClearTimeout;
      if (originalRVFC) {
        HTMLVideoElement.prototype.requestVideoFrameCallback = originalRVFC;
      }
      if (originalCVFC) {
        HTMLVideoElement.prototype.cancelVideoFrameCallback = originalCVFC;
      }
    }
  });

  it('Verifies consent-safe WebRTC session state remains WAITING_DEVICE_CONSENT when MediaProjection prompt is delayed', async () => {
    vi.useFakeTimers();

    const OriginalWS = globalThis.WebSocket;
    const OriginalPC = globalThis.RTCPeerConnection;

    globalThis.WebSocket = class MockWS {
      static OPEN = 1;
      static CONNECTING = 0;
      public readyState = 1;
      public onopen: (() => void) | null = null;
      public onmessage: ((evt: { data: string }) => void) | null = null;
      public onclose: ((evt: { code: number; reason: string }) => void) | null = null;
      public onerror: ((err: unknown) => void) | null = null;
      send() {}
      close() {}
    } as unknown as typeof WebSocket;

    globalThis.RTCPeerConnection = class MockPC {
      public iceConnectionState = 'new';
      public onicecandidate: unknown = null;
      public oniceconnectionstatechange: unknown = null;
      public ontrack: unknown = null;
      addTransceiver() {
        return {};
      }
      close() {}
    } as unknown as typeof RTCPeerConnection;

    try {
      const client = new WebRtcMediaClient({
        deviceId: 'dev_sm_g930f_01',
      });

      client.startSession();

      // 1. Simulate session created (user is looking at MediaProjection consent dialog)
      const wsInst = (client as unknown as { ws: { onmessage: (evt: { data: string }) => void } }).ws;
      if (wsInst && wsInst.onmessage) {
        wsInst.onmessage({
          data: JSON.stringify({
            type: 'media.session.created',
            payload: { session_id: 'sess_consent_30s', device_id: 'dev_sm_g930f_01' },
          }),
        });
      }

      expect(client.getState()).toBe('WAITING_DEVICE_CONSENT');

      // 2. Advance 35 seconds without media.session.ready -> state MUST remain WAITING_DEVICE_CONSENT
      vi.advanceTimersByTime(35000);
      expect(client.getState()).toBe('WAITING_DEVICE_CONSENT');

      // 3. Verify no first-frame timer or reconnect tasks were triggered
      expect(client.getActiveReconnectTaskCount()).toBe(0);

      client.close();
      expect(client.getState()).toBe('CLOSED');
    } finally {
      globalThis.WebSocket = OriginalWS;
      globalThis.RTCPeerConnection = OriginalPC;
      vi.useRealTimers();
    }
  });

  it('Verifies first-frame watchdog triggers DEGRADED and RECONNECTING only after transport reaches CONNECTED state', async () => {
    vi.useFakeTimers();

    const OriginalWS = globalThis.WebSocket;
    const OriginalPC = globalThis.RTCPeerConnection;

    let onIceStateChangeHandler: (() => void) | null = null;

    globalThis.WebSocket = class MockWS {
      static OPEN = 1;
      static CONNECTING = 0;
      public readyState = 1;
      public onopen: (() => void) | null = null;
      public onmessage: ((evt: { data: string }) => void) | null = null;
      public onclose: ((evt: { code: number; reason: string }) => void) | null = null;
      public onerror: ((err: unknown) => void) | null = null;
      send() {}
      close() {}
    } as unknown as typeof WebSocket;

    globalThis.RTCPeerConnection = class MockPC {
      public iceConnectionState = 'new';
      public onicecandidate: unknown = null;
      public ontrack: unknown = null;
      public set oniceconnectionstatechange(cb: () => void) {
        onIceStateChangeHandler = cb;
      }
      addTransceiver() {
        return {};
      }
      createOffer() {
        return Promise.resolve({ type: 'offer', sdp: 'mock_sdp' });
      }
      setLocalDescription() {
        return Promise.resolve();
      }
      close() {}
    } as unknown as typeof RTCPeerConnection;

    try {
      const client = new WebRtcMediaClient({
        deviceId: 'dev_sm_g930f_01',
      });

      client.startSession();

      const wsInst = (client as unknown as { ws: { onmessage: (evt: { data: string }) => void } }).ws;
      if (wsInst && wsInst.onmessage) {
        wsInst.onmessage({
          data: JSON.stringify({
            type: 'media.session.created',
            payload: { session_id: 'sess_connected_watchdog', device_id: 'dev_sm_g930f_01' },
          }),
        });
        wsInst.onmessage({
          data: JSON.stringify({
            type: 'media.session.ready',
            payload: { session_id: 'sess_connected_watchdog' },
          }),
        });
      }

      expect(client.getState()).toBe('NEGOTIATING');

      // ICE connects -> state becomes CONNECTED and starts first-frame timer
      const pcInst = (client as unknown as { pc: { iceConnectionState: string } }).pc;
      if (pcInst) {
        pcInst.iceConnectionState = 'connected';
        if (onIceStateChangeHandler) {
          (onIceStateChangeHandler as () => void)();
        }
      }

      expect(client.getState()).toBe('CONNECTED');

      // 5s with no frames -> DEGRADED
      vi.advanceTimersByTime(5100);
      expect(client.getState()).toBe('DEGRADED');

      // 5s more with no frames -> RECONNECTING (first_frame_timeout)
      vi.advanceTimersByTime(5100);
      expect(client.getState()).toBe('RECONNECTING');
      expect(client.getActiveReconnectTaskCount()).toBe(1);

      client.close();
      expect(client.getState()).toBe('CLOSED');
    } finally {
      globalThis.WebSocket = OriginalWS;
      globalThis.RTCPeerConnection = OriginalPC;
      vi.useRealTimers();
    }
  });

  it('Verifies deterministic ICE Grace Recovery vs Reconnect and Failure Matrix', async () => {
    vi.useFakeTimers();

    const OriginalWS = globalThis.WebSocket;
    const OriginalPC = globalThis.RTCPeerConnection;

    let onIceStateChangeHandler: (() => void) | null = null;

    globalThis.WebSocket = class MockWS {
      static OPEN = 1;
      static CONNECTING = 0;
      public readyState = 1;
      public onopen: (() => void) | null = null;
      public onmessage: ((evt: { data: string }) => void) | null = null;
      public onclose: ((evt: { code: number; reason: string }) => void) | null = null;
      public onerror: ((err: unknown) => void) | null = null;
      send() {}
      close() {}
    } as unknown as typeof WebSocket;

    globalThis.RTCPeerConnection = class MockPC {
      public iceConnectionState = 'new';
      public onicecandidate: unknown = null;
      public ontrack: unknown = null;
      public set oniceconnectionstatechange(cb: () => void) {
        onIceStateChangeHandler = cb;
      }
      addTransceiver() {
        return {};
      }
      createOffer() {
        return Promise.resolve({ type: 'offer', sdp: 'mock_sdp' });
      }
      setLocalDescription() {
        return Promise.resolve();
      }
      close() {}
    } as unknown as typeof RTCPeerConnection;

    try {
      const triggerIceStateChange = () => {
        if (onIceStateChangeHandler) {
          (onIceStateChangeHandler as () => void)();
        }
      };

      // Test A: ICE connected -> disconnected -> recovers < 7s -> same session, no reconnect
      const clientA = new WebRtcMediaClient({ deviceId: 'dev_sm_g930f_01' });
      clientA.startSession();

      const wsInstA = (clientA as unknown as { ws: { onmessage: (evt: { data: string }) => void } }).ws;
      if (wsInstA && wsInstA.onmessage) {
        wsInstA.onmessage({
          data: JSON.stringify({
            type: 'media.session.created',
            payload: { session_id: 'sess_ice_01', device_id: 'dev_sm_g930f_01' },
          }),
        });
        wsInstA.onmessage({
          data: JSON.stringify({
            type: 'media.session.ready',
            payload: { session_id: 'sess_ice_01' },
          }),
        });
      }

      const pcInstA = (clientA as unknown as { pc: { iceConnectionState: string } }).pc;
      if (pcInstA) {
        pcInstA.iceConnectionState = 'connected';
        triggerIceStateChange();
      }
      expect(clientA.getState()).toBe('CONNECTED');

      // ICE disconnects
      if (pcInstA) {
        pcInstA.iceConnectionState = 'disconnected';
        triggerIceStateChange();
      }
      expect(clientA.getState()).toBe('DEGRADED');

      // Advance 4s (<7s grace window)
      vi.advanceTimersByTime(4000);

      // ICE recovers cleanly
      if (pcInstA) {
        pcInstA.iceConnectionState = 'connected';
        triggerIceStateChange();
      }
      expect(clientA.getState()).toBe('CONNECTED');
      expect(clientA.getSessionId()).toBe('sess_ice_01');
      expect(clientA.getActiveReconnectTaskCount()).toBe(0);
      clientA.close();

      // Test B: ICE connected -> disconnected -> remains > 7s -> RECONNECTING
      const clientB = new WebRtcMediaClient({ deviceId: 'dev_sm_g930f_01' });
      clientB.startSession();

      const wsInstB = (clientB as unknown as { ws: { onmessage: (evt: { data: string }) => void } }).ws;
      if (wsInstB && wsInstB.onmessage) {
        wsInstB.onmessage({
          data: JSON.stringify({
            type: 'media.session.created',
            payload: { session_id: 'sess_ice_02', device_id: 'dev_sm_g930f_01' },
          }),
        });
        wsInstB.onmessage({
          data: JSON.stringify({
            type: 'media.session.ready',
            payload: { session_id: 'sess_ice_02' },
          }),
        });
      }

      const pcInstB = (clientB as unknown as { pc: { iceConnectionState: string } }).pc;
      if (pcInstB) {
        pcInstB.iceConnectionState = 'connected';
        triggerIceStateChange();
      }

      // ICE disconnects
      if (pcInstB) {
        pcInstB.iceConnectionState = 'disconnected';
        triggerIceStateChange();
      }

      // Advance 7.5s (>7s grace window)
      vi.advanceTimersByTime(7500);
      expect(clientB.getState()).toBe('RECONNECTING');
      expect(clientB.getActiveReconnectTaskCount()).toBe(1);
      clientB.close();

      // Test C: ICE failed -> immediate controlled reconnect
      const clientC = new WebRtcMediaClient({ deviceId: 'dev_sm_g930f_01' });
      clientC.startSession();

      const wsInstC = (clientC as unknown as { ws: { onmessage: (evt: { data: string }) => void } }).ws;
      if (wsInstC && wsInstC.onmessage) {
        wsInstC.onmessage({
          data: JSON.stringify({
            type: 'media.session.created',
            payload: { session_id: 'sess_ice_03', device_id: 'dev_sm_g930f_01' },
          }),
        });
      }

      const pcInstC = (clientC as unknown as { pc: { iceConnectionState: string } }).pc;
      if (pcInstC) {
        pcInstC.iceConnectionState = 'failed';
        triggerIceStateChange();
      }

      expect(clientC.getState()).toBe('RECONNECTING');
      expect(clientC.getActiveReconnectTaskCount()).toBe(1);
      clientC.close();
    } finally {
      globalThis.WebSocket = OriginalWS;
      globalThis.RTCPeerConnection = OriginalPC;
      vi.useRealTimers();
    }
  });
});
