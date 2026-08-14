import { StreamProfile, StreamSession } from '../types';
import { WebRtcMediaClient } from './webrtc-media-client';

export interface MediaClient {
  sessionId: string;
  startSession(deviceId: string, profile?: StreamProfile): Promise<StreamSession>;
  attach(element: HTMLCanvasElement | HTMLVideoElement): void;
  setProfile(profile: StreamProfile): Promise<void>;
  stop(): Promise<void>;
  simulateTouch(x: number, y: number): void;
  getWebRtcClient?(): WebRtcMediaClient | null;
}

export class ProductionWebRtcMediaClient implements MediaClient {
  public sessionId: string;
  private webRtcClient: WebRtcMediaClient | null = null;
  private currentElement: HTMLVideoElement | HTMLCanvasElement | null = null;
  private profile: StreamProfile = {
    resolution: '720p',
    fps: 30,
    bitrate_kbps: 2500,
  };

  constructor(sessionId: string) {
    this.sessionId = sessionId;
  }

  async startSession(deviceId: string, profile?: StreamProfile): Promise<StreamSession> {
    if (profile) {
      this.profile = profile;
    }

    if (!this.webRtcClient) {
      this.webRtcClient = new WebRtcMediaClient({
        deviceId,
        onStreamReady: (stream) => {
          if (this.currentElement && this.currentElement instanceof HTMLVideoElement) {
            this.currentElement.srcObject = stream;
            this.currentElement.play().catch(() => {});
          }
        },
      });
      this.webRtcClient.startSession();
    }

    const session: StreamSession = {
      stream_session_id: this.sessionId,
      device_id: deviceId,
      organization_id: 'org_tenant_01',
      user_id: 'usr_op_01',
      profile: this.profile,
      status: 'connected',
      started_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 3600 * 1000).toISOString(),
    };

    return session;
  }

  attach(element: HTMLCanvasElement | HTMLVideoElement): void {
    this.currentElement = element;
    if (element instanceof HTMLVideoElement && this.webRtcClient) {
      const stream = this.webRtcClient.getMediaStream();
      if (stream) {
        element.srcObject = stream;
        element.play().catch(() => {});
      }
    }
  }

  async setProfile(profile: StreamProfile): Promise<void> {
    this.profile = profile;
  }

  async stop(): Promise<void> {
    if (this.webRtcClient) {
      this.webRtcClient.close();
      this.webRtcClient = null;
    }
    if (this.currentElement && this.currentElement instanceof HTMLVideoElement) {
      this.currentElement.srcObject = null;
    }
    this.currentElement = null;
  }

  simulateTouch(): void {
    // Touch gestures dispatched via Command API
  }

  getWebRtcClient(): WebRtcMediaClient | null {
    return this.webRtcClient;
  }
}

export class MockMediaClient implements MediaClient {
  public sessionId: string;
  private activeDeviceId: string | null = null;
  private currentElement: HTMLCanvasElement | HTMLVideoElement | null = null;
  private animFrameId: number | null = null;
  private touches: { x: number; y: number; time: number }[] = [];
  private profile: StreamProfile = {
    resolution: '480p',
    fps: 30,
    bitrate_kbps: 1500,
  };
  private isRunning = false;

  constructor(sessionId: string) {
    this.sessionId = sessionId;
  }

  async startSession(deviceId: string, profile?: StreamProfile): Promise<StreamSession> {
    this.activeDeviceId = deviceId;
    if (profile) {
      this.profile = profile;
    }
    this.isRunning = true;

    const session: StreamSession = {
      stream_session_id: this.sessionId,
      device_id: deviceId,
      organization_id: 'org_demo_01',
      user_id: 'usr_op_01',
      profile: this.profile,
      status: 'connected',
      started_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 3600 * 1000).toISOString(),
    };

    return session;
  }

  attach(element: HTMLCanvasElement | HTMLVideoElement): void {
    this.currentElement = element;
    if (element instanceof HTMLCanvasElement) {
      this.startCanvasRender(element);
    }
  }

  async setProfile(profile: StreamProfile): Promise<void> {
    this.profile = profile;
  }

  async stop(): Promise<void> {
    this.isRunning = false;
    if (this.animFrameId !== null) {
      cancelAnimationFrame(this.animFrameId);
      this.animFrameId = null;
    }
    this.activeDeviceId = null;
    this.currentElement = null;
  }

  simulateTouch(x: number, y: number): void {
    this.touches.push({ x, y, time: Date.now() });
    if (this.touches.length > 5) {
      this.touches.shift();
    }
  }

  private startCanvasRender(canvas: HTMLCanvasElement): void {
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const render = () => {
      if (!this.isRunning) return;

      const width = canvas.width || 360;
      const height = canvas.height || 640;

      ctx.fillStyle = '#0f172a';
      ctx.fillRect(0, 0, width, height);

      ctx.fillStyle = 'rgba(255, 255, 255, 0.15)';
      ctx.fillRect(0, 0, width, 28);
      ctx.fillStyle = '#ffffff';
      ctx.font = '10px sans-serif';

      const now = new Date();
      const timeStr = now.toTimeString().split(' ')[0];
      ctx.fillText(timeStr, 12, 18);
      ctx.fillText(`98% ⚡`, width - 45, 18);

      const gradient = ctx.createLinearGradient(0, 28, 0, height);
      gradient.addColorStop(0, '#1e293b');
      gradient.addColorStop(1, '#0f172a');
      ctx.fillStyle = gradient;
      ctx.fillRect(0, 28, width, height - 28);

      ctx.fillStyle = 'rgba(0, 0, 0, 0.6)';
      ctx.fillRect(8, 36, 120, 20);
      ctx.fillStyle = '#4ade80';
      ctx.font = '10px monospace';
      ctx.fillText(`LIVE ${this.profile.fps}fps | ${this.profile.resolution}`, 12, 50);

      this.animFrameId = requestAnimationFrame(render);
    };

    render();
  }
}

interface MediaClientEntry {
  client: MediaClient;
  refCount: number;
}

export class DefaultMediaClientRegistry {
  private instances = new Map<string, MediaClientEntry>();

  acquire(sessionId: string): MediaClient {
    let entry = this.instances.get(sessionId);
    if (!entry) {
      const isTestEnv = typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.MODE === 'test';
      const client = isTestEnv ? new MockMediaClient(sessionId) : new ProductionWebRtcMediaClient(sessionId);
      entry = { client, refCount: 0 };
      this.instances.set(sessionId, entry);
    }
    entry.refCount += 1;
    return entry.client;
  }

  async release(sessionId: string): Promise<void> {
    const entry = this.instances.get(sessionId);
    if (!entry) return;

    entry.refCount -= 1;
    if (entry.refCount <= 0) {
      await entry.client.stop();
      this.instances.delete(sessionId);
    }
  }

  async closeAll(): Promise<void> {
    for (const entry of this.instances.values()) {
      await entry.client.stop();
    }
    this.instances.clear();
  }
}

export const defaultMediaRegistry = new DefaultMediaClientRegistry();
