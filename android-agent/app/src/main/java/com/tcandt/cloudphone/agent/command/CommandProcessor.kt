package com.tcandt.cloudphone.agent.command

import android.content.Context
import android.util.Log
import com.tcandt.cloudphone.agent.accessibility.DeviceControlService
import com.tcandt.cloudphone.agent.config.AgentConfigStore
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

class CommandProcessor(
    private val context: Context,
    private val statusPublisher: (commandId: String, status: String, error: String?, sequence: Int) -> Unit
) {
    private val configStore = AgentConfigStore(context)
    private val fencingStore = FencingStore(context)
    private val journal = CommandJournal(context)
    private val commandChannel = Channel<JSONObject>(Channel.UNLIMITED)

    companion object {
        private const val TAG = "CommandProcessor"
    }

    init {
        // Start single-threaded Serial Worker Coroutine for physical gestures
        CoroutineScope(Dispatchers.IO).launch {
            for (commandDispatch in commandChannel) {
                processSingleCommandSerial(commandDispatch)
            }
        }
    }

    fun enqueueCommand(commandDispatch: JSONObject) {
        commandChannel.trySend(commandDispatch)
    }

    private suspend fun processSingleCommandSerial(commandDispatch: JSONObject) {
        val commandId = commandDispatch.optString("command_id")
        val deviceId = commandDispatch.optString("device_id")
        val fencingToken = commandDispatch.optLong("fencing_token", 0L)
        val commandType = commandDispatch.optString("command_type")
        val expiresAtStr = commandDispatch.optString("expires_at")
        val payload = commandDispatch.optJSONObject("payload") ?: JSONObject()

        if (commandId.isEmpty()) {
            Log.e(TAG, "Received command dispatch without command_id")
            return
        }

        // 1. Strict Target Device ID Validation
        val myDeviceId = configStore.getDeviceId()
        if (deviceId.isEmpty() || myDeviceId.isEmpty() || deviceId != myDeviceId) {
            Log.w(TAG, "Command $commandId target device $deviceId mismatch with local device $myDeviceId")
            return
        }

        // 2. TTL Expiration Check before ACK
        if (isExpired(expiresAtStr)) {
            Log.w(TAG, "Command $commandId expired (expires_at=$expiresAtStr). Dropping execution.")
            journal.saveRecord(commandId, fencingToken, "expired", "TTL expired before execution")
            statusPublisher(commandId, "expired", "TTL expired before execution", 3)
            return
        }

        // 3. Persistent SQLite Deduplication & Crash Window Protection Check
        val existingRecord = journal.getRecord(commandId)
        if (existingRecord != null) {
            if (existingRecord.status == "succeeded" || existingRecord.status == "failed" || existingRecord.status == "expired") {
                Log.i(TAG, "Duplicate command $commandId detected in journal. Resending cached status ${existingRecord.status}")
                statusPublisher(commandId, existingRecord.status, existingRecord.error, 3)
                return
            } else if (existingRecord.status == "executing") {
                Log.w(TAG, "Command $commandId was interrupted in 'executing' state (process crash/restart). Preventing 2nd touch, marking failed")
                journal.saveRecord(commandId, fencingToken, "failed", "Interrupted during process restart")
                statusPublisher(commandId, "failed", "Interrupted during process restart", 3)
                return
            }
        }

        // 4. Monotonic Fencing Token check
        if (!fencingStore.validateAndUpdate(fencingToken)) {
            val errStr = "Stale fencing token $fencingToken (highest known: ${fencingStore.getHighestFencingToken()})"
            Log.w(TAG, "Rejecting command $commandId: $errStr")
            journal.saveRecord(commandId, fencingToken, "failed", errStr)
            statusPublisher(commandId, "failed", errStr, 3)
            return
        }

        // 5. Sequenced execution reporting: Sequence 1 -> ACK
        statusPublisher(commandId, "ack", null, 1)

        // 6. Pre-execution Durable Crash Window Protection: Record 'executing' in SQLite BEFORE physical touch
        journal.saveRecord(commandId, fencingToken, "executing", null)
        statusPublisher(commandId, "executing", null, 2)

        val service = DeviceControlService.instance
        if (service == null) {
            val errStr = "DeviceControlService AccessibilityService is not enabled or connected"
            Log.e(TAG, errStr)
            journal.saveRecord(commandId, fencingToken, "failed", errStr)
            statusPublisher(commandId, "failed", errStr, 3)
            return
        }

        // 7. Serial Physical Gesture Execution using CompletableDeferred for async callbacks
        when (commandType) {
            "gesture.touch" -> {
                val x = payload.optDouble("x", 0.0).toFloat()
                val y = payload.optDouble("y", 0.0).toFloat()
                val deferred = CompletableDeferred<Boolean>()
                service.performTouch(x, y) { success ->
                    deferred.complete(success)
                }
                val success = deferred.await()
                val status = if (success) "succeeded" else "failed"
                val err = if (success) null else "Accessibility touch gesture failed"
                journal.saveRecord(commandId, fencingToken, status, err)
                statusPublisher(commandId, status, err, 3)
            }
            "gesture.swipe" -> {
                val startX = payload.optDouble("startX", 0.0).toFloat()
                val startY = payload.optDouble("startY", 0.0).toFloat()
                val endX = payload.optDouble("endX", 0.0).toFloat()
                val endY = payload.optDouble("endY", 0.0).toFloat()
                val durationMs = payload.optLong("durationMs", 300L)
                val deferred = CompletableDeferred<Boolean>()
                service.performSwipe(startX, startY, endX, endY, durationMs) { success ->
                    deferred.complete(success)
                }
                val success = deferred.await()
                val status = if (success) "succeeded" else "failed"
                val err = if (success) null else "Accessibility swipe gesture failed"
                journal.saveRecord(commandId, fencingToken, status, err)
                statusPublisher(commandId, status, err, 3)
            }
            "input.text" -> {
                val text = payload.optString("text", "")
                val success = service.performTextInput(text)
                val status = if (success) "succeeded" else "failed"
                val err = if (success) null else "Failed to find focused text input node"
                journal.saveRecord(commandId, fencingToken, status, err)
                statusPublisher(commandId, status, err, 3)
            }
            "global.back", "global.home", "global.recents" -> {
                val success = service.performNavigation(commandType)
                val status = if (success) "succeeded" else "failed"
                val err = if (success) null else "Global action $commandType failed"
                journal.saveRecord(commandId, fencingToken, status, err)
                statusPublisher(commandId, status, err, 3)
            }
            else -> {
                val errStr = "Unsupported command type $commandType"
                journal.saveRecord(commandId, fencingToken, "failed", errStr)
                statusPublisher(commandId, "failed", errStr, 3)
            }
        }
    }

    private fun isExpired(expiresAtStr: String): Boolean {
        if (expiresAtStr.isEmpty()) return false
        return try {
            val sdf = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.US)
            sdf.timeZone = TimeZone.getTimeZone("UTC")
            val expiresDate = sdf.parse(expiresAtStr) ?: return false
            System.currentTimeMillis() > expiresDate.time
        } catch (e: Exception) {
            false
        }
    }
}
