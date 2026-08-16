package com.tcandt.cloudphone.agent.websocket

import android.content.Context
import android.util.Log
import com.tcandt.cloudphone.agent.command.CommandProcessor
import com.tcandt.cloudphone.agent.config.AgentConfigStore
import com.tcandt.cloudphone.agent.media.ScreenCaptureManager
import com.tcandt.cloudphone.agent.media.webrtc.WebRtcPeerConnectionManager
import com.tcandt.cloudphone.agent.security.AgentKeyStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID
import java.util.concurrent.TimeUnit

class AgentWebSocketClient(
    private val context: Context,
    private val wssUrl: String,
    private val agentId: String
) {
    private fun logD(tag: String, msg: String) { try { Log.d(tag, msg) } catch (_: Throwable) {} }
    private fun logI(tag: String, msg: String) { try { Log.i(tag, msg) } catch (_: Throwable) {} }
    private fun logW(tag: String, msg: String) { try { Log.w(tag, msg) } catch (_: Throwable) {} }
    private fun logE(tag: String, msg: String, t: Throwable? = null) { try { if (t != null) Log.e(tag, msg, t) else Log.e(tag, msg) } catch (_: Throwable) {} }

    private val client = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(15, TimeUnit.SECONDS)
        .build()

    private var webSocket: WebSocket? = null
    private val keyStore = AgentKeyStore(context)
    private val configStore = AgentConfigStore(context)
    private var commandProcessor: CommandProcessor? = null
    private var webRtcManager: WebRtcPeerConnectionManager? = null

    private var connectionId: String = ""
    private var generation: Long = 0
    private var heartbeatSequence: Long = 1
    private var heartbeatJob: Job? = null
    private var pendingIceServersJson: JSONArray? = null

    companion object {
        private const val TAG = "AgentWebSocketClient"
        private const val EMPTY_BODY_SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    }

    private var isExplicitlyStopped = false

    var currentSocketEpoch: Long = 0L
        private set

    val isStopped: Boolean get() = isExplicitlyStopped

    fun isSocketStale(targetEpoch: Long): Boolean = synchronized(this) {
        isExplicitlyStopped || targetEpoch != socketEpoch
    }

    init {
        commandProcessor = CommandProcessor(context) { commandId, status, error, sequence ->
            sendStatusMessage(commandId, status, error, sequence)
        }

        webRtcManager = WebRtcPeerConnectionManager(context) { type, payload ->
            sendWSEnvelope(type, payload)
        }

        ScreenCaptureManager.bindMediaSessionStarter { sessionId, projectionIntent ->
            logI(TAG, "WEBRTC_START_DELEGATED: Calling WebRtcPeerConnectionManager.startSession for SessionID=$sessionId")
            webRtcManager?.startSession(sessionId, projectionIntent, pendingIceServersJson)
        }

        ScreenCaptureManager.sessionListener = object : ScreenCaptureManager.SessionStateListener {
            override fun onSessionStarted(sessionId: String) {
                val respPayload = JSONObject().apply {
                    put("session_id", sessionId)
                    put("status", "started")
                }
                val envelope = JSONObject().apply {
                    put("type", "media.session.started")
                    put("message_id", "msg_${System.nanoTime()}")
                    put("payload", respPayload)
                }
                webSocket?.send(envelope.toString())
                logI(TAG, "Sent media.session.started WSS envelope (SessionID=$sessionId)")
            }

            override fun onSessionStopped(sessionId: String, reason: String) {
                webRtcManager?.closeSession()
                val respPayload = JSONObject().apply {
                    put("session_id", sessionId)
                    put("status", "stopped")
                    put("reason", reason)
                }
                val envelope = JSONObject().apply {
                    put("type", "media.session.stopped")
                    put("message_id", "msg_${System.nanoTime()}")
                    put("payload", respPayload)
                }
                webSocket?.send(envelope.toString())
                logI(TAG, "Sent media.session.stopped WSS envelope (SessionID=$sessionId, reason=$reason)")
            }

            override fun onSessionFailed(sessionId: String, error: String) {
                webRtcManager?.closeSession()
                val respPayload = JSONObject().apply {
                    put("session_id", sessionId)
                    put("status", "failed")
                    put("error_message", error)
                }
                val envelope = JSONObject().apply {
                    put("type", "media.session.started")
                    put("message_id", "msg_${System.nanoTime()}")
                    put("payload", respPayload)
                }
                webSocket?.send(envelope.toString())
                logW(TAG, "Sent media.session.started [failed] WSS envelope (SessionID=$sessionId, error=$error)")
            }
        }
    }

    private var socketEpoch: Long = 0L

    fun connect() {
        if (isExplicitlyStopped) {
            logD(TAG, "connect() ignored: AgentWebSocketClient is explicitly stopped")
            return
        }

        val attemptEpoch = synchronized(this) {
            socketEpoch++
            currentSocketEpoch = socketEpoch
            socketEpoch
        }
        logI(TAG, "Initiating WSS connection attempt (Epoch=$attemptEpoch)...")

        try {
            val timestamp = (System.currentTimeMillis() / 1000).toString()
            val nonce = "nonce_${UUID.randomUUID().toString().substring(0, 8)}"

            // Canonical WSS Upgrade Signed Message Contract Alignment
            val canonicalMessage = "GET\n/agent/v1/connect\n$EMPTY_BODY_SHA256\n$timestamp\n$nonce"
            val signature = try { keyStore.signMessage(canonicalMessage) } catch (e: Throwable) { "" }

            val request = Request.Builder()
                .url(wssUrl)
                .addHeader("X-Agent-ID", agentId)
                .addHeader("X-Agent-Timestamp", timestamp)
                .addHeader("X-Agent-Nonce", nonce)
                .addHeader("X-Agent-Signature", signature)
                .build()

            webSocket = client.newWebSocket(request, object : WebSocketListener() {
                private fun isStale(ws: WebSocket): Boolean {
                    return synchronized(this@AgentWebSocketClient) {
                        isExplicitlyStopped || attemptEpoch != socketEpoch || ws != webSocket
                    }
                }

                override fun onOpen(webSocket: WebSocket, response: Response) {
                    if (isStale(webSocket)) {
                        logW(TAG, "Ignoring onOpen from stale socket (Epoch=$attemptEpoch, activeEpoch=$socketEpoch)")
                        webSocket.close(1000, "Stale Socket Connection")
                        return
                    }
                    logI(TAG, "Connected to Phone Control Platform WSS (Epoch=$attemptEpoch): $wssUrl")
                }

                override fun onMessage(webSocket: WebSocket, text: String) {
                    if (isStale(webSocket)) {
                        logW(TAG, "Ignoring onMessage from stale socket (Epoch=$attemptEpoch, activeEpoch=$socketEpoch)")
                        return
                    }
                    try {
                        val envelope = JSONObject(text)
                        val type = envelope.optString("type")
                        val payload = envelope.optJSONObject("payload") ?: JSONObject()

                        when (type) {
                            "server.challenge" -> {
                                handleChallenge(webSocket, payload)
                            }
                            "connection.ready" -> {
                                handleConnectionReady(payload)
                            }
                            "command.dispatch" -> {
                                commandProcessor?.enqueueCommand(payload)
                            }
                            "media.session.start" -> {
                                handleMediaSessionStart(payload)
                            }
                            "media.session.stop" -> {
                                handleMediaSessionStop(payload)
                            }
                            "media.signal.offer" -> {
                                handleMediaSignalOffer(payload)
                            }
                            "media.signal.candidate" -> {
                                handleMediaSignalCandidate(payload)
                            }
                            else -> {
                                logD(TAG, "Received frame of type: $type")
                            }
                        }
                    } catch (e: Throwable) {
                        logE(TAG, "Failed to parse incoming WS message: ${e.message}", e)
                    }
                }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                    if (isStale(webSocket)) {
                        logW(TAG, "Ignoring onFailure from stale socket (Epoch=$attemptEpoch, activeEpoch=$socketEpoch)")
                        return
                    }
                    logE(TAG, "WebSocket error (Epoch=$attemptEpoch): ${t.message}", t)
                    stopHeartbeat()
                    scheduleReconnect()
                }

                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                    if (isStale(webSocket)) {
                        logW(TAG, "Ignoring onClosed from stale socket (Epoch=$attemptEpoch, activeEpoch=$socketEpoch)")
                        return
                    }
                    logW(TAG, "WebSocket connection closed (Epoch=$attemptEpoch): $reason ($code)")
                    stopHeartbeat()
                    scheduleReconnect()
                }
            })
        } catch (e: Throwable) {
            logW(TAG, "WebSocket connection creation skipped (JVM test mode or network error): ${e.message}")
        }
    }

    private fun handleChallenge(webSocket: WebSocket, challengePayload: JSONObject) {
        val challengeNonce = challengePayload.optString("challenge_nonce")
        val signature = keyStore.signMessage(challengeNonce)

        val responsePayload = JSONObject().apply {
            put("agent_id", agentId)
            put("challenge_nonce", challengeNonce)
            put("challenge_signature", signature)
        }

        val envelope = JSONObject().apply {
            put("type", "agent.challenge_response")
            put("message_id", "msg_${System.nanoTime()}")
            put("payload", responsePayload)
        }

        webSocket.send(envelope.toString())
        logI(TAG, "Sent agent.challenge_response to server")
    }

    private fun handleConnectionReady(payload: JSONObject) {
        connectionId = payload.optString("connection_id")
        generation = payload.optLong("generation", 0L)
        reconnectAttempt = 0
        logI(TAG, "Connection Ready! ConnectionID=$connectionId Generation=$generation")

        startSignedHttpHeartbeat()
    }

    private fun handleMediaSessionStart(payload: JSONObject) {
        val sessionId = payload.optString("session_id")
        val width = payload.optInt("width", 540)
        val height = payload.optInt("height", 960)
        val bitrate = payload.optInt("bitrate", 1_500_000)
        val fps = payload.optInt("fps", 24)

        pendingIceServersJson = payload.optJSONArray("ice_servers")

        logI(TAG, "Received media.session.start request for SessionID=$sessionId (${width}x${height} @ ${fps}fps)")
        ScreenCaptureManager.setProfile(width, height, fps, bitrate)
        ScreenCaptureManager.requestConsent(context, sessionId)
    }

    private fun handleMediaSessionStop(payload: JSONObject) {
        val sessionId = payload.optString("session_id")
        logI(TAG, "Received media.session.stop request for SessionID=$sessionId")

        if (sessionId.isNotEmpty() && ScreenCaptureManager.activeSessionId.isNotEmpty() && sessionId != ScreenCaptureManager.activeSessionId) {
            logW(TAG, "Ignoring stale media.session.stop request ($sessionId vs active ${ScreenCaptureManager.activeSessionId})")
            return
        }

        pendingIceServersJson = null
        webRtcManager?.closeSession()
        ScreenCaptureManager.stopCapture(context)
    }

    private fun handleMediaSignalOffer(payload: JSONObject) {
        val sessionId = payload.optString("session_id")
        val sdp = payload.optString("sdp")
        logI(TAG, "Received media.signal.offer from server/web for SessionID=$sessionId")
        webRtcManager?.handleRemoteOffer(sessionId, sdp)
    }

    private fun handleMediaSignalCandidate(payload: JSONObject) {
        val sessionId = payload.optString("session_id")
        val sdpMid = payload.optString("sdpMid")
        val sdpMLineIndex = payload.optInt("sdpMLineIndex", 0)
        val candidate = payload.optString("candidate")
        webRtcManager?.handleRemoteCandidate(sessionId, sdpMid, sdpMLineIndex, candidate)
    }

    private fun sendWSEnvelope(type: String, payload: JSONObject) {
        val envelope = JSONObject().apply {
            put("type", type)
            put("message_id", "msg_${System.nanoTime()}")
            put("payload", payload)
        }
        webSocket?.send(envelope.toString())
    }

    private fun startSignedHttpHeartbeat() {
        stopHeartbeat()
        heartbeatJob = CoroutineScope(Dispatchers.IO).launch {
            while (true) {
                delay(10000) // 10s signed HTTP heartbeat for Redis presence TTL renewal
                try {
                    sendSignedHttpHeartbeat()
                } catch (e: Throwable) {
                    logE(TAG, "Signed HTTP heartbeat failed: ${e.message}")
                }
            }
        }
    }

    private fun sendSignedHttpHeartbeat() {
        val serverUrl = configStore.getServerUrl().trimEnd('/')
        val heartbeatUrl = "$serverUrl/api/v1/agents/heartbeat"

        val bodyJson = com.tcandt.cloudphone.agent.telemetry.HardwareTelemetryCollector.collectMetrics(context).apply {
            put("connection_id", connectionId)
            put("generation", generation)
            put("sequence", heartbeatSequence++)
            put("key_protection", keyStore.getKeyProtectionMetadata())
        }

        val bodyBytes = bodyJson.toString().toByteArray(Charsets.UTF_8)
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        val bodyHashBytes = digest.digest(bodyBytes)
        val bodyHashHex = bodyHashBytes.joinToString("") { "%02x".format(it) }

        val timestamp = (System.currentTimeMillis() / 1000).toString()
        val nonce = "nonce_${UUID.randomUUID().toString().substring(0, 8)}"

        val canonicalMsg = "POST\n/api/v1/agents/heartbeat\n$bodyHashHex\n$timestamp\n$nonce"
        val signature = keyStore.signMessage(canonicalMsg)

        val request = Request.Builder()
            .url(heartbeatUrl)
            .post(bodyJson.toString().toRequestBody("application/json".toMediaType()))
            .addHeader("X-Agent-ID", agentId)
            .addHeader("X-Agent-Timestamp", timestamp)
            .addHeader("X-Agent-Nonce", nonce)
            .addHeader("X-Agent-Signature", signature)
            .build()

        client.newCall(request).execute().use { response ->
            if (response.isSuccessful) {
                logD(TAG, "10s Signed HTTP Heartbeat successful! (HTTP ${response.code})")
            } else {
                logW(TAG, "Signed HTTP Heartbeat status: ${response.code}")
            }
        }
    }

    private fun stopHeartbeat() {
        heartbeatJob?.cancel()
        heartbeatJob = null
    }

    private fun sendStatusMessage(commandId: String, status: String, error: String?, sequence: Int) {
        val payload = JSONObject().apply {
            put("command_id", commandId)
            put("status", status)
            put("sequence", sequence)
            if (!error.isNullOrEmpty()) {
                put("error_message", error)
            }
            put("timestamp", System.currentTimeMillis() / 1000)
        }

        val envelope = JSONObject().apply {
            put("type", "command.status")
            put("message_id", "msg_${System.nanoTime()}")
            put("payload", payload)
        }

        webSocket?.send(envelope.toString())
        logI(TAG, "Sent command.status envelope to server (status=$status, seq=$sequence)")
    }

    private var isReconnecting = false
    private var reconnectJob: Job? = null
    private var reconnectAttempt = 0

    private fun scheduleReconnect() {
        if (isExplicitlyStopped) {
            logD(TAG, "scheduleReconnect() ignored: AgentWebSocketClient is explicitly stopped")
            return
        }

        synchronized(this) {
            if (isReconnecting) return
            isReconnecting = true
        }

        reconnectJob?.cancel()
        reconnectJob = CoroutineScope(Dispatchers.IO).launch {
            val baseDelay = minOf(1000L * (1 shl minOf(reconnectAttempt, 5)), 30000L)
            val jitter = (0..1000).random().toLong()
            val delayMs = baseDelay + jitter

            logI(TAG, "Scheduling WSS supervised reconnect (attempt #${reconnectAttempt + 1}) in ${delayMs}ms...")
            delay(delayMs)

            reconnectAttempt++
            synchronized(this@AgentWebSocketClient) {
                isReconnecting = false
            }
            if (!isExplicitlyStopped) {
                connect()
            }
        }
    }

    fun disconnect() {
        synchronized(this) {
            isExplicitlyStopped = true
            socketEpoch++
            currentSocketEpoch = socketEpoch
        }
        reconnectJob?.cancel()
        reconnectJob = null
        stopHeartbeat()
        webRtcManager?.closeSession()
        webSocket?.close(1000, "App Service Stopped")
    }
}
