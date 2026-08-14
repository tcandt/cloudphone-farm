package com.tcandt.cloudphone.agent.command

import android.content.Context
import android.util.Log
import com.tcandt.cloudphone.agent.accessibility.DeviceControlService
import com.tcandt.cloudphone.agent.config.AgentConfigStore
import com.tcandt.cloudphone.agent.control.DisplayGeometryProvider
import com.tcandt.cloudphone.agent.control.DisplayOrientation
import com.tcandt.cloudphone.agent.control.NormalizedCoordinateMapper
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.text.SimpleDateFormat
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

        // 2. Strict Fail-Closed Expiration Validation
        if (expiresAtStr.isBlank()) {
            Log.w(TAG, "Rejecting command $commandId: missing required expires_at TTL")
            journal.saveRecord(commandId, fencingToken, "failed", "Missing required expires_at TTL")
            statusPublisher(commandId, "failed", "Missing required expires_at TTL", 3)
            return
        }

        val expiresAtMs = try {
            val sdf = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.US)
            sdf.timeZone = TimeZone.getTimeZone("UTC")
            sdf.parse(expiresAtStr)?.time ?: run {
                Log.w(TAG, "Rejecting command $commandId: invalid expires_at format $expiresAtStr")
                journal.saveRecord(commandId, fencingToken, "failed", "Invalid expires_at format")
                statusPublisher(commandId, "failed", "Invalid expires_at format", 3)
                return
            }
        } catch (_: Exception) {
            Log.w(TAG, "Rejecting command $commandId: unparseable expires_at $expiresAtStr")
            journal.saveRecord(commandId, fencingToken, "failed", "Unparseable expires_at format")
            statusPublisher(commandId, "failed", "Unparseable expires_at format", 3)
            return
        }

        if (System.currentTimeMillis() > expiresAtMs) {
            Log.w(TAG, "Command $commandId expired (expires_at=$expiresAtStr). Rejecting execution.")
            journal.saveRecord(commandId, fencingToken, "expired", "TTL expired before execution")
            statusPublisher(commandId, "expired", "TTL expired before execution", 3)
            return
        }

        // 3. Persistent SQLite Deduplication & Crash Window Protection Check
        val existingRecord = journal.getRecord(commandId)
        if (existingRecord != null) {
            if (existingRecord.status == "executing") {
                val errStr = "Interrupted during process restart"
                Log.w(TAG, "Command $commandId was interrupted while executing during process restart. Marking failed.")
                journal.saveRecord(commandId, fencingToken, "failed", errStr)
                statusPublisher(commandId, "failed", errStr, 3)
                return
            }
            if (existingRecord.status == "succeeded" || existingRecord.status == "failed" || existingRecord.status == "expired") {
                Log.i(TAG, "Duplicate command $commandId detected in journal. Resending cached status ${existingRecord.status}")
                statusPublisher(commandId, existingRecord.status, existingRecord.error, 3)
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

        // 5. Validate Coordinate Space and Orientation for Gesture Commands
        if (commandType == "gesture.touch" || commandType == "gesture.swipe") {
            val space = payload.optString("coordinateSpace", "")
            if (space != "normalized_display_v1") {
                val errStr = "Invalid coordinateSpace '$space': required 'normalized_display_v1'"
                Log.w(TAG, "Rejecting command $commandId: $errStr")
                journal.saveRecord(commandId, fencingToken, "failed", errStr)
                statusPublisher(commandId, "failed", errStr, 3)
                return
            }
        }

        // 6. Sequenced execution reporting: Sequence 1 -> ACK
        statusPublisher(commandId, "ack", null, 1)

        // 7. Pre-execution Durable Crash Window Protection: Record 'executing' in SQLite BEFORE physical touch
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

        // Fetch current physical screen geometry and orientation with fail-closed protection
        val geometry = try {
            DisplayGeometryProvider.getGeometry(context)
        } catch (e: IllegalStateException) {
            val errStr = e.message ?: "Failed to retrieve physical display geometry"
            Log.e(TAG, "Rejecting command $commandId: $errStr")
            journal.saveRecord(commandId, fencingToken, "failed", errStr)
            statusPublisher(commandId, "failed", errStr, 3)
            return
        }

        // Orientation guard
        val targetOrientation = payload.optString("orientation", "")
        if (targetOrientation.isNotEmpty()) {
            val currentOrientStr = if (geometry.orientation == DisplayOrientation.LANDSCAPE) "landscape" else "portrait"
            if (targetOrientation != currentOrientStr) {
                val errStr = "ORIENTATION_MISMATCH: command expected $targetOrientation but screen is $currentOrientStr"
                Log.w(TAG, errStr)
                journal.saveRecord(commandId, fencingToken, "failed", errStr)
                statusPublisher(commandId, "failed", errStr, 3)
                return
            }
        }

        // 8. Serial Physical Gesture Execution using CompletableDeferred for async callbacks
        when (commandType) {
            "gesture.touch" -> {
                val normX = payload.optDouble("x", 0.0).toFloat()
                val normY = payload.optDouble("y", 0.0).toFloat()

                val point = try {
                    NormalizedCoordinateMapper.map(normX, normY, geometry.widthPx, geometry.heightPx)
                } catch (e: IllegalArgumentException) {
                    val errStr = e.message ?: "Invalid touch coordinates"
                    Log.e(TAG, errStr)
                    journal.saveRecord(commandId, fencingToken, "failed", errStr)
                    statusPublisher(commandId, "failed", errStr, 3)
                    return
                }

                Log.i(TAG, "Touch normalized ($normX, $normY) -> Physical Px (${point.x}, ${point.y}) on screen ${geometry.widthPx}x${geometry.heightPx}")

                val deferred = CompletableDeferred<Boolean>()
                service.performTouch(point.x, point.y) { success ->
                    deferred.complete(success)
                }
                val success = deferred.await()
                val status = if (success) "succeeded" else "failed"
                val err = if (success) null else "Accessibility touch gesture failed"
                journal.saveRecord(commandId, fencingToken, status, err)
                statusPublisher(commandId, status, err, 3)
            }
            "gesture.swipe" -> {
                val startNormX = payload.optDouble("startX", 0.0).toFloat()
                val startNormY = payload.optDouble("startY", 0.0).toFloat()
                val endNormX = payload.optDouble("endX", 0.0).toFloat()
                val endNormY = payload.optDouble("endY", 0.0).toFloat()
                val durationMs = payload.optLong("durationMs", 300L)

                val (startPt, endPt) = try {
                    Pair(
                        NormalizedCoordinateMapper.map(startNormX, startNormY, geometry.widthPx, geometry.heightPx),
                        NormalizedCoordinateMapper.map(endNormX, endNormY, geometry.widthPx, geometry.heightPx)
                    )
                } catch (e: IllegalArgumentException) {
                    val errStr = e.message ?: "Invalid swipe coordinates"
                    Log.e(TAG, errStr)
                    journal.saveRecord(commandId, fencingToken, "failed", errStr)
                    statusPublisher(commandId, "failed", errStr, 3)
                    return
                }

                Log.d(TAG, "Swipe normalized ($startNormX, $startNormY)->($endNormX, $endNormY) -> Physical Px (${startPt.x}, ${startPt.y})->(${endPt.x}, ${endPt.y}) on screen ${geometry.widthPx}x${geometry.heightPx}")

                val deferred = CompletableDeferred<Boolean>()
                service.performSwipe(startPt.x, startPt.y, endPt.x, endPt.y, durationMs) { success ->
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
                val err = if (success) null else "Failed to perform global navigation action $commandType"
                journal.saveRecord(commandId, fencingToken, status, err)
                statusPublisher(commandId, status, err, 3)
            }
            else -> {
                val errStr = "Unsupported command_type: $commandType"
                Log.e(TAG, errStr)
                journal.saveRecord(commandId, fencingToken, "failed", errStr)
                statusPublisher(commandId, "failed", errStr, 3)
            }
        }
    }
}
