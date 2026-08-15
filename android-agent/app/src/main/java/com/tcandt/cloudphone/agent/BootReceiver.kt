package com.tcandt.cloudphone.agent

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Build
import android.util.Log

/**
 * Handles ACTION_BOOT_COMPLETED to launch core background AgentService for WSS connectivity & presence.
 * 
 * IMPORTANT (Android 12+/15 FGS Restrictions):
 * BOOT_COMPLETED starts ONLY core AgentService connectivity for WSS reconnect and heartbeat presence.
 * It strictly DOES NOT auto-start MediaProjection foreground service, which requires explicit user consent flow.
 */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == Intent.ACTION_BOOT_COMPLETED) {
            Log.i("BootReceiver", "Device boot completed. Auto-starting core AgentService connectivity...")
            val serviceIntent = Intent(context, AgentService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(serviceIntent)
            } else {
                context.startService(serviceIntent)
            }
        }
    }
}
