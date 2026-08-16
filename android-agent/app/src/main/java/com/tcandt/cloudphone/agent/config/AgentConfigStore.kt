package com.tcandt.cloudphone.agent.config

import android.content.Context
import android.content.SharedPreferences

class AgentConfigStore(context: Context) {
    private val prefs: SharedPreferences = context.getSharedPreferences("pcp_agent_config", Context.MODE_PRIVATE)

    companion object {
        private const val KEY_SERVER_URL = "server_url"
        private const val KEY_AGENT_ID = "agent_id"
        private const val KEY_DEVICE_ID = "device_id"
        private const val KEY_ORG_ID = "org_id"
        private const val KEY_IS_ENROLLED = "is_enrolled"
    }

    fun getServerUrl(): String {
        return prefs.getString(KEY_SERVER_URL, "http://192.168.1.100:8080") ?: "http://192.168.1.100:8080"
    }

    fun getWssUrl(): String {
        val base = getServerUrl().trimEnd('/')
        val wssBase = if (base.startsWith("https://")) {
            base.replace("https://", "wss://")
        } else {
            base.replace("http://", "ws://")
        }
        return "$wssBase/agent/v1/connect"
    }

    fun getAgentId(): String {
        return prefs.getString(KEY_AGENT_ID, "") ?: ""
    }

    fun getDeviceId(): String {
        return prefs.getString(KEY_DEVICE_ID, "") ?: ""
    }

    fun isEnrolled(): Boolean {
        return prefs.getBoolean(KEY_IS_ENROLLED, false)
    }

    fun saveEnrollment(serverUrl: String, agentId: String, deviceId: String, orgId: String) {
        prefs.edit()
            .putString(KEY_SERVER_URL, serverUrl)
            .putString(KEY_AGENT_ID, agentId)
            .putString(KEY_DEVICE_ID, deviceId)
            .putString(KEY_ORG_ID, orgId)
            .putBoolean(KEY_IS_ENROLLED, true)
            .commit()
    }

    fun resetEnrollment() {
        prefs.edit()
            .remove(KEY_AGENT_ID)
            .remove(KEY_DEVICE_ID)
            .remove(KEY_ORG_ID)
            .putBoolean(KEY_IS_ENROLLED, false)
            .commit()
    }
}
