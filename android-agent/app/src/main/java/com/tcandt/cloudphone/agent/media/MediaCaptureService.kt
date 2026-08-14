package com.tcandt.cloudphone.agent.media

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.MediaCodec
import android.media.MediaCodecInfo
import android.media.MediaFormat
import android.media.projection.MediaProjection
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

class MediaCaptureService : Service() {

    private var mediaProjection: MediaProjection? = null
    private var virtualDisplay: VirtualDisplay? = null
    private var mediaCodec: MediaCodec? = null
    private var encoderThreadJob: Job? = null
    private var isEncoderRunning = false

    companion object {
        private const val TAG = "MediaCaptureService"
        private const val NOTIFICATION_ID = 2001
        private const val CHANNEL_ID = "pcp_media_capture_channel"

        const val ACTION_START_CAPTURE = "com.tcandt.cloudphone.agent.media.START_CAPTURE"
        const val ACTION_STOP_CAPTURE = "com.tcandt.cloudphone.agent.media.STOP_CAPTURE"

        const val EXTRA_RESULT_CODE = "extra_result_code"
        const val EXTRA_RESULT_DATA = "extra_result_data"
        const val EXTRA_WIDTH = "extra_width"
        const val EXTRA_HEIGHT = "extra_height"
        const val EXTRA_BITRATE = "extra_bitrate"
        const val EXTRA_FPS = "extra_fps"
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent == null) return START_NOT_STICKY

        when (intent.action) {
            ACTION_START_CAPTURE -> {
                val resultCode = intent.getIntExtra(EXTRA_RESULT_CODE, 0)
                val resultData = intent.getParcelableExtra<Intent>(EXTRA_RESULT_DATA)
                val width = intent.getIntExtra(EXTRA_WIDTH, 720)
                val height = intent.getIntExtra(EXTRA_HEIGHT, 1280)
                val bitrate = intent.getIntExtra(EXTRA_BITRATE, 2_500_000)
                val fps = intent.getIntExtra(EXTRA_FPS, 30)

                startForegroundServiceNotification()
                if (resultCode != 0 && resultData != null) {
                    startMediaProjectionEncoder(resultCode, resultData, width, height, bitrate, fps)
                } else {
                    Log.e(TAG, "Invalid result code or result data for MediaProjection")
                }
            }
            ACTION_STOP_CAPTURE -> {
                stopMediaProjectionEncoder()
                stopForeground(true)
                stopSelf()
            }
        }

        return START_STICKY
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Phone Control Platform Screen Capture",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Active MediaProjection H.264 screen capture service"
            }
            val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            notificationManager.createNotificationChannel(channel)
        }
    }

    private fun startForegroundServiceNotification() {
        val notification: Notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Phone Control Platform Screen Stream")
            .setContentText("Screen capture and hardware H.264 encoding active")
            .setSmallIcon(android.R.drawable.ic_menu_camera)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setOngoing(true)
            .build()

        startForeground(NOTIFICATION_ID, notification)
    }

    private fun startMediaProjectionEncoder(
        resultCode: Int,
        resultData: Intent,
        width: Int,
        height: Int,
        bitrate: Int,
        fps: Int
    ) {
        try {
            val projectionManager = getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
            mediaProjection = projectionManager.getMediaProjection(resultCode, resultData)

            // Setup MediaCodec H.264 AVC Encoder
            val format = MediaFormat.createVideoFormat(MediaFormat.MIMETYPE_VIDEO_AVC, width, height).apply {
                setInteger(MediaFormat.KEY_COLOR_FORMAT, MediaCodecInfo.CodecCapabilities.COLOR_FormatSurface)
                setInteger(MediaFormat.KEY_BIT_RATE, bitrate)
                setInteger(MediaFormat.KEY_FRAME_RATE, fps)
                setInteger(MediaFormat.KEY_I_FRAME_INTERVAL, 2) // 2-second keyframe interval
            }

            mediaCodec = MediaCodec.createEncoderByType(MediaFormat.MIMETYPE_VIDEO_AVC).apply {
                configure(format, null, null, MediaCodec.CONFIGURE_FLAG_ENCODE)
            }

            val inputSurface = mediaCodec!!.createInputSurface()
            mediaCodec!!.start()

            // Create VirtualDisplay feeding into MediaCodec Surface
            val dpi = resources.displayMetrics.densityDpi
            virtualDisplay = mediaProjection!!.createVirtualDisplay(
                "PCP_VirtualDisplay",
                width,
                height,
                dpi,
                DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
                inputSurface,
                null,
                null
            )

            isEncoderRunning = true
            startEncoderLoop()

            Log.i(TAG, "MediaCodec H.264 Encoder & VirtualDisplay created successfully (${width}x${height})")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start MediaProjection H.264 encoder: ${e.message}", e)
            stopMediaProjectionEncoder()
        }
    }

    private fun startEncoderLoop() {
        encoderThreadJob = CoroutineScope(Dispatchers.IO).launch {
            val bufferInfo = MediaCodec.BufferInfo()
            val codec = mediaCodec ?: return@launch

            while (isEncoderRunning) {
                try {
                    val outputBufferId = codec.dequeueOutputBuffer(bufferInfo, 10_000L) // 10ms timeout
                    if (outputBufferId >= 0) {
                        val outputBuffer = codec.getOutputBuffer(outputBufferId)
                        if (outputBuffer != null && bufferInfo.size > 0) {
                            outputBuffer.position(bufferInfo.offset)
                            outputBuffer.limit(bufferInfo.offset + bufferInfo.size)

                            // NAL unit ready: Ready for WebRTC video track packetization
                            // (In Phase 1.3.2 this feeds directly into WebRTC VideoTrack / PeerConnection)
                        }
                        codec.releaseOutputBuffer(outputBufferId, false)
                    } else if (outputBufferId == MediaCodec.INFO_OUTPUT_FORMAT_CHANGED) {
                        val newFormat = codec.outputFormat
                        Log.i(TAG, "MediaCodec output format changed: $newFormat")
                    }
                } catch (e: Exception) {
                    if (isEncoderRunning) {
                        Log.e(TAG, "Error in MediaCodec output buffer loop: ${e.message}")
                    }
                }
            }
        }
    }

    private fun stopMediaProjectionEncoder() {
        isEncoderRunning = false
        encoderThreadJob?.cancel()
        encoderThreadJob = null

        try {
            virtualDisplay?.release()
            virtualDisplay = null

            mediaCodec?.stop()
            mediaCodec?.release()
            mediaCodec = null

            mediaProjection?.stop()
            mediaProjection = null

            Log.i(TAG, "Released MediaProjection, VirtualDisplay and MediaCodec hardware encoder")
        } catch (e: Exception) {
            Log.e(TAG, "Error releasing MediaProjection resources: ${e.message}")
        }
    }

    override fun onDestroy() {
        stopMediaProjectionEncoder()
        super.onDestroy()
    }
}
