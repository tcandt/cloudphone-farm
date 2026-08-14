package com.tcandt.cloudphone.agent.websocket

import android.content.Context
import android.util.Log
import com.tcandt/cloudphone.agent.command.CommandProcessor
import com.tcandt/cloudphone.agent.security.AgentKeyStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request
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
    private var commandProcessor: CommandProcessor? = null

    private var connectionId: String = ""
    private var generation: Long = 0
    private var heartbeatJob: Job? = null

    companion object {
        private const val TAG = "AgentWebSocketClient"
        private const val EMPTY_BODY_SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    }

    fun connect() {
        commandProcessor = CommandProcessor(context) { commandId, status, error, sequence ->
            sendStatusMessage(commandId, status, error, sequence)
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

        startHeartbeat()
    }

    private fun startHeartbeat() {
        stopHeartbeat()
        heartbeatJob = CoroutineScope(Dispatchers.IO).launch {
            while (true) {
                delay(10000) // 10s heartbeat
                val pingPayload = JSONObject().apply {
                    put("agent_id", agentId)
                    put("connection_id", connectionId)
                    put("timestamp", System.currentTimeMillis() / 1000)
                }
                val pingEnvelope = JSONObject().apply {
                    put("type", "agent.ping")
                    put("message_id", "msg_${System.nanoTime()}")
                    put("payload", pingPayload)
                }
                webSocket?.send(pingEnvelope.toString())
                Log.d(TAG, "Sent 10s agent.ping heartbeat to server")
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
        webSocket?.close(1000, "App Service Stopped")
    }
}
