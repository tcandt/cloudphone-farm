package com.tcandt.cloudphone.agent.media

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat

object MediaCaptureServiceNotifier {
    var onFgsReadyListener: (() -> Unit)? = null
}

class MediaCaptureService : Service() {

    companion object {
        private const val TAG = "MediaCaptureService"
        private const val NOTIFICATION_ID = 2001
        private const val CHANNEL_ID = "pcp_media_capture_channel"

        const val ACTION_START_CAPTURE = "com.tcandt.cloudphone.agent.media.START_CAPTURE"
        const val ACTION_STOP_CAPTURE = "com.tcandt.cloudphone.agent.media.STOP_CAPTURE"
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
                startForegroundServiceNotification()
                Log.i(TAG, "MediaCaptureService started as active mediaProjection Foreground Service (FGS)")
                MediaCaptureServiceNotifier.onFgsReadyListener?.invoke()
            }
            ACTION_STOP_CAPTURE -> {
                stopForeground(true)
                stopSelf()
                Log.i(TAG, "MediaCaptureService stopped Foreground Service (FGS)")
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
                description = "Active MediaProjection WebRTC screen capture service"
            }
            val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            notificationManager.createNotificationChannel(channel)
        }
    }

    private fun startForegroundServiceNotification() {
        val notification: Notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Phone Control Platform Screen Stream")
            .setContentText("Screen capture and WebRTC video streaming active")
            .setSmallIcon(android.R.drawable.ic_menu_camera)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setOngoing(true)
            .build()

        startForeground(NOTIFICATION_ID, notification)
    }

    override fun onDestroy() {
        stopForeground(true)
        super.onDestroy()
    }
}
