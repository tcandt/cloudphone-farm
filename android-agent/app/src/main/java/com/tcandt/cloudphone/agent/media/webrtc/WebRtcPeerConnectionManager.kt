package com.tcandt.cloudphone.agent.media.webrtc

import android.content.Context
import android.content.Intent
import android.hardware.display.DisplayManager
import android.util.Log
import com.tcandt.cloudphone.agent.control.DisplayGeometryProvider
import com.tcandt.cloudphone.agent.control.DisplayOrientation
import com.tcandt.cloudphone.agent.media.ScreenCaptureManager
import org.json.JSONArray
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

private fun logD(tag: String, msg: String) { try { Log.d(tag, msg) } catch (_: Throwable) {} }
private fun logI(tag: String, msg: String) { try { Log.i(tag, msg) } catch (_: Throwable) {} }
private fun logW(tag: String, msg: String) { try { Log.w(tag, msg) } catch (_: Throwable) {} }
private fun logE(tag: String, msg: String, t: Throwable? = null) { try { if (t != null) Log.e(tag, msg, t) else Log.e(tag, msg) } catch (_: Throwable) {} }

class WebRtcPeerConnectionManager(
    private val context: Context,
    private val signalPublisher: (type: String, payload: JSONObject) -> Unit
) {
    private val rootEglBase: EglBase? by lazy {
        try {
            EglBase.create()
        } catch (e: Throwable) {
            logW(TAG, "EglBase.create failed in JVM test mode: ${e.message}")
            null
        }
    }
    private var peerConnectionFactory: PeerConnectionFactory? = null
    private var peerConnection: PeerConnection? = null

    private var videoCapturer: ScreenCapturerAndroid? = null
    private var videoSource: VideoSource? = null
    private var videoTrack: VideoTrack? = null
    private var surfaceTextureHelper: SurfaceTextureHelper? = null

    private var activeSessionId: String = ""
    private var isRemoteDescriptionSet: Boolean = false
    private val pendingIceCandidates = mutableListOf<IceCandidate>()

    // Display Orientation Listener state
    private var displayManager: DisplayManager? = null
    private var displayListener: DisplayManager.DisplayListener? = null
    private var lastOrientation: DisplayOrientation? = null
    private var lastCaptureWidth: Int = 0
    private var lastCaptureHeight: Int = 0

    companion object {
        private const val TAG = "WebRtcPeerConnection"
    }

    init {
        try {
            initFactory()
        } catch (e: Throwable) {
            logW(TAG, "WebRTC PeerConnectionFactory initialization skipped (JVM test mode): ${e.message}")
        }
    }

    private fun initFactory() {
        val options = PeerConnectionFactory.InitializationOptions.builder(context)
            .setEnableInternalTracer(true)
            .createInitializationOptions()
        PeerConnectionFactory.initialize(options)

        val eglContext = rootEglBase?.eglBaseContext
        val encoderFactory = DefaultVideoEncoderFactory(eglContext, true, true)
        val decoderFactory = DefaultVideoDecoderFactory(eglContext)

        peerConnectionFactory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(encoderFactory)
            .setVideoDecoderFactory(decoderFactory)
            .createPeerConnectionFactory()

        logI(TAG, "Initialized Native WebRTC PeerConnectionFactory cleanly")
    }

    fun startSession(sessionId: String, projectionResultData: Intent, iceServersJson: JSONArray? = null) {
        activeSessionId = sessionId
        isRemoteDescriptionSet = false
        pendingIceCandidates.clear()

        val iceServers = mutableListOf<PeerConnection.IceServer>()

        if (iceServersJson != null && iceServersJson.length() > 0) {
            for (i in 0 until iceServersJson.length()) {
                val serverObj = iceServersJson.getJSONObject(i)
                val urlsArr = serverObj.optJSONArray("urls")
                val username = serverObj.optString("username", "")
                val credential = serverObj.optString("credential", "")

                if (urlsArr != null) {
                    val urlList = mutableListOf<String>()
                    for (j in 0 until urlsArr.length()) {
                        urlList.add(urlsArr.getString(j))
                    }
                    val builder = PeerConnection.IceServer.builder(urlList)
                    if (username.isNotEmpty()) builder.setUsername(username)
                    if (credential.isNotEmpty()) builder.setPassword(credential)
                    iceServers.add(builder.createIceServer())
                }
            }
        }

        if (iceServers.isEmpty()) {
            iceServers.add(PeerConnection.IceServer.builder("stun:stun.l.google.com:19302").createIceServer())
            iceServers.add(PeerConnection.IceServer.builder("stun:stun1.l.google.com:19302").createIceServer())
        }

        val rtcConfig = PeerConnection.RTCConfiguration(iceServers).apply {
            sdpSemantics = PeerConnection.SdpSemantics.UNIFIED_PLAN
        }

        peerConnection = peerConnectionFactory?.createPeerConnection(rtcConfig, object : PeerConnection.Observer {
            override fun onSignalingChange(state: PeerConnection.SignalingState?) {
                logD(TAG, "WebRTC SignalingState changed: $state")
            }

            override fun onIceConnectionChange(state: PeerConnection.IceConnectionState?) {
                logI(TAG, "WebRTC IceConnectionState changed: $state (SessionID=$activeSessionId)")
                if (state == PeerConnection.IceConnectionState.CONNECTED) {
                    ScreenCaptureManager.markConnected(activeSessionId)

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
                logD(TAG, "WebRTC IceGatheringState changed: $state")
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

        // Attach WebRTC ScreenCapturerAndroid to VideoTrack (Fail-Closed)
        val attachResult = attachScreenCapturer(projectionResultData)
        if (attachResult.isFailure) {
            val err = attachResult.exceptionOrNull()?.message ?: "Screen capture startup failed"
            logE(TAG, "Rejecting media session initialization for SessionID=$activeSessionId: $err")

            val stopPayload = JSONObject().apply {
                put("session_id", activeSessionId)
                put("reason", err)
            }
            signalPublisher("media.session.stop", stopPayload)
            closeSession()
            return
        }

        ScreenCaptureManager.markReady(activeSessionId)

        // Dispatch media.session.ready to backend/web
        val readyPayload = JSONObject().apply {
            put("session_id", activeSessionId)
            put("status", "ready")
        }
        signalPublisher("media.session.ready", readyPayload)
        logI(TAG, "WebRTC PeerConnection and ScreenCapturer initialized for SessionID=$activeSessionId. Sent media.session.ready")
    }

    private fun attachScreenCapturer(projectionResultData: Intent): Result<Unit> {
        return try {
            val initialGeom = DisplayGeometryProvider.getGeometry(context)

            videoCapturer = ScreenCapturerAndroid(projectionResultData, AgentMediaProjectionCallback())
            surfaceTextureHelper = SurfaceTextureHelper.create("PCP_WebRTC_Thread", rootEglBase?.eglBaseContext)
            videoSource = peerConnectionFactory?.createVideoSource(videoCapturer!!.isScreencast)
                ?: return Result.failure(IllegalStateException("PeerConnectionFactory videoSource is null"))

            lastOrientation = initialGeom.orientation
            val pWidth = ScreenCaptureManager.profileWidth
            val pHeight = ScreenCaptureManager.profileHeight
            val pFps = ScreenCaptureManager.profileFps
            val pBitrate = ScreenCaptureManager.profileBitrateBps

            val (targetW, targetH) = if (initialGeom.orientation == DisplayOrientation.LANDSCAPE) {
                Pair(maxOf(pWidth, pHeight), minOf(pWidth, pHeight))
            } else {
                Pair(minOf(pWidth, pHeight), maxOf(pWidth, pHeight))
            }
            lastCaptureWidth = targetW
            lastCaptureHeight = targetH

            videoCapturer?.initialize(surfaceTextureHelper, context, videoSource?.capturerObserver)
            videoCapturer?.startCapture(targetW, targetH, pFps)

            videoTrack = peerConnectionFactory?.createVideoTrack("video_track_0", videoSource)
                ?: return Result.failure(IllegalStateException("PeerConnectionFactory videoTrack is null"))
            videoTrack?.setEnabled(true)

            val rtpSender = peerConnection?.addTrack(videoTrack, listOf("pcp_media_stream_0"))
                ?: return Result.failure(IllegalStateException("Failed to add video track to PeerConnection"))

            // Apply bitrate & framerate limits to RtpSender
            try {
                val params = rtpSender.parameters
                if (params != null && params.encodings.isNotEmpty()) {
                    for (encoding in params.encodings) {
                        encoding.maxBitrateBps = pBitrate
                        encoding.maxFramerate = pFps
                    }
                    rtpSender.parameters = params
                    logI(TAG, "Applied RtpSender bitrate limit (${pBitrate} bps) & framerate (${pFps} fps)")
                }
            } catch (e: Throwable) {
                logW(TAG, "RtpSender parameters tuning skipped: ${e.message}")
            }

            logI(TAG, "Attached MediaProjection VideoTrack (${targetW}x${targetH} @ ${pFps}fps, max ${pBitrate}bps) to WebRTC PeerConnection successfully")

            // Register DisplayListener for orientation change handling
            registerDisplayListener()
            Result.success(Unit)
        } catch (e: Throwable) {
            logE(TAG, "Error attaching ScreenCapturerAndroid: ${e.message}", e)
            cleanupCapturerResources()
            Result.failure(e)
        }
    }

    private fun cleanupCapturerResources() {
        try {
            unregisterDisplayListener()
            videoCapturer?.stopCapture()
            videoCapturer?.dispose()
            videoCapturer = null
            videoTrack?.dispose()
            videoTrack = null
            videoSource?.dispose()
            videoSource = null
            surfaceTextureHelper?.dispose()
            surfaceTextureHelper = null
        } catch (e: Throwable) {
            logW(TAG, "Error during capturer cleanup: ${e.message}")
        }
    }

    private fun registerDisplayListener() {
        try {
            displayManager = context.getSystemService(Context.DISPLAY_SERVICE) as? DisplayManager
            displayListener = object : DisplayManager.DisplayListener {
                override fun onDisplayAdded(displayId: Int) {}
                override fun onDisplayRemoved(displayId: Int) {}
                override fun onDisplayChanged(displayId: Int) {
                    try {
                        val currentGeom = DisplayGeometryProvider.getGeometry(context)
                        if (currentGeom.orientation != lastOrientation) {
                            logI(TAG, "Display orientation changed: $lastOrientation -> ${currentGeom.orientation}")
                            lastOrientation = currentGeom.orientation
                            val (newW, newH) = if (currentGeom.orientation == DisplayOrientation.LANDSCAPE) {
                                Pair(1280, 720)
                            } else {
                                Pair(720, 1280)
                            }
                            if (newW != lastCaptureWidth || newH != lastCaptureHeight) {
                                lastCaptureWidth = newW
                                lastCaptureHeight = newH
                                videoCapturer?.changeCaptureFormat(newW, newH, 30)
                                logI(TAG, "Changed capture format to ${newW}x${newH}@30fps")
                            }
                        }
                    } catch (e: Throwable) {
                        logW(TAG, "DisplayGeometryProvider error during onDisplayChanged: ${e.message}")
                    }
                }
            }
            displayManager?.registerDisplayListener(displayListener, null)
            logI(TAG, "Registered DisplayManager.DisplayListener for orientation changes")
        } catch (e: Throwable) {
            logE(TAG, "Failed to register DisplayListener: ${e.message}")
        }
    }

    private fun unregisterDisplayListener() {
        try {
            if (displayManager != null && displayListener != null) {
                displayManager?.unregisterDisplayListener(displayListener)
                logI(TAG, "Unregistered DisplayManager.DisplayListener cleanly")
            }
        } catch (e: Throwable) {
            logE(TAG, "Error unregistering DisplayListener: ${e.message}")
        } finally {
            displayListener = null
            displayManager = null
        }
    }

    fun updateCaptureFormat(width: Int, height: Int, fps: Int = 30) {
        if (width == lastCaptureWidth && height == lastCaptureHeight) {
            logD(TAG, "Ignoring duplicate changeCaptureFormat call (${width}x${height})")
            return
        }
        try {
            videoCapturer?.changeCaptureFormat(width, height, fps)
            lastCaptureWidth = width
            lastCaptureHeight = height
            logI(TAG, "Changed WebRTC ScreenCapturer format to ${width}x${height}@${fps}fps")
        } catch (e: Throwable) {
            logE(TAG, "Failed to change WebRTC capture format: ${e.message}")
        }
    }

    fun handleRemoteOffer(sessionId: String, offerSdpText: String) {
        if (sessionId.isNotEmpty() && activeSessionId.isNotEmpty() && sessionId != activeSessionId) {
            logW(TAG, "SessionID mismatch in handleRemoteOffer ($sessionId vs $activeSessionId)")
            return
        }

        if (peerConnection == null) {
            logE(TAG, "Cannot handleRemoteOffer: PeerConnection is null")
            return
        }

        ScreenCaptureManager.markNegotiating(activeSessionId)

        val remoteSdp = SessionDescription(SessionDescription.Type.OFFER, offerSdpText)
        peerConnection?.setRemoteDescription(object : SimpleSdpObserver() {
            override fun onSetSuccess() {
                isRemoteDescriptionSet = true
                logI(TAG, "Set WebRTC Remote Description (OFFER) successfully")

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
                    logI(TAG, "Created and sent WebRTC Local Description (ANSWER) for SessionID=$activeSessionId")
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
            logD(TAG, "Added remote ICE candidate: $sdpMid [$sdpMLineIndex]")
        } else {
            pendingIceCandidates.add(candidate)
            logD(TAG, "Queued remote ICE candidate prior to RemoteDescription set")
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
            unregisterDisplayListener()

            try {
                videoCapturer?.stopCapture()
                videoCapturer?.dispose()
            } catch (e: Throwable) {
                logW(TAG, "Error disposing videoCapturer: ${e.message}")
            }
            videoCapturer = null

            try {
                videoTrack?.dispose()
            } catch (e: Throwable) {
                logW(TAG, "Error disposing videoTrack: ${e.message}")
            }
            videoTrack = null

            try {
                videoSource?.dispose()
            } catch (e: Throwable) {
                logW(TAG, "Error disposing videoSource: ${e.message}")
            }
            videoSource = null

            try {
                surfaceTextureHelper?.dispose()
            } catch (e: Throwable) {
                logW(TAG, "Error disposing surfaceTextureHelper: ${e.message}")
            }
            surfaceTextureHelper = null

            try {
                peerConnection?.close()
                peerConnection?.dispose()
            } catch (e: Throwable) {
                logW(TAG, "Error closing peerConnection: ${e.message}")
            }
            peerConnection = null

            isRemoteDescriptionSet = false
            pendingIceCandidates.clear()

            ScreenCaptureManager.stopCapture(context)

            logI(TAG, "Closed WebRTC PeerConnection session and FGS cleanly for SessionID=$activeSessionId")
            activeSessionId = ""
        } catch (e: Throwable) {
            logE(TAG, "Error closing WebRTC session: ${e.message}")
        }
    }

    inner class AgentMediaProjectionCallback : android.media.projection.MediaProjection.Callback() {
        override fun onStop() {
            logW(TAG, "MediaProjectionCallback.onStop triggered by system for SessionID=$activeSessionId")
            ScreenCaptureManager.onProjectionStoppedBySystem(context)
            closeSession()
        }
    }

    open class SimpleSdpObserver : SdpObserver {
        override fun onCreateSuccess(desc: SessionDescription?) {}
        override fun onSetSuccess() {}
        override fun onCreateFailure(reason: String?) {
            logE(TAG, "SDP Observer onCreateFailure: $reason")
        }
        override fun onSetFailure(reason: String?) {
            logE(TAG, "SDP Observer onSetFailure: $reason")
        }
    }
}
