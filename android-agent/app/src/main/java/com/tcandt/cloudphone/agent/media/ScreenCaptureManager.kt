package com.tcandt.cloudphone.agent.media

import android.content.Context
import android.content.Intent
import android.util.Log

object ScreenCaptureManager {
    private const val TAG = "ScreenCaptureManager"

    private var projectionResultCode: Int = 0
    private var projectionResultData: Intent? = null
    private var isStreamingActive: Boolean = false

    fun initConsent(resultCode: Int, resultData: Intent) {
        projectionResultCode = resultCode
        projectionResultData = resultData
        Log.i(TAG, "MediaProjection permission consent initialized (ResultCode=$resultCode)")
    }

    fun hasConsent(): Boolean {
        return projectionResultCode != 0 && projectionResultData != null
    }

    fun startCapture(
        context: Context,
        width: Int = 720,
        height: Int = 1280,
        bitrate: Int = 2_500_000,
        fps: Int = 30
    ): Boolean {
        if (!hasConsent()) {
            Log.e(TAG, "Cannot start MediaProjection capture: Consent missing. Request consent via SetupActivity first.")
            return false
        }

        if (isStreamingActive) {
            Log.w(TAG, "MediaProjection capture stream is already active.")
            return true
        }

        val intent = Intent(context, MediaCaptureService::class.java).apply {
            action = MediaCaptureService.ACTION_START_CAPTURE
            putExtra(MediaCaptureService.EXTRA_RESULT_CODE, projectionResultCode)
            putExtra(MediaCaptureService.EXTRA_RESULT_DATA, projectionResultData)
            putExtra(MediaCaptureService.EXTRA_WIDTH, width)
            putExtra(MediaCaptureService.EXTRA_HEIGHT, height)
            putExtra(MediaCaptureService.EXTRA_BITRATE, bitrate)
            putExtra(MediaCaptureService.EXTRA_FPS, fps)
        }

        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
            context.startForegroundService(intent)
        } else {
            context.startService(intent)
        }

        isStreamingActive = true
        Log.i(TAG, "Started MediaProjection H.264 Hardware Capture Service (${width}x${height} @ ${fps}FPS, ${bitrate / 1000}kbps)")
        return true
    }

    fun stopCapture(context: Context) {
        if (!isStreamingActive) return

        val intent = Intent(context, MediaCaptureService::class.java).apply {
            action = MediaCaptureService.ACTION_STOP_CAPTURE
        }
        context.startService(intent)
        isStreamingActive = false
        Log.i(TAG, "Stopped MediaProjection H.264 Hardware Capture Service.")
    }

    fun isStreaming(): Boolean = isStreamingActive
}
