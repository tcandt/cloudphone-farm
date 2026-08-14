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
import com.tcandt.cloudphone.agent.websocket.AgentWebSocketClient

class AgentService : Service() {

    private var wsClient: AgentWebSocketClient? = null

    companion object {
        private const val TAG = "AgentService"
        private const val CHANNEL_ID = "pcp_agent_service_channel"
        private const val NOTIFICATION_ID = 1001
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, buildNotification())

        val serverUrl = "ws://10.0.2.2:8080/agent/v1/connect" // Default dev server URL
        val agentId = "agt_s7_edge_001"

        wsClient = AgentWebSocketClient(applicationContext, serverUrl, agentId)
        wsClient?.connect()
        Log.i(TAG, "AgentService started successfully")
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        super.onDestroy()
        wsClient?.disconnect()
        Log.i(TAG, "AgentService destroyed")
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
