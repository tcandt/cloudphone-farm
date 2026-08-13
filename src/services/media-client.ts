import { StreamProfile, StreamSession } from '../types';

export interface MediaClient {
  sessionId: string;
  startSession(deviceId: string, profile?: StreamProfile): Promise<StreamSession>;
  attach(element: HTMLCanvasElement | HTMLVideoElement): void;
  setProfile(profile: StreamProfile): Promise<void>;
  stop(): Promise<void>;
  simulateTouch(x: number, y: number): void;
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

      // Draw simulated phone screen background
      ctx.fillStyle = '#0f172a';
      ctx.fillRect(0, 0, width, height);

      // Status Bar at Top
      ctx.fillStyle = 'rgba(255, 255, 255, 0.15)';
      ctx.fillRect(0, 0, width, 28);
      ctx.fillStyle = '#ffffff';
      ctx.font = '10px sans-serif';

      const now = new Date();
      const timeStr = now.toTimeString().split(' ')[0];
      ctx.fillText(timeStr, 12, 18);
      ctx.fillText(`98% ⚡`, width - 45, 18);

      // App Header / Wallpaper
      const gradient = ctx.createLinearGradient(0, 28, 0, height);
      gradient.addColorStop(0, '#1e293b');
      gradient.addColorStop(1, '#0f172a');
      ctx.fillStyle = gradient;
      ctx.fillRect(0, 28, width, height - 28);

      // Simulated Phone App Icons / Grid
      const iconSize = Math.min(width * 0.14, 48);
      const cols = 4;
      const marginX = (width - cols * iconSize) / (cols + 1);

      const appIcons = [
        { name: 'Phone', color: '#22c55e' },
        { name: 'Messages', color: '#3b82f6' },
        { name: 'Chrome', color: '#eab308' },
        { name: 'Gallery', color: '#ec4899' },
        { name: 'Camera', color: '#64748b' },
        { name: 'Settings', color: '#94a3b8' },
        { name: 'Agent', color: '#2563eb' },
        { name: 'Logs', color: '#8b5cf6' },
      ];

      appIcons.forEach((app, idx) => {
        const col = idx % cols;
        const row = Math.floor(idx / cols);
        const x = marginX + col * (iconSize + marginX);
        const y = 60 + row * (iconSize + 28);

        ctx.fillStyle = app.color;
        ctx.beginPath();
        ctx.roundRect(x, y, iconSize, iconSize, 12);
        ctx.fill();

        ctx.fillStyle = '#cbd5e1';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(app.name, x + iconSize / 2, y + iconSize + 14);
      });

      // Bottom Navigation Bar
      ctx.textAlign = 'left';
      ctx.fillStyle = 'rgba(255, 255, 255, 0.1)';
      ctx.fillRect(0, height - 40, width, 40);

      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 2;

      const navY = height - 20;
      ctx.beginPath();
      ctx.moveTo(width * 0.25 + 6, navY - 6);
      ctx.lineTo(width * 0.25 - 4, navY);
      ctx.lineTo(width * 0.25 + 6, navY + 6);
      ctx.stroke();

      ctx.beginPath();
      ctx.arc(width * 0.5, navY, 6, 0, Math.PI * 2);
      ctx.stroke();

      ctx.strokeRect(width * 0.75 - 5, navY - 5, 10, 10);

      // Render Touch Ripples
      const nowTime = Date.now();
      this.touches = this.touches.filter((t) => nowTime - t.time < 600);
      this.touches.forEach((t) => {
        const age = (nowTime - t.time) / 600;
        const radius = age * 25 + 5;
        const alpha = 1 - age;

        ctx.strokeStyle = `rgba(59, 130, 246, ${alpha})`;
        ctx.fillStyle = `rgba(59, 130, 246, ${alpha * 0.3})`;
        ctx.lineWidth = 2;

        ctx.beginPath();
        ctx.arc(t.x * width, t.y * height, radius, 0, Math.PI * 2);
        ctx.fill();
        ctx.stroke();
      });

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
      const client = new MockMediaClient(sessionId);
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
    for (const [id, entry] of this.instances.entries()) {
      await entry.client.stop();
      this.instances.delete(id);
    }
  }
}

export const defaultMediaRegistry = new DefaultMediaClientRegistry();
export const defaultMediaClient = defaultMediaRegistry.acquire('str_default_global');
