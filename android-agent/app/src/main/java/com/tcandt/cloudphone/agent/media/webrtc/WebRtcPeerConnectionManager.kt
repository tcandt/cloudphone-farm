package com.tcandt.cloudphone.agent.media.webrtc

import android.content.Context
import android.content.Intent
import android.util.Log
import org.json.JSONObject
import org.webrtc.DefaultVideoDecoderFactory
import org.webrtc.DefaultVideoEncoderFactory
import org.webrtc.EglBase
import org.webrtc.IceCandidate
import org.webrtc.MediaConstraints
import org.webrtc.PeerConnection
import org.webrtc.PeerConnectionFactory
import org.webrtc.RtpReceiver
import org.webrtc.ScreenCapturerAndroid
import org.webrtc.SdpObserver
import org.webrtc.SessionDescription
import org.webrtc.SurfaceTextureHelper
import org.webrtc.VideoSource
import org.webrtc.VideoTrack

class WebRtcPeerConnectionManager(
    private val context: Context,
    private val signalPublisher: (type: String, payload: JSONObject) -> Unit
) {
    private val rootEglBase: EglBase = EglBase.create()
    private var peerConnectionFactory: PeerConnectionFactory? = null
    private var peerConnection: PeerConnection? = null

    private var videoCapturer: ScreenCapturerAndroid? = null
    private var videoSource: VideoSource? = null
    private var videoTrack: VideoTrack? = null
    private var surfaceTextureHelper: SurfaceTextureHelper? = null

    private var activeSessionId: String = ""
    private var isRemoteDescriptionSet: Boolean = false
    private val pendingIceCandidates = mutableListOf<IceCandidate>()

    companion object {
        private const val TAG = "WebRtcPeerConnection"
    }

    init {
        initFactory()
    }

    private fun initFactory() {
        val options = PeerConnectionFactory.InitializationOptions.builder(context)
            .setEnableInternalTracer(true)
            .createInitializationOptions()
        PeerConnectionFactory.initialize(options)

        val encoderFactory = DefaultVideoEncoderFactory(rootEglBase.eglBaseContext, true, true)
        val decoderFactory = DefaultVideoDecoderFactory(rootEglBase.eglBaseContext)

        peerConnectionFactory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(encoderFactory)
            .setVideoDecoderFactory(decoderFactory)
            .createPeerConnectionFactory()

        Log.i(TAG, "Initialized Native WebRTC PeerConnectionFactory cleanly")
    }

    fun startSession(sessionId: String, projectionResultData: Intent) {
        activeSessionId = sessionId
        isRemoteDescriptionSet = false
        pendingIceCandidates.clear()

        val iceServers = listOf(
            PeerConnection.IceServer.builder("stun:stun.l.google.com:19302").createIceServer(),
            PeerConnection.IceServer.builder("stun:stun1.l.google.com:19302").createIceServer(),
            PeerConnection.IceServer.builder("turn:stun.l.google.com:19302")
                .setUsername("pcp_guest")
                .setPassword("pcp_pass_guest")
                .createIceServer()
        )

        val rtcConfig = PeerConnection.RTCConfiguration(iceServers).apply {
            sdpSemantics = PeerConnection.SdpSemantics.UNIFIED_PLAN
        }

        peerConnection = peerConnectionFactory?.createPeerConnection(rtcConfig, object : PeerConnection.Observer {
            override fun onSignalingChange(state: PeerConnection.SignalingState?) {
                Log.d(TAG, "WebRTC SignalingState changed: $state")
            }

            override fun onIceConnectionChange(state: PeerConnection.IceConnectionState?) {
                Log.i(TAG, "WebRTC IceConnectionState changed: $state (SessionID=$activeSessionId)")
                if (state == PeerConnection.IceConnectionState.CONNECTED) {
                    val payload = JSONObject().apply {
                        put("session_id", activeSessionId)
                        put("status", "started")
                    }
                    signalPublisher("media.session.started", payload)
                } else if (state == PeerConnection.IceConnectionState.FAILED || state == PeerConnection.IceConnectionState.DISCONNECTED) {
                    val payload = JSONObject().apply {
                        put("session_id", activeSessionId)
                        put("status", "failed")
                        put("error_message", "ICE Connection $state")
                    }
                    signalPublisher("media.session.started", payload)
                }
            }

            override fun onIceConnectionReceivingChange(receiving: Boolean) {}

            override fun onIceGatheringChange(state: PeerConnection.IceGatheringState?) {
                Log.d(TAG, "WebRTC IceGatheringState changed: $state")
            }

            override fun onIceCandidate(candidate: IceCandidate?) {
                if (candidate != null) {
                    val candPayload = JSONObject().apply {
                        put("session_id", activeSessionId)
                        put("sdpMid", candidate.sdpMid)
                        put("sdpMLineIndex", candidate.sdpMLineIndex)
                        put("candidate", candidate.sdp)
                    }
                    signalPublisher("media.signal.candidate", candPayload)
                }
            }

            override fun onIceCandidatesRemoved(candidates: Array<out IceCandidate>?) {}
            override fun onAddStream(stream: org.webrtc.MediaStream?) {}
            override fun onRemoveStream(stream: org.webrtc.MediaStream?) {}
            override fun onDataChannel(dataChannel: org.webrtc.DataChannel?) {}
            override fun onRenegotiationNeeded() {}
            override fun onAddTrack(receiver: RtpReceiver?, mediaStreams: Array<out org.webrtc.MediaStream>?) {}
        })

        // Attach WebRTC ScreenCapturerAndroid to VideoTrack
        attachScreenCapturer(projectionResultData)

        // Dispatch media.session.ready to backend/web
        val readyPayload = JSONObject().apply {
            put("session_id", activeSessionId)
            put("status", "ready")
        }
        signalPublisher("media.session.ready", readyPayload)
        Log.i(TAG, "WebRTC PeerConnection and ScreenCapturer initialized for SessionID=$activeSessionId. Sent media.session.ready")
    }

    private fun attachScreenCapturer(projectionResultData: Intent) {
        try {
            videoCapturer = ScreenCapturerAndroid(projectionResultData, object : MediaProjectionCallback() {})
            surfaceTextureHelper = SurfaceTextureHelper.create("PCP_WebRTC_Thread", rootEglBase.eglBaseContext)
            videoSource = peerConnectionFactory?.createVideoSource(videoCapturer!!.isScreencast)

            videoCapturer?.initialize(surfaceTextureHelper, context, videoSource?.capturerObserver)
            videoCapturer?.startCapture(720, 1280, 30)

            videoTrack = peerConnectionFactory?.createVideoTrack("video_track_0", videoSource)
            videoTrack?.setEnabled(true)

            peerConnection?.addTrack(videoTrack, listOf("pcp_media_stream_0"))
            Log.i(TAG, "Attached MediaProjection VideoTrack to WebRTC PeerConnection successfully")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to attach WebRTC ScreenCapturer: ${e.message}", e)
        }
    }

    fun handleRemoteOffer(sessionId: String, offerSdpText: String) {
        if (sessionId.isNotEmpty() && activeSessionId.isNotEmpty() && sessionId != activeSessionId) {
            Log.w(TAG, "SessionID mismatch in handleRemoteOffer ($sessionId vs $activeSessionId)")
            return
        }

        if (peerConnection == null) {
            Log.e(TAG, "Cannot handleRemoteOffer: PeerConnection is null")
            return
        }

        val remoteSdp = SessionDescription(SessionDescription.Type.OFFER, offerSdpText)
        peerConnection?.setRemoteDescription(object : SimpleSdpObserver() {
            override fun onSetSuccess() {
                isRemoteDescriptionSet = true
                Log.i(TAG, "Set WebRTC Remote Description (OFFER) successfully")

                // Drain queued pending ICE candidates
                drainPendingIceCandidates()

                createAnswer()
            }
        }, remoteSdp)
    }

    private fun createAnswer() {
        val mediaConstraints = MediaConstraints().apply {
            mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveVideo", "true"))
        }

        peerConnection?.createAnswer(object : SimpleSdpObserver() {
            override fun onCreateSuccess(desc: SessionDescription?) {
                if (desc != null) {
                    peerConnection?.setLocalDescription(SimpleSdpObserver(), desc)

                    val answerPayload = JSONObject().apply {
                        put("session_id", activeSessionId)
                        put("sdp", desc.description)
                        put("type", "answer")
                    }
                    signalPublisher("media.signal.answer", answerPayload)
                    Log.i(TAG, "Created and sent WebRTC Local Description (ANSWER) for SessionID=$activeSessionId")
                }
            }
        }, mediaConstraints)
    }

    fun handleRemoteCandidate(sessionId: String, sdpMid: String, sdpMLineIndex: Int, candidateSdp: String) {
        if (sessionId.isNotEmpty() && activeSessionId.isNotEmpty() && sessionId != activeSessionId) {
            return
        }

        val candidate = IceCandidate(sdpMid, sdpMLineIndex, candidateSdp)
        if (isRemoteDescriptionSet && peerConnection != null) {
            peerConnection?.addIceCandidate(candidate)
            Log.d(TAG, "Added remote ICE candidate: $sdpMid [$sdpMLineIndex]")
        } else {
            pendingIceCandidates.add(candidate)
            Log.d(TAG, "Queued remote ICE candidate prior to RemoteDescription set")
        }
    }

    private fun drainPendingIceCandidates() {
        for (candidate in pendingIceCandidates) {
            peerConnection?.addIceCandidate(candidate)
        }
        pendingIceCandidates.clear()
    }

    fun closeSession() {
        try {
            videoCapturer?.stopCapture()
            videoCapturer?.dispose()
            videoCapturer = null

            videoSource?.dispose()
            videoSource = null

            surfaceTextureHelper?.dispose()
            surfaceTextureHelper = null

            peerConnection?.close()
            peerConnection = null

            isRemoteDescriptionSet = false
            pendingIceCandidates.clear()

            Log.i(TAG, "Closed WebRTC PeerConnection session cleanly for SessionID=$activeSessionId")
            activeSessionId = ""
        } catch (e: Exception) {
            Log.e(TAG, "Error closing WebRTC session: ${e.message}")
        }
    }

    open class MediaProjectionCallback : android.media.projection.MediaProjection.Callback() {
        override fun onStop() {
            Log.i("MediaProjectionCallback", "MediaProjection stopped by system")
        }
    }

    open class SimpleSdpObserver : SdpObserver {
        override fun onCreateSuccess(desc: SessionDescription?) {}
        override fun onSetSuccess() {}
        override fun onCreateFailure(reason: String?) {
            Log.e(TAG, "SDP Observer onCreateFailure: $reason")
        }
        override fun onSetFailure(reason: String?) {
            Log.e(TAG, "SDP Observer onSetFailure: $reason")
        }
    }
}
