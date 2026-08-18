package com.tcandt.cloudphone.agent

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import com.tcandt.cloudphone.agent.config.AgentConfigStore
import com.tcandt.cloudphone.agent.media.ScreenCaptureManager
import com.tcandt.cloudphone.agent.websocket.AgentWebSocketClient

class AgentService : Service() {

    private var wsClient: AgentWebSocketClient? = null
    private lateinit var configStore: AgentConfigStore

    companion object {
        private const val TAG = "AgentService"
        private const val CHANNEL_ID = "pcp_agent_service_channel"
        private const val NOTIFICATION_ID = 1001

        @Volatile
        var instance: AgentService? = null
            private set
    }

    override fun onCreate() {
        super.onCreate()
        instance = this
        configStore = AgentConfigStore(applicationContext)

        createNotificationChannel()
        startForeground(NOTIFICATION_ID, buildNotification())

        val wssUrl = configStore.getWssUrl()
        val agentId = configStore.getAgentId()

        if (agentId.isNotEmpty()) {
            wsClient = AgentWebSocketClient(applicationContext, wssUrl, agentId)
            wsClient?.connect()
            Log.i(TAG, "AgentService started and connecting to $wssUrl for AgentID=$agentId")
        } else {
            Log.w(TAG, "AgentService started but device is not yet enrolled. Open SetupActivity to enroll.")
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        super.onDestroy()
        if (instance == this) {
            instance = null
        }
        wsClient?.disconnect()
        Log.i(TAG, "AgentService destroyed")
    }

    fun decommission(context: android.content.Context) {
        try {
            wsClient?.decommissionAndDisconnect()
            wsClient = null
            ScreenCaptureManager.stopCapture(context)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                stopForeground(STOP_FOREGROUND_REMOVE)
            } else {
                @Suppress("DEPRECATION")
                stopForeground(true)
            }
            stopSelf()
            Log.i(TAG, "AgentService decommission completed and service stopped")
        } catch (e: Throwable) {
            Log.e(TAG, "Error during decommission: ${e.message}", e)
        }
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Phone Control Agent Service",
                NotificationManager.IMPORTANCE_LOW
            )
            val manager = getSystemService(NotificationManager::class.java)
            manager?.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(): Notification {
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Phone Control Agent Active")
            .setContentText("Listening for remote physical device commands...")
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
    }
}
