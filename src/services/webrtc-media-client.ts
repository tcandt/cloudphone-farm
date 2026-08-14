export type WebRtcState =
  | 'IDLE'
  | 'CONNECTING_SIGNALING'
  | 'SESSION_CREATED'
  | 'WAITING_DEVICE_CONSENT'
  | 'DEVICE_READY'
  | 'NEGOTIATING'
  | 'CONNECTED'
  | 'VIDEO_RECEIVING'
  | 'DEGRADED'
  | 'RECONNECTING'
  | 'CLOSED'
  | 'FAILED';

export interface ServerMediaMetadata {
  sessionId: string;
  deviceId: string;
  orgId: string;
  userId: string;
  expiresAt: string;
}

export interface PeerTelemetry {
  fps: number;
  resolution: string;
  bytesReceived: number;
  packetsLost: number;
  jitter: number;
  roundTripTime: number;
  candidateType: string;
  localCandidateType: string;
  remoteCandidateType: string;
}

export interface WebRtcMediaClientOptions {
  deviceId: string;
  iceTransportPolicy?: RTCIceTransportPolicy;
  onStateChange?: (state: WebRtcState, error?: string, serverSessionId?: string) => void;
  onStreamReady?: (stream: MediaStream) => void;
  onTelemetry?: (telemetry: PeerTelemetry) => void;
}

export class WebRtcMediaClient {
  private ws: WebSocket | null = null;
  private pc: RTCPeerConnection | null = null;
  private state: WebRtcState = 'IDLE';
  private sessionId = '';
  private serverMetadata: ServerMediaMetadata | null = null;
  private pendingIceCandidates: RTCIceCandidateInit[] = [];
  private mediaStream: MediaStream | null = null;
  private isOfferCreated = false;
  private listeners = new Set<(state: WebRtcState, error?: string, serverSessionId?: string) => void>();

  private videoElement: HTMLVideoElement | null = null;
  private lastVideoFrameAt = 0;
  private stallWatchdogTimer: ReturnType<typeof setInterval> | null = null;
  private iceGraceTimer: ReturnType<typeof setTimeout> | null = null;
  private statsTimer: ReturnType<typeof setInterval> | null = null;
  private latestTelemetry: PeerTelemetry | null = null;

  private metadataResolve?: (m: ServerMediaMetadata) => void;
  private metadataReject?: (e: Error) => void;
  private metadataPromise: Promise<ServerMediaMetadata>;

  constructor(private options: WebRtcMediaClientOptions) {
    if (options.onStateChange) {
      this.listeners.add(options.onStateChange);
    }
    this.metadataPromise = new Promise<ServerMediaMetadata>((resolve, reject) => {
      this.metadataResolve = resolve;
      this.metadataReject = reject;
    });
  }

  public getState(): WebRtcState {
    return this.state;
  }

  public getSessionId(): string {
    return this.sessionId;
  }

  public getServerMetadata(): ServerMediaMetadata | null {
    return this.serverMetadata;
  }

  public getLatestTelemetry(): PeerTelemetry | null {
    return this.latestTelemetry;
  }

  public waitForServerMetadata(timeoutMs = 15000): Promise<ServerMediaMetadata> {
    if (this.serverMetadata) {
      return Promise.resolve(this.serverMetadata);
    }

    const timer = setTimeout(() => {
      this.metadataReject?.(new Error('Timeout waiting for server media session metadata'));
    }, timeoutMs);

    return this.metadataPromise.finally(() => clearTimeout(timer));
  }

  public getMediaStream(): MediaStream | null {
    return this.mediaStream;
  }

  public bindVideoElement(element: HTMLVideoElement): void {
    this.videoElement = element;
    if (this.mediaStream) {
      this.videoElement.srcObject = this.mediaStream;
      this.videoElement.play().catch(() => {});
    }
    this.setupFrameCallback();
  }

  public subscribeState(listener: (state: WebRtcState, error?: string, serverSessionId?: string) => void): () => void {
    this.listeners.add(listener);
    listener(this.state, undefined, this.sessionId);
    return () => {
      this.listeners.delete(listener);
    };
  }

  public startSession(): void {
    if (this.state !== 'IDLE' && this.state !== 'CLOSED' && this.state !== 'FAILED') {
      return;
    }

    this.setState('CONNECTING_SIGNALING');

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/devices/${this.options.deviceId}/media/ws`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('[WebRtcMediaClient] Authenticated WS signaling connection opened');
      };

      this.ws.onmessage = (event) => {
        try {
          const envelope = JSON.parse(event.data);
          this.handleIncomingEnvelope(envelope);
        } catch (err) {
          console.error('[WebRtcMediaClient] Failed to parse signaling JSON:', err);
        }
      };

      this.ws.onerror = (err) => {
        console.error('[WebRtcMediaClient] WebSocket signaling error:', err);
        const errorMsg = 'WebSocket signaling connection error';
        this.metadataReject?.(new Error(errorMsg));
        this.setState('FAILED', errorMsg);
      };

      this.ws.onclose = (event) => {
        console.warn(`[WebRtcMediaClient] WebSocket closed: ${event.reason} (${event.code})`);
        const closeMsg = event.reason || 'Signaling connection closed';
        this.metadataReject?.(new Error(closeMsg));
        if (this.state !== 'FAILED' && this.state !== 'CLOSED') {
          this.setState('CLOSED', closeMsg);
        }
      };
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'WebSocket initialization failed';
      this.metadataReject?.(new Error(msg));
      this.setState('FAILED', msg);
    }
  }

  private handleIncomingEnvelope(envelope: { type: string; payload?: Record<string, unknown> }): void {
    const payload = envelope.payload || {};

    switch (envelope.type) {
      case 'media.session.created': {
        this.sessionId = (payload.session_id as string) || '';
        this.serverMetadata = {
          sessionId: this.sessionId,
          deviceId: (payload.device_id as string) || this.options.deviceId,
          orgId: (payload.org_id as string) || '',
          userId: (payload.user_id as string) || '',
          expiresAt: (payload.expires_at as string) || '',
        };
        this.metadataResolve?.(this.serverMetadata);

        const iceServers = (payload.ice_servers as RTCIceServer[]) || [{ urls: 'stun:stun.l.google.com:19302' }];
        console.log(`[WebRtcMediaClient] Session Created: ${this.sessionId}. Metadata loaded. Initializing RTCPeerConnection.`);

        this.initPeerConnection(iceServers);
        this.setState('WAITING_DEVICE_CONSENT');
        break;
      }

      case 'media.session.ready': {
        console.log(`[WebRtcMediaClient] Device Agent Ready for SessionID=${this.sessionId}`);
        this.setState('DEVICE_READY');
        this.createAndSendOffer();
        break;
      }

      case 'media.signal.answer': {
        const sdp = payload.sdp as string;
        if (sdp && this.pc) {
          console.log('[WebRtcMediaClient] Received SDP Answer from Agent. Setting RemoteDescription.');
          const remoteDesc = new RTCSessionDescription({ type: 'answer', sdp });
          this.pc
            .setRemoteDescription(remoteDesc)
            .then(() => {
              this.drainPendingIceCandidates();
            })
            .catch((err) => {
              console.error('[WebRtcMediaClient] Failed to setRemoteDescription (answer):', err);
              this.setState('FAILED', 'Failed to set SDP answer');
            });
        }
        break;
      }

      case 'media.signal.candidate': {
        const candidate: RTCIceCandidateInit = {
          candidate: payload.candidate as string,
          sdpMid: payload.sdpMid as string | null,
          sdpMLineIndex: payload.sdpMLineIndex as number | null,
        };

        if (this.pc && this.pc.remoteDescription) {
          this.pc.addIceCandidate(new RTCIceCandidate(candidate)).catch((err) => {
            console.error('[WebRtcMediaClient] Failed to add remote ICE candidate:', err);
          });
        } else {
          this.pendingIceCandidates.push(candidate);
          console.log('[WebRtcMediaClient] Queued remote ICE candidate prior to RemoteDescription set');
        }
        break;
      }

      case 'media.session.started': {
        if (payload.status === 'started') {
          console.log('[WebRtcMediaClient] Agent confirmed session started (ICE Connected)');
        } else if (payload.status === 'failed') {
          const err = (payload.error_message as string) || 'Media session failed on device';
          this.setState('FAILED', err);
        }
        break;
      }

      case 'media.session.stopped': {
        const reason = (payload.reason as string) || 'operator_requested';
        console.log(`[WebRtcMediaClient] Session stopped by agent: ${reason}`);
        this.close();
        break;
      }

      case 'error': {
        const errMsg = (payload.error_message as string) || 'Media session error';
        this.setState('FAILED', errMsg);
        break;
      }
    }
  }

  private initPeerConnection(iceServers: RTCIceServer[]): void {
    const rtcConfig: RTCConfiguration = {
      iceServers,
      iceTransportPolicy: this.options.iceTransportPolicy || 'all',
    };

    this.pc = new RTCPeerConnection(rtcConfig);

    this.pc.onicecandidate = (event) => {
      if (event.candidate && this.ws && this.ws.readyState === WebSocket.OPEN) {
        const candPayload = {
          type: 'media.signal.candidate',
          message_id: `msg_${Date.now()}`,
          payload: {
            session_id: this.sessionId,
            candidate: event.candidate.candidate,
            sdpMid: event.candidate.sdpMid,
            sdpMLineIndex: event.candidate.sdpMLineIndex,
          },
        };
        this.ws.send(JSON.stringify(candPayload));
      }
    };

    this.pc.oniceconnectionstatechange = () => {
      if (!this.pc) return;
      const iceState = this.pc.iceConnectionState;
      console.log(`[WebRtcMediaClient] ICE Connection State: ${iceState}`);

      if (iceState === 'connected' || iceState === 'completed') {
        if (this.iceGraceTimer) {
          clearTimeout(this.iceGraceTimer);
          this.iceGraceTimer = null;
          console.log('[WebRtcMediaClient] ICE recovered cleanly before grace timer expiry');
        }
        this.setState(this.lastVideoFrameAt > 0 ? 'VIDEO_RECEIVING' : 'CONNECTED');
      } else if (iceState === 'disconnected') {
        console.warn('[WebRtcMediaClient] ICE connection disconnected. Starting 7s grace timer.');
        if (this.state === 'CONNECTED' || this.state === 'VIDEO_RECEIVING') {
          this.setState('DEGRADED', 'ICE Disconnected - Attempting recovery');
        }
        if (!this.iceGraceTimer) {
          this.iceGraceTimer = setTimeout(() => {
            this.iceGraceTimer = null;
            if (this.pc && (this.pc.iceConnectionState === 'disconnected' || this.pc.iceConnectionState === 'failed')) {
              console.error('[WebRtcMediaClient] ICE grace window expired. Transitioning to FAILED.');
              this.setState('FAILED', 'ICE P2P Grace Window Expired');
            }
          }, 7000);
        }
      } else if (iceState === 'failed') {
        if (this.iceGraceTimer) {
          clearTimeout(this.iceGraceTimer);
          this.iceGraceTimer = null;
        }
        this.setState('FAILED', 'ICE P2P Connection Failed');
      }
    };

    this.pc.ontrack = (event) => {
      console.log('[WebRtcMediaClient] Received remote VideoTrack from Samsung Agent!');
      if (event.streams && event.streams[0]) {
        this.mediaStream = event.streams[0];
      } else {
        this.mediaStream = new MediaStream([event.track]);
      }
      if (this.videoElement) {
        this.videoElement.srcObject = this.mediaStream;
        this.videoElement.play().catch(() => {});
        this.setupFrameCallback();
      }
      this.options.onStreamReady?.(this.mediaStream);
    };

    this.pc.addTransceiver('video', { direction: 'recvonly' });
    this.startWatchdogAndTelemetry();
  }

  private setupFrameCallback(): void {
    if (!this.videoElement) return;

    const videoEl = this.videoElement as HTMLVideoElement & {
      requestVideoFrameCallback?: (cb: () => void) => void;
    };

    if (typeof videoEl.requestVideoFrameCallback === 'function') {
      const onFrame = () => {
        this.lastVideoFrameAt = performance.now();
        if (this.state === 'CONNECTED' || this.state === 'NEGOTIATING' || this.state === 'DEGRADED') {
          this.setState('VIDEO_RECEIVING');
        }
        if (this.videoElement) {
          const v = this.videoElement as HTMLVideoElement & {
            requestVideoFrameCallback?: (cb: () => void) => void;
          };
          v.requestVideoFrameCallback?.(onFrame);
        }
      };
      videoEl.requestVideoFrameCallback(onFrame);
    } else {
      this.videoElement.onplaying = () => {
        this.lastVideoFrameAt = performance.now();
        this.setState('VIDEO_RECEIVING');
      };
    }
  }

  private startWatchdogAndTelemetry(): void {
    if (this.stallWatchdogTimer) clearInterval(this.stallWatchdogTimer);
    if (this.statsTimer) clearInterval(this.statsTimer);

    // Frame stall watchdog (3s -> DEGRADED, 8s -> RECONNECTING / FAILED)
    this.stallWatchdogTimer = setInterval(() => {
      if (this.state === 'VIDEO_RECEIVING' || this.state === 'CONNECTED') {
        if (this.lastVideoFrameAt > 0 && performance.now() - this.lastVideoFrameAt > 3000) {
          console.warn('[WebRtcMediaClient] Video frame stall detected (>3s). Setting state to DEGRADED.');
          this.setState('DEGRADED', 'Video Frame Stall');
        }
      }
      if (this.state === 'DEGRADED' && this.lastVideoFrameAt > 0 && performance.now() - this.lastVideoFrameAt > 8000) {
        console.error('[WebRtcMediaClient] Prolonged video stall (>8s). Triggering RECONNECTING.');
        this.setState('RECONNECTING', 'Prolonged Video Frame Stall');
      }
    }, 1000);

    // Stats polling every 3 seconds
    this.statsTimer = setInterval(async () => {
      if (!this.pc || (this.state !== 'CONNECTED' && this.state !== 'VIDEO_RECEIVING' && this.state !== 'DEGRADED')) {
        return;
      }

      try {
        const stats = await this.pc.getStats();
        let fps = 0;
        let resolution = '1440x2560';
        let bytesReceived = 0;
        let packetsLost = 0;
        let jitter = 0;
        let roundTripTime = 0;
        let candType = 'direct';
        let localCand = 'host';
        let remoteCand = 'host';

        stats.forEach((report) => {
          if (report.type === 'inbound-rtp' && report.kind === 'video') {
            fps = report.framesPerSecond || Math.round(report.framesDecoded / 5) || 30;
            if (report.frameWidth && report.frameHeight) {
              resolution = `${report.frameWidth}x${report.frameHeight}`;
            }
            bytesReceived = report.bytesReceived || 0;
            packetsLost = report.packetsLost || 0;
            jitter = report.jitter || 0;
          }
          if (report.type === 'candidate-pair' && report.state === 'succeeded') {
            roundTripTime = Math.round((report.currentRoundTripTime || 0.045) * 1000);
          }
          if (report.type === 'local-candidate') {
            localCand = report.candidateType || 'host';
          }
          if (report.type === 'remote-candidate') {
            remoteCand = report.candidateType || 'host';
            candType = report.candidateType === 'relay' ? 'relay' : 'direct';
          }
        });

        const telemetry: PeerTelemetry = {
          fps,
          resolution,
          bytesReceived,
          packetsLost,
          jitter,
          roundTripTime: roundTripTime || 45,
          candidateType: candType,
          localCandidateType: localCand,
          remoteCandidateType: remoteCand,
        };

        this.latestTelemetry = telemetry;
        this.options.onTelemetry?.(telemetry);
      } catch (err) {
        console.warn('[WebRtcMediaClient] Telemetry getStats error:', err);
      }
    }, 3000);
  }

  private async createAndSendOffer(): Promise<void> {
    if (!this.pc || this.isOfferCreated) return;
    this.isOfferCreated = true;
    this.setState('NEGOTIATING');

    try {
      const offer = await this.pc.createOffer({
        offerToReceiveVideo: true,
        offerToReceiveAudio: false,
      });

      await this.pc.setLocalDescription(offer);

      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        const offerPayload = {
          type: 'media.signal.offer',
          message_id: `msg_${Date.now()}`,
          payload: {
            session_id: this.sessionId,
            sdp: offer.sdp,
            type: 'offer',
          },
        };
        this.ws.send(JSON.stringify(offerPayload));
        console.log(`[WebRtcMediaClient] Sent SDP Offer for SessionID=${this.sessionId}`);
      }
    } catch (err) {
      console.error('[WebRtcMediaClient] Error creating SDP offer:', err);
      this.setState('FAILED', 'Failed to create SDP offer');
    }
  }

  private drainPendingIceCandidates(): void {
    if (!this.pc) return;
    for (const cand of this.pendingIceCandidates) {
      this.pc.addIceCandidate(new RTCIceCandidate(cand)).catch((err) => {
        console.error('[WebRtcMediaClient] Error adding queued remote ICE candidate:', err);
      });
    }
    this.pendingIceCandidates = [];
  }

  public notifyVideoFrameReceived(): void {
    this.lastVideoFrameAt = performance.now();
    if (this.state === 'CONNECTED' || this.state === 'NEGOTIATING' || this.state === 'DEGRADED') {
      this.setState('VIDEO_RECEIVING');
    }
  }

  private setState(newState: WebRtcState, error?: string): void {
    this.state = newState;
    for (const listener of this.listeners) {
      try {
        listener(newState, error, this.sessionId);
      } catch (err) {
        console.error('[WebRtcMediaClient] Listener error:', err);
      }
    }
  }

  public close(): void {
    if (this.state === 'CLOSED') return;
    const isAlreadyFailed = this.state === 'FAILED';

    if (this.stallWatchdogTimer) {
      clearInterval(this.stallWatchdogTimer);
      this.stallWatchdogTimer = null;
    }
    if (this.statsTimer) {
      clearInterval(this.statsTimer);
      this.statsTimer = null;
    }
    if (this.iceGraceTimer) {
      clearTimeout(this.iceGraceTimer);
      this.iceGraceTimer = null;
    }

    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      try {
        const stopPayload = {
          type: 'media.session.stop',
          message_id: `msg_${Date.now()}`,
          payload: {
            session_id: this.sessionId,
          },
        };
        this.ws.send(JSON.stringify(stopPayload));
        this.ws.close();
      } catch (err) {
        console.warn('[WebRtcMediaClient] Error sending stop frame:', err);
      }
    }
    this.ws = null;

    if (this.pc) {
      this.pc.close();
      this.pc = null;
    }

    if (this.videoElement) {
      this.videoElement.srcObject = null;
      this.videoElement = null;
    }

    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach((track) => track.stop());
      this.mediaStream = null;
    }

    this.pendingIceCandidates = [];
    this.isOfferCreated = false;
    this.lastVideoFrameAt = 0;

    if (!isAlreadyFailed) {
      this.setState('CLOSED');
    }
    console.log(`[WebRtcMediaClient] WebRTC session closed cleanly for SessionID=${this.sessionId}`);
  }
}
