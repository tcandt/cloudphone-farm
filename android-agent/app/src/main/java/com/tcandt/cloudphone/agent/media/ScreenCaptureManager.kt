package com.tcandt.cloudphone.agent.media

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat

enum class ScreenCaptureState {
    IDLE,
    CONSENT_REQUIRED,
    STARTING,
    CAPTURING,
    STOPPING,
    FAILED
}

object ScreenCaptureManager {
    private const val TAG = "ScreenCaptureManager"
    private const val CONSENT_NOTIFICATION_ID = 2002
    private const val CONSENT_CHANNEL_ID = "pcp_consent_channel"

    private var currentState: ScreenCaptureState = ScreenCaptureState.IDLE
    private var activeSessionId: String = ""
    private var sessionRequestGeneration: Long = 0L

    private var projectionResultCode: Int = 0
    private var projectionResultData: Intent? = null

    private var pendingWidth: Int = 720
    private var pendingHeight: Int = 1280
    private var pendingBitrate: Int = 2_500_000
    private var pendingFps: Int = 30

    var sessionListener: SessionStateListener? = null

    interface SessionStateListener {
        fun onSessionStarted(sessionId: String)
        fun onSessionStopped(sessionId: String, reason: String)
        fun onSessionFailed(sessionId: String, error: String)
    }

    fun getState(): ScreenCaptureState = currentState
    fun getActiveSessionId(): String = activeSessionId
    fun getRequestGeneration(): Long = sessionRequestGeneration

    fun requestCapture(
        context: Context,
        sessionId: String,
        width: Int = 720,
        height: Int = 1280,
        bitrate: Int = 2_500_000,
        fps: Int = 30
    ) {
        if (currentState == ScreenCaptureState.CAPTURING && activeSessionId == sessionId) {
            Log.i(TAG, "Screen capture already active for SessionID=$sessionId")
            sessionListener?.onSessionStarted(sessionId)
            return
        }

        // If in CONSENT_REQUIRED state for same session, retain request generation without duplicate notifications
        if (currentState == ScreenCaptureState.CONSENT_REQUIRED && activeSessionId == sessionId) {
            Log.i(TAG, "Screen capture consent prompt already pending for SessionID=$sessionId")
            return
        }

        // Check Notification Permission on Android 13+ (API 33+)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            !NotificationManagerCompat.from(context).areNotificationsEnabled()
        ) {
            Log.w(TAG, "Notification permission denied on Android 13+. Failing capture request for SessionID=$sessionId")
            currentState = ScreenCaptureState.FAILED
            sessionListener?.onSessionFailed(sessionId, "notification_permission_required")
            clearToken()
            currentState = ScreenCaptureState.IDLE
            return
        }

        // New request generation guard
        sessionRequestGeneration++
        activeSessionId = sessionId
        pendingWidth = width
        pendingHeight = height
        pendingBitrate = bitrate
        pendingFps = fps

        val currentGen = sessionRequestGeneration

        if (projectionResultCode == 0 || projectionResultData == null) {
            Log.i(TAG, "MediaProjection token missing. Transitioning state IDLE -> CONSENT_REQUIRED (Gen=$currentGen)")
            currentState = ScreenCaptureState.CONSENT_REQUIRED

            // Send notification for Android 10+ Background Activity Launch (BAL) compliance
            postConsentNotification(context, currentGen)
        } else {
            startServiceInternal(context, currentGen)
        }
    }

    private fun postConsentNotification(context: Context, generation: Long) {
        createConsentNotificationChannel(context)

        val intent = Intent(context, ConsentPromptActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra("extra_generation", generation)
        }

        val pendingIntent = PendingIntent.getActivity(
            context,
            generation.toInt(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val notification = NotificationCompat.Builder(context, CONSENT_CHANNEL_ID)
            .setContentTitle("Request Remote Screen Stream")
            .setContentText("Tap to allow Phone Control Platform screen capture")
            .setSmallIcon(android.R.drawable.ic_menu_camera)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setContentIntent(pendingIntent)
            .setAutoCancel(true)
            .build()

        val notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(CONSENT_NOTIFICATION_ID, notification)
        Log.i(TAG, "Posted MediaProjection user consent notification (Gen=$generation)")
    }

    private fun createConsentNotificationChannel(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CONSENT_CHANNEL_ID,
                "Screen Capture Authorization Requests",
                NotificationManager.IMPORTANCE_HIGH
            ).apply {
                description = "Notifications prompting user authorization for remote screen streaming"
            }
            val notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            notificationManager.createNotificationChannel(channel)
        }
    }

    private fun dismissConsentNotification(context: Context) {
        val notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.cancel(CONSENT_NOTIFICATION_ID)
    }

    fun onConsentGranted(context: Context, generation: Long, resultCode: Int, resultData: Intent) {
        dismissConsentNotification(context)

        // Session Request Generation Guard: Reject stale consent if canceled or superseded
        if (generation != sessionRequestGeneration || currentState != ScreenCaptureState.CONSENT_REQUIRED) {
            Log.w(TAG, "Stale or canceled MediaProjection consent received (Gen=$generation, currentGen=$sessionRequestGeneration, state=$currentState). Ignoring.")
            return
        }

        Log.i(TAG, "Consent granted for Gen=$generation. Storing single-use MediaProjection token result")
        projectionResultCode = resultCode
        projectionResultData = resultData
        startServiceInternal(context, generation)
    }

    fun onConsentDenied(context: Context, generation: Long) {
        dismissConsentNotification(context)

        if (generation != sessionRequestGeneration) return

        Log.w(TAG, "Consent denied by user for Gen=$generation")
        currentState = ScreenCaptureState.FAILED
        sessionListener?.onSessionFailed(activeSessionId, "MediaProjection consent denied by user")
        clearToken()
        currentState = ScreenCaptureState.IDLE
    }

    private fun startServiceInternal(context: Context, generation: Long) {
        if (generation != sessionRequestGeneration) return

        currentState = ScreenCaptureState.STARTING
        Log.i(TAG, "Transitioning state -> STARTING (SessionID=$activeSessionId, Gen=$generation)")

        val intent = Intent(context, MediaCaptureService::class.java).apply {
            action = MediaCaptureService.ACTION_START_CAPTURE
            putExtra(MediaCaptureService.EXTRA_RESULT_CODE, projectionResultCode)
            putExtra(MediaCaptureService.EXTRA_RESULT_DATA, projectionResultData)
            putExtra(MediaCaptureService.EXTRA_WIDTH, pendingWidth)
            putExtra(MediaCaptureService.EXTRA_HEIGHT, pendingHeight)
            putExtra(MediaCaptureService.EXTRA_BITRATE, pendingBitrate)
            putExtra(MediaCaptureService.EXTRA_FPS, pendingFps)
        }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent)
        } else {
            context.startService(intent)
        }
    }

    fun onEncoderFormatConfirmed() {
        if (currentState == ScreenCaptureState.STARTING) {
            currentState = ScreenCaptureState.CAPTURING
            Log.i(TAG, "Transitioning state -> CAPTURING (AVC Format & KeyFrame output confirmed for SessionID=$activeSessionId)")
            sessionListener?.onSessionStarted(activeSessionId)
        }
    }

    fun onEncoderFailed(errorMsg: String) {
        Log.e(TAG, "Encoder setup failed: $errorMsg")
        currentState = ScreenCaptureState.FAILED
        sessionListener?.onSessionFailed(activeSessionId, errorMsg)
        clearToken()
        currentState = ScreenCaptureState.IDLE
    }

    fun onProjectionStoppedBySystem() {
        Log.w(TAG, "Projection stopped by system/user")
        currentState = ScreenCaptureState.STOPPING
        val sessId = activeSessionId
        clearToken()
        currentState = ScreenCaptureState.IDLE
        sessionListener?.onSessionStopped(sessId, "projection_stopped_by_system")
    }

    fun stopCapture(context: Context) {
        if (currentState == ScreenCaptureState.IDLE) return

        // Invalidate generation to cancel any open consent dialogs
        sessionRequestGeneration++
        dismissConsentNotification(context)

        currentState = ScreenCaptureState.STOPPING
        Log.i(TAG, "Stopping capture session (SessionID=$activeSessionId, invalidated Gen=$sessionRequestGeneration)")

        val intent = Intent(context, MediaCaptureService::class.java).apply {
            action = MediaCaptureService.ACTION_STOP_CAPTURE
        }
        context.startService(intent)
    }

    fun onServiceStoppedFully(sessionId: String, reason: String) {
        if (currentState == ScreenCaptureState.STOPPING || currentState == ScreenCaptureState.CAPTURING) {
            Log.i(TAG, "MediaCaptureService fully released all hardware resources for SessionID=$sessionId. Confirming IDLE state.")
            val sessId = if (sessionId.isNotEmpty()) sessionId else activeSessionId
            clearToken()
            currentState = ScreenCaptureState.IDLE
            sessionListener?.onSessionStopped(sessId, reason)
        }
    }

    private fun clearToken() {
        projectionResultCode = 0
        projectionResultData = null
        activeSessionId = ""
        Log.d(TAG, "Cleared MediaProjection token (consumed per Android 14 targetSdk 34 rules)")
    }
}
