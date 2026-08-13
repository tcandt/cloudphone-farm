import { describe, it, expect } from 'vitest';
import { defaultMediaClient } from '../services/media-client';
import { defaultCommandEngine } from '../services/command-engine';
import { defaultWsSimulator } from '../services/websocket-simulator';
import { mockDevices, mockCurrentUserSession } from '../data/mockData';

describe('Phone Control Platform — End-to-End Workflow Integration Test', () => {
  it('Executes full remote device control lease & command dispatch lifecycle', async () => {
    // 1. Target device selection
    const targetDevice = mockDevices[0];
    expect(targetDevice.status).toBe('online');

    // 2. Media Client stream session initiation
    const streamSession = await defaultMediaClient.startSession(targetDevice.device_id, {
      resolution: '720p',
      fps: 30,
      bitrate_kbps: 2000,
    });
    expect(streamSession.status).toBe('connected');

    // 3. Command execution simulation
    const command = await defaultCommandEngine.dispatchCommand(targetDevice.device_id, 'global_action', {
      action: 'home',
    });
    expect(command.command_id).toMatch(/^cmd_/);
    expect(command.device_id).toBe(targetDevice.device_id);

    // 4. WebSocket presence event publishing
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

    // 5. Cleanup session
    await defaultMediaClient.stop();
  });
});
