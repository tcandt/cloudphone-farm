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

    enum class SessionOutcome { STOPPED, FAILED }

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

    private fun logD(tag: String, msg: String) { try { Log.d(tag, msg) } catch (_: Throwable) {} }
    private fun logI(tag: String, msg: String) { try { Log.i(tag, msg) } catch (_: Throwable) {} }
    private fun logW(tag: String, msg: String) { try { Log.w(tag, msg) } catch (_: Throwable) {} }
    private fun logE(tag: String, msg: String) { try { Log.e(tag, msg) } catch (_: Throwable) {} }

    fun requestConsent(context: Context, sessionId: String): Long {
        if (currentState != ScreenCaptureState.IDLE) {
            logW(TAG, "New requestConsent for SessionID=$sessionId superseding existing state=$currentState (SessionID=$activeSessionId). Resetting state.")
            terminateMediaSession(context, SessionOutcome.STOPPED, "superseded_by_new_session")
        }

        sessionRequestGeneration++
        activeSessionId = sessionId
        currentState = ScreenCaptureState.CONSENT_REQUIRED

        logI(TAG, "Requesting MediaProjection consent for SessionID=$sessionId (Gen=$sessionRequestGeneration)")
        showConsentNotification(context, sessionRequestGeneration)
        return sessionRequestGeneration
    }

    private fun showConsentNotification(context: Context, generation: Long) {
        try {
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

            val notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
            notificationManager?.notify(CONSENT_NOTIFICATION_ID, notification)
        } catch (e: Throwable) {
            logW(TAG, "Notification skipped (JVM test mode): ${e.message}")
        }
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
        try {
            val notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            notificationManager.cancel(CONSENT_NOTIFICATION_ID)
        } catch (e: Exception) {
            logW(TAG, "Failed to dismiss consent notification: ${e.message}")
        }
    }

    fun onConsentGranted(context: Context, generation: Long, resultCode: Int, resultData: Intent) {
        dismissConsentNotification(context)

        if (generation != sessionRequestGeneration || currentState != ScreenCaptureState.CONSENT_REQUIRED) {
            logW(TAG, "Stale or canceled MediaProjection consent received (Gen=$generation, currentGen=$sessionRequestGeneration, state=$currentState). Ignoring.")
            return
        }

        logI(TAG, "Consent granted for Gen=$generation. Registering FGS_READY listener and starting MediaCaptureService for SessionID=$activeSessionId")
        val intentGrant = resultData
        val capturedSessionId = activeSessionId
        val capturedGen = generation

        var mainHandler: Handler? = null
        var timeoutRunnable: Runnable? = null
        try {
            mainHandler = Handler(Looper.getMainLooper())
            timeoutRunnable = Runnable {
                if (MediaCaptureServiceNotifier.onFgsReadyListener != null) {
                    MediaCaptureServiceNotifier.onFgsReadyListener = null
                    logE(TAG, "FGS_READY timeout after 10s for SessionID=$capturedSessionId (Gen=$capturedGen). Stopping capture.")
                    terminateMediaSession(context, SessionOutcome.FAILED, "FGS_READY timeout after 10s")
                }
            }
            mainHandler.postDelayed(timeoutRunnable, 10000L)
        } catch (e: Throwable) {
            logW(TAG, "Main handler initialization skipped (JVM test mode): ${e.message}")
        }

        // TargetSdk 34 Rule: Register FGS_READY listener BEFORE starting foreground service with generation fencing
        MediaCaptureServiceNotifier.onFgsReadyListener = {
            try {
                if (mainHandler != null && timeoutRunnable != null) {
                    mainHandler.removeCallbacks(timeoutRunnable)
                }
            } catch (e: Throwable) {
                // Ignore Handler removal errors in JVM unit tests
            }
            MediaCaptureServiceNotifier.onFgsReadyListener = null

            if (capturedGen != sessionRequestGeneration || activeSessionId != capturedSessionId || currentState != ScreenCaptureState.CONSENT_REQUIRED) {
                logW(TAG, "FGS_READY callback rejected: stale session or generation (Gen=$capturedGen vs currentGen=$sessionRequestGeneration, state=$currentState). Stopping orphan FGS.")
                terminateMediaSession(context, SessionOutcome.STOPPED, "stale_fgs_ready", capturedSessionId)
            } else {
                logI(TAG, "MediaCaptureService FGS_READY signal received. Consuming MediaProjection token for SessionID=$capturedSessionId")
                clearPermissionGrantOnly()
                currentState = ScreenCaptureState.READY
                try {
                    onConsentGrantedHandler?.invoke(capturedSessionId, intentGrant)
                } catch (e: Throwable) {
                    logW(TAG, "onConsentGrantedHandler invocation exception (JVM test mode or WebRTC setup error): ${e.message}")
                }
            }
        }

        try {
            val fgsIntent = Intent(context, MediaCaptureService::class.java).apply {
                action = MediaCaptureService.ACTION_START_CAPTURE
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(fgsIntent)
            } else {
                context.startService(fgsIntent)
            }
        } catch (e: Throwable) {
            logW(TAG, "MediaCaptureService start ignored (running in JVM test environment): ${e.message}")
        }
        isFgsRunning = true
    }

    fun onConsentDenied(context: Context, generation: Long) {
        dismissConsentNotification(context)

        if (generation != sessionRequestGeneration) return

        logW(TAG, "Consent denied by user for Gen=$generation")
        terminateMediaSession(context, SessionOutcome.FAILED, "MediaProjection consent denied by user")
    }

    fun markReady(sessionId: String) {
        if (activeSessionId == sessionId || activeSessionId.isEmpty()) {
            activeSessionId = sessionId
            currentState = ScreenCaptureState.READY
            logI(TAG, "ScreenCaptureState -> READY (SessionID=$sessionId)")
        }
    }

    fun markNegotiating(sessionId: String) {
        if (activeSessionId == sessionId) {
            currentState = ScreenCaptureState.NEGOTIATING
            logI(TAG, "ScreenCaptureState -> NEGOTIATING (SessionID=$sessionId)")
        }
    }

    fun markConnected(sessionId: String) {
        if (activeSessionId == sessionId) {
            currentState = ScreenCaptureState.CONNECTED
            logI(TAG, "ScreenCaptureState -> CONNECTED (SessionID=$sessionId)")
        }
    }

    fun onCapturerStopped() {
        logI(TAG, "Capturer stopped callback received for SessionID=$activeSessionId")
        terminateMediaSession(null, SessionOutcome.STOPPED, "capturer_stopped")
    }

    fun onEncoderFormatConfirmed() {
        if (currentState == ScreenCaptureState.READY || currentState == ScreenCaptureState.NEGOTIATING) {
            currentState = ScreenCaptureState.CAPTURING
            logI(TAG, "Transitioning state -> CAPTURING (SessionID=$activeSessionId)")
            sessionListener?.onSessionStarted(activeSessionId)
        }
    }

    fun onEncoderFailed(errorMsg: String) {
        logE(TAG, "Encoder setup failed: $errorMsg")
        terminateMediaSession(null, SessionOutcome.FAILED, errorMsg)
    }

    fun onProjectionStoppedBySystem(context: Context? = null) {
        logW(TAG, "Projection stopped by system/user for SessionID=$activeSessionId")
        terminateMediaSession(context, SessionOutcome.STOPPED, "system_projection_stopped")
    }

    fun stopCapture(context: Context? = null) {
        terminateMediaSession(context, SessionOutcome.STOPPED, "operator_requested")
    }

    fun terminateMediaSession(context: Context?, outcome: SessionOutcome, reason: String, targetSessionId: String? = null) {
        logI(TAG, "terminateMediaSession called (outcome=$outcome, reason=$reason, targetSessionId=$targetSessionId, activeSessionId=$activeSessionId, isFgsRunning=$isFgsRunning, state=$currentState)")

        val sessId = targetSessionId ?: activeSessionId

        if (targetSessionId == null || targetSessionId == activeSessionId) {
            sessionRequestGeneration++
            MediaCaptureServiceNotifier.onFgsReadyListener = null
        }

        if (context != null) {
            dismissConsentNotification(context)
            if (isFgsRunning) {
                try {
                    val stopIntent = Intent(context, MediaCaptureService::class.java).apply {
                        action = MediaCaptureService.ACTION_STOP_CAPTURE
                    }
                    context.stopService(stopIntent)
                } catch (e: Throwable) {
                    logW(TAG, "Error stopping MediaCaptureService (JVM test mode or service stopped): ${e.message}")
                }
                isFgsRunning = false
                logI(TAG, "MediaCaptureService FGS stopped unconditionally by ScreenCaptureManager")
            }
        } else {
            isFgsRunning = false
        }

        if (currentState == ScreenCaptureState.IDLE && sessId.isEmpty()) {
            return
        }

        val isFailed = (outcome == SessionOutcome.FAILED)
        if (targetSessionId == null || targetSessionId == activeSessionId) {
            clearAllSessionState()
        }

        if (sessId.isNotEmpty()) {
            if (isFailed) {
                sessionListener?.onSessionFailed(sessId, reason)
            } else {
                sessionListener?.onSessionStopped(sessId, reason)
            }
        }
    }

    fun onServiceStoppedFully(sessionId: String, reason: String) {
        val sessId = if (sessionId.isNotEmpty()) sessionId else activeSessionId
        clearAllSessionState()
        sessionListener?.onSessionStopped(sessId, reason)
    }

    private fun clearPermissionGrantOnly() {
        projectionResultCode = 0
        projectionResultData = null
        logD(TAG, "Cleared MediaProjection permission grant Intent (consumed per targetSdk 34 rules)")
    }

    private fun clearAllSessionState() {
        projectionResultCode = 0
        projectionResultData = null
        activeSessionId = ""
        currentState = ScreenCaptureState.IDLE
        onConsentGrantedHandler = null
        logD(TAG, "Cleared all MediaProjection session state and activeSessionId")
    }
}
