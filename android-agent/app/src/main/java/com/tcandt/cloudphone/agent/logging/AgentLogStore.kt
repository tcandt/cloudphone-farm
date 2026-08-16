package com.tcandt.cloudphone.agent.logging

import android.content.Context
import android.util.Log
import org.json.JSONObject
import java.io.File
import java.io.FileWriter
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.ConcurrentLinkedDeque
import java.util.concurrent.Executors

data class AgentLogEvent(
    val id: Long,
    val timestamp: String,
    val level: String, // DEBUG, INFO, WARN, ERROR
    val category: String, // ENROLLMENT, AUTH, WSS, HEARTBEAT, PERMISSION, MEDIA, WEBRTC, ICE, CONTROL, APK_INSTALL, SYSTEM
    val eventCode: String,
    val message: String,
    val deviceId: String? = null,
    val agentId: String? = null,
    val sessionId: String? = null,
    val connectionId: String? = null
)

class AgentLogStore private constructor(private val appContext: Context) {

    private val ringBuffer = ConcurrentLinkedDeque<AgentLogEvent>()
    private val maxEvents = 500
    private val maxFileBytes = 2 * 1024 * 1024L // 2MB
    private val fileExecutor = Executors.newSingleThreadExecutor()
    private val logFile: File
    private var sequenceId = 0L

    init {
        val logDir = File(appContext.filesDir, "diagnostics").apply { mkdirs() }
        logFile = File(logDir, "agent_diagnostics.jsonl")
    }

    companion object {
        @Volatile
        private var instance: AgentLogStore? = null

        fun getInstance(context: Context): AgentLogStore {
            return instance ?: synchronized(this) {
                instance ?: AgentLogStore(context.applicationContext).also { instance = it }
            }
        }

        fun maskSensitive(raw: String?): String {
            if (raw.isNullOrEmpty()) return ""
            var masked = raw
            // Mask PCP Tokens: PCP-XXXX-XXXX-XXXX
            masked = masked.replace(Regex("PCP-[A-Za-z0-9]+-[A-Za-z0-9]+-[A-Za-z0-9]+"), "PCP-***-***-***")
            // Mask Ed25519 signatures (base64 ~88 chars)
            masked = masked.replace(Regex("([A-Za-z0-9+/]{40,})={0,2}"), "[MASKED_KEY_OR_SIG]")
            // Mask bearer tokens
            masked = masked.replace(Regex("Bearer\\s+[A-Za-z0-9._-]+"), "Bearer [MASKED_TOKEN]")
            return masked
        }
    }

    fun log(
        level: String,
        category: String,
        eventCode: String,
        message: String,
        deviceId: String? = null,
        agentId: String? = null,
        sessionId: String? = null,
        connectionId: String? = null
    ) {
        val ts = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US).format(Date())
        val maskedMsg = maskSensitive(message)
        val seq: Long
        synchronized(this) {
            sequenceId++
            seq = sequenceId
        }

        val event = AgentLogEvent(
            id = seq,
            timestamp = ts,
            level = level.uppercase(Locale.US),
            category = category.uppercase(Locale.US),
            eventCode = eventCode,
            message = maskedMsg,
            deviceId = deviceId,
            agentId = agentId,
            sessionId = sessionId,
            connectionId = connectionId
        )

        // Android Logcat mirror
        val tag = "PCP_${event.category}"
        when (event.level) {
            "DEBUG" -> Log.d(tag, "[${event.eventCode}] $maskedMsg")
            "INFO" -> Log.i(tag, "[${event.eventCode}] $maskedMsg")
            "WARN" -> Log.w(tag, "[${event.eventCode}] $maskedMsg")
            "ERROR" -> Log.e(tag, "[${event.eventCode}] $maskedMsg")
            else -> Log.i(tag, "[${event.eventCode}] $maskedMsg")
        }

        // 1. In-memory RAM Ring Buffer (Bounded 500 items)
        ringBuffer.addFirst(event)
        while (ringBuffer.size > maxEvents) {
            ringBuffer.pollLast()
        }

        // 2. Persistent Bounded Rotating JSONL File
        fileExecutor.execute {
            try {
                if (logFile.exists() && logFile.length() > maxFileBytes) {
                    val backup = File(logFile.parentFile, "agent_diagnostics_prev.jsonl")
                    if (backup.exists()) backup.delete()
                    logFile.renameTo(backup)
                }

                val json = JSONObject().apply {
                    put("id", event.id)
                    put("ts", event.timestamp)
                    put("lvl", event.level)
                    put("cat", event.category)
                    put("code", event.eventCode)
                    put("msg", event.message)
                    event.deviceId?.let { put("dev_id", it) }
                    event.agentId?.let { put("agent_id", it) }
                    event.sessionId?.let { put("sess_id", it) }
                    event.connectionId?.let { put("conn_id", it) }
                }

                FileWriter(logFile, true).use { writer ->
                    writer.write(json.toString() + "\n")
                }
            } catch (e: Throwable) {
                Log.w("AgentLogStore", "Failed to write diagnostic log file: ${e.message}")
            }
        }
    }

    fun getRecentEvents(limit: Int = 100): List<AgentLogEvent> {
        return ringBuffer.take(limit)
    }

    fun getAllEvents(): List<AgentLogEvent> {
        return ringBuffer.toList()
    }

    fun clear() {
        ringBuffer.clear()
        fileExecutor.execute {
            try {
                if (logFile.exists()) logFile.delete()
            } catch (ignored: Throwable) {}
        }
    }

    fun exportFormattedText(): String {
        val sb = StringBuilder()
        sb.append("=== PCP AGENT DIAGNOSTIC LOG DUMP ===\n")
        sb.append("Exported: ${SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(Date())}\n")
        sb.append("Total Events: ${ringBuffer.size}\n\n")

        for (e in ringBuffer.reversed()) {
            sb.append("[${e.timestamp}] [${e.level}] [${e.category}] ${e.eventCode}: ${e.message}\n")
        }
        return sb.toString()
    }
}
