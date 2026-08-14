export type WebRtcState =
  | 'IDLE'
  | 'CONNECTING_SIGNALING'
  | 'SESSION_CREATED'
  | 'WAITING_DEVICE_CONSENT'
  | 'DEVICE_READY'
  | 'NEGOTIATING'
  | 'CONNECTED'
  | 'VIDEO_RECEIVING'
  | 'CLOSED'
  | 'FAILED';

export interface WebRtcMediaClientOptions {
  deviceId: string;
  onStateChange?: (state: WebRtcState, error?: string) => void;
  onStreamReady?: (stream: MediaStream) => void;
}

export class WebRtcMediaClient {
  private ws: WebSocket | null = null;
  private pc: RTCPeerConnection | null = null;
  private state: WebRtcState = 'IDLE';
  private sessionId = '';
  private pendingIceCandidates: RTCIceCandidateInit[] = [];
  private mediaStream: MediaStream | null = null;
  private isOfferCreated = false;

  constructor(private options: WebRtcMediaClientOptions) {}

  public getState(): WebRtcState {
    return this.state;
  }

  public getSessionId(): string {
    return this.sessionId;
  }

  public getMediaStream(): MediaStream | null {
    return this.mediaStream;
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
        this.setState('FAILED', 'WebSocket signaling connection error');
      };

      this.ws.onclose = (event) => {
        console.warn(`[WebRtcMediaClient] WebSocket closed: ${event.reason} (${event.code})`);
        if (this.state !== 'CLOSED') {
          this.setState('CLOSED', event.reason || 'Signaling connection closed');
        }
      };
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'WebSocket initialization failed';
      this.setState('FAILED', msg);
    }
  }

  private handleIncomingEnvelope(envelope: { type: string; payload?: Record<string, unknown> }): void {
    const payload = envelope.payload || {};

    switch (envelope.type) {
      case 'media.session.created': {
        this.sessionId = (payload.session_id as string) || '';
        const iceServers = (payload.ice_servers as RTCIceServer[]) || [{ urls: 'stun:stun.l.google.com:19302' }];
        console.log(`[WebRtcMediaClient] Session Created: ${this.sessionId}. Initializing RTCPeerConnection with STUN/TURN servers`);

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
    this.pc = new RTCPeerConnection({
      iceServers,
      sdpSemantics: 'unified-plan',
    } as RTCConfiguration);

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
      console.log(`[WebRtcMediaClient] ICE Connection State: ${this.pc.iceConnectionState}`);
      if (this.pc.iceConnectionState === 'connected' || this.pc.iceConnectionState === 'completed') {
        this.setState('CONNECTED');
      } else if (this.pc.iceConnectionState === 'failed') {
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
      this.options.onStreamReady?.(this.mediaStream);
    };

    // Add video recvonly transceiver
    this.pc.addTransceiver('video', { direction: 'recvonly' });
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
    if (this.state === 'CONNECTED' || this.state === 'NEGOTIATING') {
      this.setState('VIDEO_RECEIVING');
    }
  }

  private setState(newState: WebRtcState, error?: string): void {
    this.state = newState;
    this.options.onStateChange?.(newState, error);
  }

  public close(): void {
    if (this.state === 'CLOSED') return;
    this.state = 'CLOSED';

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

    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach((track) => track.stop());
      this.mediaStream = null;
    }

    this.pendingIceCandidates = [];
    this.isOfferCreated = false;
    this.setState('CLOSED');
    console.log(`[WebRtcMediaClient] WebRTC session closed cleanly for SessionID=${this.sessionId}`);
  }
}
