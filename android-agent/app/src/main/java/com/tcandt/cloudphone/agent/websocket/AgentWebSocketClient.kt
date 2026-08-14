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
import org.json.JSONObject
import java.util.UUID
import java.util.concurrent.TimeUnit

class AgentWebSocketClient(
    private val context: Context,
    private val wssUrl: String,
    private val agentId: String
) {
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

    companion object {
        private const val TAG = "AgentWebSocketClient"
        private const val EMPTY_BODY_SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    }

    fun connect() {
        commandProcessor = CommandProcessor(context) { commandId, status, error, sequence ->
            sendStatusMessage(commandId, status, error, sequence)
        }

        webRtcManager = WebRtcPeerConnectionManager(context) { type, payload ->
            sendWSEnvelope(type, payload)
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
                Log.i(TAG, "Sent media.session.started WSS envelope (SessionID=$sessionId)")
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
                Log.i(TAG, "Sent media.session.stopped WSS envelope (SessionID=$sessionId, reason=$reason)")
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
                Log.w(TAG, "Sent media.session.started [failed] WSS envelope (SessionID=$sessionId, error=$error)")
            }
        }

        val timestamp = (System.currentTimeMillis() / 1000).toString()
        val nonce = "nonce_${UUID.randomUUID().toString().substring(0, 8)}"

        // Canonical WSS Upgrade Signed Message Contract Alignment
        val canonicalMessage = "GET\n/agent/v1/connect\n$EMPTY_BODY_SHA256\n$timestamp\n$nonce"
        val signature = keyStore.signMessage(canonicalMessage)

        val request = Request.Builder()
            .url(wssUrl)
            .addHeader("X-Agent-ID", agentId)
            .addHeader("X-Agent-Timestamp", timestamp)
            .addHeader("X-Agent-Nonce", nonce)
            .addHeader("X-Agent-Signature", signature)
            .build()

        webSocket = client.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                Log.i(TAG, "Connected to Phone Control Platform WSS: $wssUrl")
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
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
                            Log.d(TAG, "Received frame of type: $type")
                        }
                    }
                } catch (e: Exception) {
                    Log.e(TAG, "Failed to parse incoming WS message: ${e.message}", e)
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                Log.e(TAG, "WebSocket error: ${t.message}", t)
                stopHeartbeat()
                scheduleReconnect()
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                Log.w(TAG, "WebSocket connection closed: $reason ($code)")
                stopHeartbeat()
                scheduleReconnect()
            }
        })
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
        Log.i(TAG, "Sent agent.challenge_response to server")
    }

    private fun handleConnectionReady(payload: JSONObject) {
        connectionId = payload.optString("connection_id")
        generation = payload.optLong("generation", 0L)
        Log.i(TAG, "Connection Ready! ConnectionID=$connectionId Generation=$generation")

        startSignedHttpHeartbeat()
    }

    private fun handleMediaSessionStart(payload: JSONObject) {
        val sessionId = payload.optString("session_id")
        val width = payload.optInt("width", 720)
        val height = payload.optInt("height", 1280)
        val bitrate = payload.optInt("bitrate", 2_500_000)
        val fps = payload.optInt("fps", 30)

        Log.i(TAG, "Received media.session.start request for SessionID=$sessionId (${width}x${height} @ ${fps}fps)")
        ScreenCaptureManager.requestCapture(context, sessionId, width, height, bitrate, fps)
    }

    private fun handleMediaSessionStop(payload: JSONObject) {
        val sessionId = payload.optString("session_id")
        Log.i(TAG, "Received media.session.stop request for SessionID=$sessionId")

        webRtcManager?.closeSession()
        ScreenCaptureManager.stopCapture(context)
    }

    private fun handleMediaSignalOffer(payload: JSONObject) {
        val sdp = payload.optString("sdp")
        Log.i(TAG, "Received media.signal.offer from server/web")
        webRtcManager?.handleRemoteOffer(sdp)
    }

    private fun handleMediaSignalCandidate(payload: JSONObject) {
        val sdpMid = payload.optString("sdpMid")
        val sdpMLineIndex = payload.optInt("sdpMLineIndex", 0)
        val candidate = payload.optString("candidate")
        webRtcManager?.handleRemoteCandidate(sdpMid, sdpMLineIndex, candidate)
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
                } catch (e: Exception) {
                    Log.e(TAG, "Signed HTTP heartbeat failed: ${e.message}")
                }
            }
        }
    }

    private fun sendSignedHttpHeartbeat() {
        val serverUrl = configStore.getServerUrl().trimEnd('/')
        val heartbeatUrl = "$serverUrl/api/v1/agents/heartbeat"

        val bodyJson = JSONObject().apply {
            put("connection_id", connectionId)
            put("generation", generation)
            put("sequence", heartbeatSequence++)
            put("battery", 85)
            put("network", "wifi")
            put("cpu_usage", 15.0)
            put("ram_usage", 42.0)
            put("temperature_c", 34.5)
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
                Log.d(TAG, "10s Signed HTTP Heartbeat successful! (HTTP ${response.code})")
            } else {
                Log.w(TAG, "Signed HTTP Heartbeat status: ${response.code}")
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
        Log.i(TAG, "Sent command.status envelope to server (status=$status, seq=$sequence)")
    }

    private fun scheduleReconnect() {
        Log.i(TAG, "Scheduling WSS reconnect in 3 seconds...")
        Thread.sleep(3000)
        connect()
    }

    fun disconnect() {
        stopHeartbeat()
        webRtcManager?.closeSession()
        webSocket?.close(1000, "App Service Stopped")
    }
}
