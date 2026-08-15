package com.tcandt.cloudphone.agent.media

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.util.Log

enum class ScreenCaptureState {
    IDLE,
    CONSENT_REQUIRED,
    READY,
    NEGOTIATING,
    CONNECTED,
    CAPTURING,
    STOPPING,
    FAILED
}

object ScreenCaptureManager {
    private const val TAG = "ScreenCaptureManager"
    private const val CONSENT_NOTIFICATION_ID = 2002
    private const val CHANNEL_ID = "pcp_consent_channel"

    var currentState: ScreenCaptureState = ScreenCaptureState.IDLE
        private set

    var activeSessionId: String = ""
        private set

    var sessionRequestGeneration: Long = 0L
        private set

    var isFgsRunning: Boolean = false

    private var projectionResultCode: Int = 0
    private var projectionResultData: Intent? = null

    interface SessionStateListener {
        fun onSessionStarted(sessionId: String)
        fun onSessionStopped(sessionId: String, reason: String)
        fun onSessionFailed(sessionId: String, error: String)
    }

    var sessionListener: SessionStateListener? = null
    var onConsentGrantedHandler: ((sessionId: String, projectionIntent: Intent) -> Unit)? = null

    fun requestConsent(context: Context, sessionId: String): Long {
        if (currentState != ScreenCaptureState.IDLE) {
            Log.w(TAG, "New requestConsent for SessionID=$sessionId superseding existing state=$currentState (SessionID=$activeSessionId). Resetting state.")
            stopCapture(context)
        }

        sessionRequestGeneration++
        activeSessionId = sessionId
        currentState = ScreenCaptureState.CONSENT_REQUIRED

        Log.i(TAG, "Requesting MediaProjection consent for SessionID=$sessionId (Gen=$sessionRequestGeneration)")
        showConsentNotification(context, sessionRequestGeneration)
        return sessionRequestGeneration
    }

    private fun showConsentNotification(context: Context, generation: Long) {
        createNotificationChannel(context)

        val intent = Intent(context, ConsentPromptActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra("generation", generation)
            putExtra("session_id", activeSessionId)
        }

        val pendingIntent = android.app.PendingIntent.getActivity(
            context,
            generation.toInt(),
            intent,
            android.app.PendingIntent.FLAG_UPDATE_CURRENT or android.app.PendingIntent.FLAG_IMMUTABLE
        )

        val notification = androidx.core.app.NotificationCompat.Builder(context, CHANNEL_ID)
            .setContentTitle("Screen Capture Requested")
            .setContentText("Tap to grant MediaProjection permission for WebRTC remote view")
            .setSmallIcon(android.R.drawable.ic_menu_camera)
            .setPriority(androidx.core.app.NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()

        val notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.notify(CONSENT_NOTIFICATION_ID, notification)
    }

    private fun createNotificationChannel(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Phone Control Platform Consent",
                NotificationManager.IMPORTANCE_HIGH
            ).apply {
                description = "Notifications for user MediaProjection consent prompt"
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

        if (generation != sessionRequestGeneration || currentState != ScreenCaptureState.CONSENT_REQUIRED) {
            Log.w(TAG, "Stale or canceled MediaProjection consent received (Gen=$generation, currentGen=$sessionRequestGeneration, state=$currentState). Ignoring.")
            return
        }

        Log.i(TAG, "Consent granted for Gen=$generation. Registering FGS_READY listener and starting MediaCaptureService for SessionID=$activeSessionId")
        val intentGrant = resultData
        val capturedSessionId = activeSessionId
        val capturedGen = generation

        val mainHandler = Handler(Looper.getMainLooper())
        val timeoutRunnable = Runnable {
            if (MediaCaptureServiceNotifier.onFgsReadyListener != null) {
                MediaCaptureServiceNotifier.onFgsReadyListener = null
                Log.e(TAG, "FGS_READY timeout after 10s for SessionID=$capturedSessionId (Gen=$capturedGen). Stopping capture.")
                stopCapture(context)
                sessionListener?.onSessionFailed(capturedSessionId, "FGS_READY timeout after 10s")
            }
        }
        mainHandler.postDelayed(timeoutRunnable, 10000L)

        // TargetSdk 34 Rule: Register FGS_READY listener BEFORE starting foreground service with generation fencing
        MediaCaptureServiceNotifier.onFgsReadyListener = {
            mainHandler.removeCallbacks(timeoutRunnable)
            MediaCaptureServiceNotifier.onFgsReadyListener = null

            if (capturedGen != sessionRequestGeneration || activeSessionId != capturedSessionId || currentState != ScreenCaptureState.CONSENT_REQUIRED) {
                Log.w(TAG, "FGS_READY callback rejected: stale session or generation (Gen=$capturedGen vs currentGen=$sessionRequestGeneration, state=$currentState). Stopping orphan FGS.")
                stopCapture(context)
            } else {
                Log.i(TAG, "MediaCaptureService FGS_READY signal received. Consuming MediaProjection token for SessionID=$capturedSessionId")
                clearPermissionGrantOnly()
                currentState = ScreenCaptureState.READY
                onConsentGrantedHandler?.invoke(capturedSessionId, intentGrant)
            }
        }

        val fgsIntent = Intent(context, MediaCaptureService::class.java).apply {
            action = MediaCaptureService.ACTION_START_CAPTURE
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(fgsIntent)
        } else {
            context.startService(fgsIntent)
        }
        isFgsRunning = true
    }

    fun onConsentDenied(context: Context, generation: Long) {
        dismissConsentNotification(context)

        if (generation != sessionRequestGeneration) return

        Log.w(TAG, "Consent denied by user for Gen=$generation")
        currentState = ScreenCaptureState.FAILED
        sessionListener?.onSessionFailed(activeSessionId, "MediaProjection consent denied by user")
        clearAllSessionState()
    }

    fun markReady(sessionId: String) {
        if (activeSessionId == sessionId || activeSessionId.isEmpty()) {
            activeSessionId = sessionId
            currentState = ScreenCaptureState.READY
            Log.i(TAG, "ScreenCaptureState -> READY (SessionID=$sessionId)")
        }
    }

    fun markNegotiating(sessionId: String) {
        if (activeSessionId == sessionId) {
            currentState = ScreenCaptureState.NEGOTIATING
            Log.i(TAG, "ScreenCaptureState -> NEGOTIATING (SessionID=$sessionId)")
        }
    }

    fun markConnected(sessionId: String) {
        if (activeSessionId == sessionId) {
            currentState = ScreenCaptureState.CONNECTED
            Log.i(TAG, "ScreenCaptureState -> CONNECTED (SessionID=$sessionId)")
            sessionListener?.onSessionStarted(sessionId)
        }
    }

    fun onEncoderFormatConfirmed() {
        if (currentState == ScreenCaptureState.READY || currentState == ScreenCaptureState.NEGOTIATING) {
            currentState = ScreenCaptureState.CAPTURING
            Log.i(TAG, "Transitioning state -> CAPTURING (SessionID=$activeSessionId)")
            sessionListener?.onSessionStarted(activeSessionId)
        }
    }

    fun onEncoderFailed(errorMsg: String) {
        Log.e(TAG, "Encoder setup failed: $errorMsg")
        currentState = ScreenCaptureState.FAILED
        sessionListener?.onSessionFailed(activeSessionId, errorMsg)
        clearAllSessionState()
    }

    fun onProjectionStoppedBySystem(context: Context) {
        Log.w(TAG, "Projection stopped by system/user for SessionID=$activeSessionId")
        stopCapture(context)
    }

    fun stopCapture(context: Context) {
        Log.i(TAG, "stopCapture called (SessionID=$activeSessionId, isFgsRunning=$isFgsRunning, state=$currentState)")

        // Invalidate generation and clear pending listener
        sessionRequestGeneration++
        MediaCaptureServiceNotifier.onFgsReadyListener = null
        dismissConsentNotification(context)

        // UNCONDITIONAL FGS CLEANUP: Always stop MediaCaptureService if it was ever started
        if (isFgsRunning) {
            val stopIntent = Intent(context, MediaCaptureService::class.java).apply {
                action = MediaCaptureService.ACTION_STOP_CAPTURE
            }
            context.stopService(stopIntent)
            isFgsRunning = false
            Log.i(TAG, "MediaCaptureService FGS stopped unconditionally by ScreenCaptureManager")
        }

        if (currentState == ScreenCaptureState.IDLE) return

        currentState = ScreenCaptureState.STOPPING
        val sessId = activeSessionId
        clearAllSessionState()
        sessionListener?.onSessionStopped(sessId, "operator_requested")
    }

    fun onServiceStoppedFully(sessionId: String, reason: String) {
        val sessId = if (sessionId.isNotEmpty()) sessionId else activeSessionId
        clearAllSessionState()
        sessionListener?.onSessionStopped(sessId, reason)
    }

    private fun clearPermissionGrantOnly() {
        projectionResultCode = 0
        projectionResultData = null
        Log.d(TAG, "Cleared MediaProjection permission grant Intent (consumed per targetSdk 34 rules)")
    }

    private fun clearAllSessionState() {
        projectionResultCode = 0
        projectionResultData = null
        activeSessionId = ""
        currentState = ScreenCaptureState.IDLE
        Log.d(TAG, "Cleared all MediaProjection session state and activeSessionId")
    }
}
