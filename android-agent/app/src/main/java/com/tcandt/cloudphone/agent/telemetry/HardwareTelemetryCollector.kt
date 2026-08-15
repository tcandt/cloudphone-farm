package com.tcandt.cloudphone.agent.telemetry

import android.app.ActivityManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.BatteryManager
import org.json.JSONObject

object HardwareTelemetryCollector {

    fun collectMetrics(context: Context): JSONObject {
        val json = JSONObject()

        val battery = getBatteryLevel(context)
        if (battery != null) {
            json.put("battery", battery)
        }

        val network = getNetworkType(context)
        if (network != null) {
            json.put("network", network)
        }

        val ramUsage = getRamUsagePercent(context)
        if (ramUsage != null) {
            json.put("ram_usage", ramUsage)
        }

        val tempC = getBatteryTemperature(context)
        if (tempC != null) {
            json.put("temperature_c", tempC)
        }

        val cpuUsage = getCpuUsagePercent()
        if (cpuUsage != null) {
            json.put("cpu_usage", cpuUsage)
        }

        return json
    }

    private fun getBatteryLevel(context: Context): Int? {
        return try {
            val bm = context.getSystemService(Context.BATTERY_SERVICE) as? BatteryManager
            val level = bm?.getIntProperty(BatteryManager.BATTERY_PROPERTY_CAPACITY) ?: -1
            if (level in 0..100) level else null
        } catch (e: Exception) {
            null
        }
    }

    private fun getNetworkType(context: Context): String? {
        return try {
            val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager
            val activeNetwork = cm?.activeNetwork ?: return null
            val caps = cm.getNetworkCapabilities(activeNetwork) ?: return null
            when {
                caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
                caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "cellular"
                caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "ethernet"
                else -> "other"
            }
        } catch (e: Exception) {
            null
        }
    }

    private fun getRamUsagePercent(context: Context): Double? {
        return try {
            val am = context.getSystemService(Context.ACTIVITY_SERVICE) as? ActivityManager
            val memInfo = ActivityManager.MemoryInfo()
            am?.getMemoryInfo(memInfo)
            if (memInfo.totalMem > 0) {
                val used = memInfo.totalMem - memInfo.availMem
                val pct = (used.toDouble() / memInfo.totalMem.toDouble()) * 100.0
                Math.round(pct * 10.0) / 10.0
            } else null
        } catch (e: Exception) {
            null
        }
    }

    private fun getBatteryTemperature(context: Context): Double? {
        return try {
            val intent = context.registerReceiver(null, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
            val tempTenths = intent?.getIntExtra(BatteryManager.EXTRA_TEMPERATURE, -1) ?: -1
            if (tempTenths > 0) {
                val tempC = tempTenths / 10.0
                Math.round(tempC * 10.0) / 10.0
            } else null
        } catch (e: Exception) {
            null
        }
    }

    private fun getCpuUsagePercent(): Double? {
        // Proc stat reading restricted on Android 8+; return null if unreadable. Zero synthetic fallbacks.
        return try {
            val statFile = java.io.File("/proc/stat")
            if (!statFile.canRead()) return null
            val lines = statFile.readLines()
            if (lines.isEmpty()) return null
            val toks = lines[0].split("\\s+".toRegex())
            if (toks.size < 8) return null
            val user = toks[1].toDoubleOrNull() ?: return null
            val nice = toks[2].toDoubleOrNull() ?: return null
            val system = toks[3].toDoubleOrNull() ?: return null
            val idle = toks[4].toDoubleOrNull() ?: return null
            val total = user + nice + system + idle
            if (total <= 0) return null
            val pct = ((user + nice + system) / total) * 100.0
            Math.round(pct * 10.0) / 10.0
        } catch (e: Exception) {
            null
        }
    }
}
