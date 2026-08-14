package com.tcandt.cloudphone.agent.command

import android.content.Context
import android.content.SharedPreferences

class FencingStore(context: Context) {
    private val prefs: SharedPreferences = context.getSharedPreferences("pcp_fencing_store", Context.MODE_PRIVATE)

    companion object {
        private const val KEY_HIGHEST_FENCING_TOKEN = "highest_fencing_token"
    }

    @Synchronized
    fun getHighestFencingToken(): Long {
        return prefs.getLong(KEY_HIGHEST_FENCING_TOKEN, 0L)
    }

    @Synchronized
    fun validateAndUpdate(incomingToken: Long): Boolean {
        val currentHighest = getHighestFencingToken()
        if (incomingToken < currentHighest) {
            return false // Mismatch / Stale fencing token from previous lease
        }
        prefs.edit().putLong(KEY_HIGHEST_FENCING_TOKEN, incomingToken).apply()
        return true
    }
}
