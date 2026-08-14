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
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log
import android.view.Surface
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

class MediaCaptureService : Service() {

    private var mediaProjection: MediaProjection? = null
    private var virtualDisplay: VirtualDisplay? = null
    private var mediaCodec: MediaCodec? = null
    private var encoderInputSurface: Surface? = null
    private var encoderThreadJob: Job? = null
    private var isEncoderRunning = false

    private var currentWidth = 720
    private var currentHeight = 1280

    companion object {
        private const val TAG = "MediaCaptureService"
        private const val NOTIFICATION_ID = 2001
        private const val CHANNEL_ID = "pcp_media_capture_channel"

        const val ACTION_START_CAPTURE = "com.tcandt.cloudphone.agent.media.START_CAPTURE"
        const val ACTION_STOP_CAPTURE = "com.tcandt.cloudphone.agent.media.STOP_CAPTURE"
        const val ACTION_UPDATE_RESOLUTION = "com.tcandt.cloudphone.agent.media.UPDATE_RESOLUTION"

        const val EXTRA_RESULT_CODE = "extra_result_code"
        const val EXTRA_RESULT_DATA = "extra_result_data"
        const val EXTRA_WIDTH = "extra_width"
        const val EXTRA_HEIGHT = "extra_height"
        const val EXTRA_BITRATE = "extra_bitrate"
        const val EXTRA_FPS = "extra_fps"

        var videoSink: EncodedVideoSink? = null
    }

    private val projectionCallback = object : MediaProjection.Callback() {
        override fun onStop() {
            Log.i(TAG, "MediaProjection.Callback.onStop triggered (user/system stopped sharing)")
            stopMediaProjectionEncoder(stopProjection = false)
            ScreenCaptureManager.onProjectionStoppedBySystem()
            stopForeground(true)
            stopSelf()
        }
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
                    ScreenCaptureManager.onEncoderFailed("Invalid MediaProjection token result")
                }
            }
            ACTION_UPDATE_RESOLUTION -> {
                val newWidth = intent.getIntExtra(EXTRA_WIDTH, currentWidth)
                val newHeight = intent.getIntExtra(EXTRA_HEIGHT, currentHeight)
                updateVirtualDisplayResolution(newWidth, newHeight)
            }
            ACTION_STOP_CAPTURE -> {
                stopMediaProjectionEncoder(stopProjection = true)
                stopForeground(true)
                stopSelf()
                ScreenCaptureManager.onServiceStoppedFully("", "operator_requested")
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
            currentWidth = width
            currentHeight = height

            val projectionManager = getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
            mediaProjection = projectionManager.getMediaProjection(resultCode, resultData)

            if (mediaProjection == null) {
                Log.e(TAG, "MediaProjectionManager returned null projection")
                ScreenCaptureManager.onEncoderFailed("MediaProjection token returned null")
                return
            }

            // MANDATORY for Android 14 targetSdk 34: Register callback BEFORE createVirtualDisplay
            mediaProjection?.registerCallback(projectionCallback, Handler(Looper.getMainLooper()))
            Log.i(TAG, "Registered MediaProjection.Callback successfully")

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

            encoderInputSurface = mediaCodec!!.createInputSurface()
            mediaCodec!!.start()

            // Create VirtualDisplay feeding into MediaCodec Surface
            val dpi = resources.displayMetrics.densityDpi
            virtualDisplay = mediaProjection!!.createVirtualDisplay(
                "PCP_VirtualDisplay",
                width,
                height,
                dpi,
                DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
                encoderInputSurface,
                null,
                null
            )

            isEncoderRunning = true
            startEncoderLoop()

            Log.i(TAG, "MediaCodec H.264 Encoder & VirtualDisplay created successfully (${width}x${height})")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start MediaProjection H.264 encoder: ${e.message}", e)
            ScreenCaptureManager.onEncoderFailed(e.message ?: "MediaCodec setup failed")
            stopMediaProjectionEncoder(stopProjection = true)
        }
    }

    private fun updateVirtualDisplayResolution(newWidth: Int, newHeight: Int) {
        if (virtualDisplay != null && (currentWidth != newWidth || currentHeight != newHeight)) {
            currentWidth = newWidth
            currentHeight = newHeight
            val dpi = resources.displayMetrics.densityDpi
            virtualDisplay?.resize(newWidth, newHeight, dpi)
            Log.i(TAG, "Resized VirtualDisplay dynamically to ${newWidth}x${newHeight} without recreating MediaProjection")
        }
    }

    private fun startEncoderLoop() {
        encoderThreadJob = CoroutineScope(Dispatchers.IO).launch {
            val bufferInfo = MediaCodec.BufferInfo()
            val codec = mediaCodec ?: return@launch
            var hasOutputFormat = false
            var hasKeyFrame = false

            while (isEncoderRunning) {
                try {
                    val outputBufferId = codec.dequeueOutputBuffer(bufferInfo, 10_000L) // 10ms timeout
                    if (outputBufferId >= 0) {
                        val outputBuffer = codec.getOutputBuffer(outputBufferId)
                        if (outputBuffer != null && bufferInfo.size > 0) {
                            outputBuffer.position(bufferInfo.offset)
                            outputBuffer.limit(bufferInfo.offset + bufferInfo.size)

                            // Copy byte array before releasing output buffer
                            val frameData = ByteArray(bufferInfo.size)
                            outputBuffer.get(frameData)

                            val isKeyFrame = (bufferInfo.flags and MediaCodec.BUFFER_FLAG_KEY_FRAME) != 0
                            val isCodecConfig = (bufferInfo.flags and MediaCodec.BUFFER_FLAG_CODEC_CONFIG) != 0

                            if (isKeyFrame) {
                                hasKeyFrame = true
                            }

                            val frame = EncodedVideoFrame(
                                data = frameData,
                                ptsUs = bufferInfo.presentationTimeUs,
                                isKeyFrame = isKeyFrame,
                                isCodecConfig = isCodecConfig
                            )

                            // Truthful frame readiness: Trigger CAPTURING state only when BOTH OutputFormat AND KeyFrame are produced
                            if (hasOutputFormat && hasKeyFrame) {
                                ScreenCaptureManager.onEncoderFormatConfirmed()
                            }

                            videoSink?.onEncodedFrame(frame)
                        }
                        codec.releaseOutputBuffer(outputBufferId, false)
                    } else if (outputBufferId == MediaCodec.INFO_OUTPUT_FORMAT_CHANGED) {
                        val newFormat = codec.outputFormat
                        hasOutputFormat = true
                        Log.i(TAG, "MediaCodec output format changed: $newFormat")
                        videoSink?.onFormatChanged(newFormat)
                    }
                } catch (e: Exception) {
                    if (isEncoderRunning) {
                        Log.e(TAG, "Error in MediaCodec output buffer loop: ${e.message}")
                    }
                }
            }
        }
    }

    private fun stopMediaProjectionEncoder(stopProjection: Boolean) {
        isEncoderRunning = false
        encoderThreadJob?.cancel()
        encoderThreadJob = null

        try {
            virtualDisplay?.release()
            virtualDisplay = null

            encoderInputSurface?.release()
            encoderInputSurface = null

            mediaCodec?.stop()
            mediaCodec?.release()
            mediaCodec = null

            if (stopProjection && mediaProjection != null) {
                mediaProjection?.unregisterCallback(projectionCallback)
                mediaProjection?.stop()
                mediaProjection = null
            }

            Log.i(TAG, "Released MediaProjection, VirtualDisplay, encoderInputSurface and MediaCodec hardware encoder")
        } catch (e: Exception) {
            Log.e(TAG, "Error releasing MediaProjection resources: ${e.message}")
        }
    }

    override fun onDestroy() {
        stopMediaProjectionEncoder(stopProjection = true)
        super.onDestroy()
    }
}
