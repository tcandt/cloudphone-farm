package com.tcandt/cloudphone.agent.command

import android.content.Context
import android.util.Log
import com.tcandt.cloudphone.agent.accessibility.DeviceControlService
import org.json.JSONObject

class CommandProcessor(
    private val context: Context,
    private val statusPublisher: (commandId: String, status: String, error: String?, sequence: Int) -> Unit
) {
    private val fencingStore = FencingStore(context)
    private val journal = CommandJournal(context)

    companion object {
        private const val TAG = "CommandProcessor"
    }

    fun processCommand(commandDispatch: JSONObject) {
        val commandId = commandDispatch.optString("command_id")
        val fencingToken = commandDispatch.optLong("fencing_token", 0L)
        val commandType = commandDispatch.optString("command_type")
        val payload = commandDispatch.optJSONObject("payload") ?: JSONObject()

        if (commandId.isEmpty()) {
            Log.e(TAG, "Received command dispatch without command_id")
            return
        }

        // 1. Deduplication check against persistent SQLite journal
        val existingRecord = journal.getRecord(commandId)
        if (existingRecord != null) {
            Log.i(TAG, "Duplicate command $commandId detected in journal. Resending cached status ${existingRecord.status}")
            statusPublisher(commandId, existingRecord.status, existingRecord.error, 3)
            return
        }

        // 2. Monotonic Fencing Token check
        if (!fencingStore.validateAndUpdate(fencingToken)) {
            val errStr = "Stale fencing token $fencingToken (highest known: ${fencingStore.getHighestFencingToken()})"
            Log.w(TAG, "Rejecting command $commandId: $errStr")
            journal.saveRecord(commandId, fencingToken, "failed", errStr)
            statusPublisher(commandId, "failed", errStr, 3)
            return
        }

        // 3. Sequenced execution reporting: Sequence 1 -> ACK
        statusPublisher(commandId, "ack", null, 1)

        // Sequence 2 -> EXECUTING
        statusPublisher(commandId, "executing", null, 2)

        val service = DeviceControlService.instance
        if (service == null) {
            val errStr = "DeviceControlService AccessibilityService is not enabled or connected"
            Log.e(TAG, errStr)
            journal.saveRecord(commandId, fencingToken, "failed", errStr)
            statusPublisher(commandId, "failed", errStr, 3)
            return
        }

        // 4. Physical gesture execution via AccessibilityService
        when (commandType) {
            "gesture.touch" -> {
                val x = payload.optDouble("x", 0.0).toFloat()
                val y = payload.optDouble("y", 0.0).toFloat()
                service.performTouch(x, y) { success ->
                    val status = if (success) "succeeded" else "failed"
                    val err = if (success) null else "Accessibility touch gesture failed"
                    journal.saveRecord(commandId, fencingToken, status, err)
                    statusPublisher(commandId, status, err, 3)
                }
            }
            "gesture.swipe" -> {
                val startX = payload.optDouble("startX", 0.0).toFloat()
                val startY = payload.optDouble("startY", 0.0).toFloat()
                val endX = payload.optDouble("endX", 0.0).toFloat()
                val endY = payload.optDouble("endY", 0.0).toFloat()
                val durationMs = payload.optLong("durationMs", 300L)
                service.performSwipe(startX, startY, endX, endY, durationMs) { success ->
                    val status = if (success) "succeeded" else "failed"
                    val err = if (success) null else "Accessibility swipe gesture failed"
                    journal.saveRecord(commandId, fencingToken, status, err)
                    statusPublisher(commandId, status, err, 3)
                }
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
}
