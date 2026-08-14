package com.tcandt.cloudphone.agent.media

import android.content.Context
import android.content.Intent
import android.util.Log

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

    private var currentState: ScreenCaptureState = ScreenCaptureState.IDLE
    private var activeSessionId: String = ""

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

        activeSessionId = sessionId
        pendingWidth = width
        pendingHeight = height
        pendingBitrate = bitrate
        pendingFps = fps

        if (projectionResultCode == 0 || projectionResultData == null) {
            Log.i(TAG, "MediaProjection token missing. Transitioning state IDLE -> CONSENT_REQUIRED")
            currentState = ScreenCaptureState.CONSENT_REQUIRED

            // Launch on-demand consent prompt activity
            val intent = Intent(context, ConsentPromptActivity::class.java).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(intent)
        } else {
            startServiceInternal(context)
        }
    }

    fun onConsentGranted(context: Context, resultCode: Int, resultData: Intent) {
        Log.i(TAG, "Consent granted. Received MediaProjection token result")
        projectionResultCode = resultCode
        projectionResultData = resultData
        startServiceInternal(context)
    }

    fun onConsentDenied() {
        Log.w(TAG, "Consent denied by user for MediaProjection")
        currentState = ScreenCaptureState.FAILED
        sessionListener?.onSessionFailed(activeSessionId, "MediaProjection consent denied by user")
        clearToken()
        currentState = ScreenCaptureState.IDLE
    }

    private fun startServiceInternal(context: Context) {
        currentState = ScreenCaptureState.STARTING
        Log.i(TAG, "Transitioning state -> STARTING (SessionID=$activeSessionId)")

        val intent = Intent(context, MediaCaptureService::class.java).apply {
            action = MediaCaptureService.ACTION_START_CAPTURE
            putExtra(MediaCaptureService.EXTRA_RESULT_CODE, projectionResultCode)
            putExtra(MediaCaptureService.EXTRA_RESULT_DATA, projectionResultData)
            putExtra(MediaCaptureService.EXTRA_WIDTH, pendingWidth)
            putExtra(MediaCaptureService.EXTRA_HEIGHT, pendingHeight)
            putExtra(MediaCaptureService.EXTRA_BITRATE, pendingBitrate)
            putExtra(MediaCaptureService.EXTRA_FPS, pendingFps)
        }

        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
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
        sessionListener?.onSessionStopped(activeSessionId, "projection_stopped_by_system")
        clearToken()
        currentState = ScreenCaptureState.IDLE
    }

    fun stopCapture(context: Context) {
        if (currentState == ScreenCaptureState.IDLE) return

        currentState = ScreenCaptureState.STOPPING
        Log.i(TAG, "Stopping capture session (SessionID=$activeSessionId)")

        val intent = Intent(context, MediaCaptureService::class.java).apply {
            action = MediaCaptureService.ACTION_STOP_CAPTURE
        }
        context.startService(intent)

        sessionListener?.onSessionStopped(activeSessionId, "operator_requested")
        clearToken()
        currentState = ScreenCaptureState.IDLE
    }

    private fun clearToken() {
        projectionResultCode = 0
        projectionResultData = null
        activeSessionId = ""
        Log.d(TAG, "Cleared MediaProjection token (consumed per Android 14 targetSdk 34 rules)")
    }
}
