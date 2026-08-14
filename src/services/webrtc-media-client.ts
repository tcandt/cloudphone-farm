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
  fps?: number;
  resolution?: string;
  bytesReceived?: number;
  packetsLost?: number;
  jitter?: number;
  roundTripTime?: number;
  candidateType?: 'direct' | 'relay' | 'unknown';
  localCandidateType?: string;
  remoteCandidateType?: string;
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
  private lastFramesDecoded = 0;
  private stallWatchdogTimer: ReturnType<typeof setInterval> | null = null;
  private iceGraceTimer: ReturnType<typeof setTimeout> | null = null;
  private statsTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private latestTelemetry: PeerTelemetry | null = null;

  private isReconnecting = false;
  private reconnectAttempts = 0;
  private isClosedExplicitly = false;
  private frameCallbackId: number | null = null;

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
    if (this.videoElement === element) return;
    this.unbindVideoElement();
    this.videoElement = element;
    if (this.mediaStream) {
      this.videoElement.srcObject = this.mediaStream;
      const p = this.videoElement.play();
      if (p && typeof p.catch === 'function') {
        p.catch(() => {});
      }
    }
    this.setupFrameCallback();
  }

  public unbindVideoElement(): void {
    this.cancelFrameAuthorityLoop();
    if (this.videoElement) {
      this.videoElement.srcObject = null;
      this.videoElement = null;
    }
  }

  public subscribeState(listener: (state: WebRtcState, error?: string, serverSessionId?: string) => void): () => void {
    this.listeners.add(listener);
    listener(this.state, undefined, this.sessionId);
    return () => {
      this.listeners.delete(listener);
    };
  }

  public startSession(): void {
    if (this.isClosedExplicitly) {
      this.isClosedExplicitly = false;
    }

    if (this.ws || (this.state !== 'IDLE' && this.state !== 'RECONNECTING' && this.state !== 'FAILED')) {
      console.warn(`[WebRtcMediaClient] Session already active or connecting for device ${this.options.deviceId}`);
      return;
    }

    this.setState('CONNECTING_SIGNALING');

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/devices/${this.options.deviceId}/media/ws`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log(`[WebRtcMediaClient] Authenticated WS signaling connection opened for ${this.options.deviceId}`);
      };

      this.ws.onmessage = (event) => {
        try {
          const envelope = JSON.parse(event.data);
          this.handleIncomingEnvelope(envelope);
        } catch (err) {
          console.error('[WebRtcMediaClient] Failed to parse signaling envelope:', err);
        }
      };

      this.ws.onerror = (err) => {
        console.error('[WebRtcMediaClient] WebSocket signaling error:', err);
        if (!this.isClosedExplicitly) {
          this.performReconnect('websocket_signaling_error');
        }
      };

      this.ws.onclose = (event) => {
        console.log(`[WebRtcMediaClient] WebSocket signaling connection closed (${event.code}: ${event.reason})`);
        if (!this.isClosedExplicitly && this.state !== 'CLOSED') {
          this.performReconnect('websocket_signaling_closed');
        }
      };
    } catch (err) {
      console.error('[WebRtcMediaClient] Failed to instantiate WebSocket:', err);
      this.performReconnect('websocket_instantiation_failed');
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
        console.log(`[WebRtcMediaClient] Session Created: ${this.sessionId}. Initializing RTCPeerConnection.`);

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
              this.performReconnect('sdp_answer_set_failed');
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
          this.performReconnect(err);
        }
        break;
      }

      case 'media.session.stopped': {
        const reason = (payload.reason as string) || 'operator_requested';
        console.log(`[WebRtcMediaClient] Session stopped by agent: ${reason}`);
        if (!this.isClosedExplicitly) {
          this.performReconnect(`agent_stopped:${reason}`);
        }
        break;
      }

      case 'error': {
        const errMsg = (payload.error_message as string) || 'Media session error';
        this.performReconnect(errMsg);
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
              console.error('[WebRtcMediaClient] ICE grace window expired (7s). Triggering controlled reconnect.');
              this.performReconnect('ice_grace_window_expired');
            }
          }, 7000);
        }
      } else if (iceState === 'failed') {
        if (this.iceGraceTimer) {
          clearTimeout(this.iceGraceTimer);
          this.iceGraceTimer = null;
        }
        this.performReconnect('ice_p2p_failed');
      }
    };

    this.pc.ontrack = (event) => {
      console.log('[WebRtcMediaClient] Received remote VideoTrack!');
      if (event.streams && event.streams[0]) {
        this.mediaStream = event.streams[0];
      } else {
        this.mediaStream = new MediaStream([event.track]);
      }
      if (this.videoElement) {
        this.videoElement.srcObject = this.mediaStream;
        const p = this.videoElement.play();
        if (p && typeof p.catch === 'function') {
          p.catch(() => {});
        }
        this.setupFrameCallback();
      }
      this.options.onStreamReady?.(this.mediaStream);
    };

    this.pc.addTransceiver('video', { direction: 'recvonly' });
    this.startWatchdogAndTelemetry();
  }

  private cancelFrameAuthorityLoop(): void {
    if (this.frameCallbackId !== null && this.videoElement) {
      const videoEl = this.videoElement as HTMLVideoElement & {
        cancelVideoFrameCallback?: (id: number) => void;
      };
      if (typeof videoEl.cancelVideoFrameCallback === 'function') {
        videoEl.cancelVideoFrameCallback(this.frameCallbackId);
      }
    }
    this.frameCallbackId = null;
  }

  private setupFrameCallback(): void {
    this.cancelFrameAuthorityLoop();
    if (!this.videoElement) return;

    const videoEl = this.videoElement as HTMLVideoElement & {
      requestVideoFrameCallback?: (cb: () => void) => number;
    };

    if (typeof videoEl.requestVideoFrameCallback === 'function') {
      const onFrame = () => {
        if (this.isClosedExplicitly) return;
        this.notifyVideoFrameReceived();
        if (this.videoElement) {
          const v = this.videoElement as HTMLVideoElement & {
            requestVideoFrameCallback?: (cb: () => void) => number;
          };
          if (typeof v.requestVideoFrameCallback === 'function') {
            this.frameCallbackId = v.requestVideoFrameCallback(onFrame);
          }
        }
      };
      this.frameCallbackId = videoEl.requestVideoFrameCallback(onFrame);
    }
  }

  private startWatchdogAndTelemetry(): void {
    if (this.stallWatchdogTimer) clearInterval(this.stallWatchdogTimer);
    if (this.statsTimer) clearInterval(this.statsTimer);

    // Frame stall watchdog (3s -> DEGRADED, 8s -> RECONNECTING)
    this.stallWatchdogTimer = setInterval(() => {
      if (this.state === 'VIDEO_RECEIVING' || this.state === 'CONNECTED') {
        if (this.lastVideoFrameAt > 0 && performance.now() - this.lastVideoFrameAt > 3000) {
          console.warn('[WebRtcMediaClient] Video frame stall detected (>3s). Setting state to DEGRADED.');
          this.setState('DEGRADED', 'Video Frame Stall');
        }
      }
      if (this.state === 'DEGRADED' && this.lastVideoFrameAt > 0 && performance.now() - this.lastVideoFrameAt > 8000) {
        console.error('[WebRtcMediaClient] Prolonged video stall (>8s). Triggering controlled reconnect.');
        this.performReconnect('prolonged_video_stall');
      }
    }, 1000);

    // Stats polling every 3 seconds (Authoritative PeerTelemetry)
    this.statsTimer = setInterval(async () => {
      if (!this.pc || (this.state !== 'CONNECTED' && this.state !== 'VIDEO_RECEIVING' && this.state !== 'DEGRADED')) {
        return;
      }

      try {
        const stats = await this.pc.getStats();
        let fps: number | undefined;
        let resolution: string | undefined;
        let bytesReceived: number | undefined;
        let packetsLost: number | undefined;
        let jitter: number | undefined;
        let roundTripTime: number | undefined;
        let candType: 'direct' | 'relay' | 'unknown' = 'unknown';
        let localCandType: string | undefined;
        let remoteCandType: string | undefined;

        const candidateMap = new Map<string, Record<string, unknown>>();
        let selectedPairId = '';
        let selectedPairReport: Record<string, unknown> | null = null;

        stats.forEach((report) => {
          if (report.type === 'transport' && typeof report.selectedCandidatePairId === 'string') {
            selectedPairId = report.selectedCandidatePairId;
          }
          if (report.type === 'local-candidate' || report.type === 'remote-candidate') {
            candidateMap.set(report.id as string, report as Record<string, unknown>);
          }
          if (report.type === 'inbound-rtp' && report.kind === 'video') {
            const r = report as Record<string, unknown>;
            if (typeof r.framesPerSecond === 'number' && r.framesPerSecond > 0) {
              fps = r.framesPerSecond;
            }
            if (typeof r.frameWidth === 'number' && typeof r.frameHeight === 'number' && r.frameWidth > 0 && r.frameHeight > 0) {
              resolution = `${r.frameWidth}x${r.frameHeight}`;
            }
            if (typeof r.bytesReceived === 'number') bytesReceived = r.bytesReceived;
            if (typeof r.packetsLost === 'number') packetsLost = r.packetsLost;
            if (typeof r.jitter === 'number') jitter = Math.round(r.jitter * 1000);

            // Non-rVFC Fallback: Check framesDecoded progression
            if (typeof r.framesDecoded === 'number') {
              if (r.framesDecoded > this.lastFramesDecoded) {
                this.lastFramesDecoded = r.framesDecoded;
                this.notifyVideoFrameReceived();
              }
            }
          }
        });

        stats.forEach((report) => {
          if (report.type === 'candidate-pair') {
            const r = report as Record<string, unknown>;
            if (selectedPairId && r.id === selectedPairId) {
              selectedPairReport = r;
            } else if (!selectedPairReport && (r.selected === true || (r.nominated === true && r.state === 'succeeded'))) {
              selectedPairReport = r;
            }
          }
        });

        if (selectedPairReport) {
          const pair = selectedPairReport as Record<string, unknown>;
          if (typeof pair.currentRoundTripTime === 'number') {
            roundTripTime = Math.round((pair.currentRoundTripTime as number) * 1000);
          }
          const remoteCand = candidateMap.get(pair.remoteCandidateId as string);
          const localCand = candidateMap.get(pair.localCandidateId as string);

          if (localCand && typeof localCand.candidateType === 'string') {
            localCandType = localCand.candidateType as string;
          }
          if (remoteCand && typeof remoteCand.candidateType === 'string') {
            remoteCandType = remoteCand.candidateType as string;
            candType = remoteCandType === 'relay' || localCandType === 'relay' ? 'relay' : 'direct';
          }
        }

        const telemetry: PeerTelemetry = {
          fps,
          resolution,
          bytesReceived,
          packetsLost,
          jitter,
          roundTripTime,
          candidateType: candType,
          localCandidateType: localCandType,
          remoteCandidateType: remoteCandType,
        };

        this.latestTelemetry = telemetry;
        this.options.onTelemetry?.(telemetry);
      } catch (err) {
        console.warn('[WebRtcMediaClient] Telemetry getStats error:', err);
      }
    }, 3000);
  }

  private performReconnect(reason: string): void {
    if (this.isReconnecting || this.isClosedExplicitly) return;
    this.isReconnecting = true;
    this.reconnectAttempts++;
    console.warn(`[WebRtcMediaClient] Initiating controlled reconnect #${this.reconnectAttempts}. Reason: ${reason}`);
    this.setState('RECONNECTING', `Reconnecting (${reason})`);

    // 1. Clear timers
    if (this.stallWatchdogTimer) clearInterval(this.stallWatchdogTimer);
    if (this.statsTimer) clearInterval(this.statsTimer);
    if (this.iceGraceTimer) clearTimeout(this.iceGraceTimer);
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.stallWatchdogTimer = null;
    this.statsTimer = null;
    this.iceGraceTimer = null;
    this.reconnectTimer = null;

    // 2. Stop old MediaStream tracks & clear video srcObject
    this.cancelFrameAuthorityLoop();
    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach((track) => track.stop());
      this.mediaStream = null;
    }
    if (this.videoElement) {
      this.videoElement.srcObject = null;
    }

    // 2. Best-effort stop frame to old signaling WS & close
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      try {
        if (this.sessionId) {
          this.ws.send(
            JSON.stringify({
              type: 'media.session.stop',
              message_id: `stop_${Date.now()}`,
              payload: { session_id: this.sessionId },
            })
          );
        }
        this.ws.close();
      } catch (err) {
        console.warn('[WebRtcMediaClient] Error stopping old session on reconnect:', err);
      }
    }
    this.ws = null;

    // 4. Close old PeerConnection
    if (this.pc) {
      this.pc.close();
      this.pc = null;
    }

    // 5. Reset state & create new metadata promise
    this.sessionId = '';
    this.serverMetadata = null;
    this.isOfferCreated = false;
    this.pendingIceCandidates = [];
    this.metadataPromise = new Promise<ServerMediaMetadata>((resolve, reject) => {
      this.metadataResolve = resolve;
      this.metadataReject = reject;
    });

    // 6. Cancellable Exponential backoff timer
    const backoffMs = Math.min(
      30000,
      Math.pow(2, Math.min(this.reconnectAttempts - 1, 5)) * 1000 + Math.floor(Math.random() * 500)
    );
    console.log(`[WebRtcMediaClient] Waiting ${backoffMs}ms before executing reconnect attempt #${this.reconnectAttempts}`);

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.isClosedExplicitly) {
        this.isReconnecting = false;
        this.startSession();
      }
    }, backoffMs);
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
      this.performReconnect('sdp_offer_failed');
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
    if (this.state !== 'CLOSED' && this.state !== 'FAILED' && this.state !== 'IDLE') {
      this.setState('VIDEO_RECEIVING');
      // Reset backoff ONLY after authoritative video frame recovery
      this.reconnectAttempts = 0;
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
    this.isClosedExplicitly = true;
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
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
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

    this.unbindVideoElement();

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
