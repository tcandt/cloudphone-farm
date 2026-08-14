package com.tcandt.cloudphone.agent.security

import android.content.Context
import android.util.Base64
import android.util.Log
import com.google.crypto.tink.subtle.Ed25519Sign
import java.security.MessageDigest

class AgentKeyStore(context: Context) {
    private val prefs = context.getSharedPreferences("pcp_agent_keystore", Context.MODE_PRIVATE)

    companion object {
        private const val TAG = "AgentKeyStore"
        private const val KEY_PRIVATE_RAW_B64 = "agent_private_raw_b64"
        private const val KEY_PUBLIC_RAW_B64 = "agent_public_raw_b64"
        private const val KEY_FINGERPRINT = "agent_fingerprint"
    }

    init {
        if (!prefs.contains(KEY_PRIVATE_RAW_B64)) {
            generateAndStoreKeys()
        }
    }

    private fun generateAndStoreKeys() {
        try {
            // Standard RFC 8032 Ed25519 Key Generation via Google Tink
            val keyPair = Ed25519Sign.KeyPair.newKeyPair()
            val privBytes = keyPair.privateKey // 32 bytes raw private seed
            val pubBytes = keyPair.publicKey   // 32 bytes raw public key

            val digest = MessageDigest.getInstance("SHA-256")
            val fpBytes = digest.digest(pubBytes)
            val fpHex = fpBytes.joinToString("") { "%02x".format(it) }

            val privB64 = Base64.encodeToString(privBytes, Base64.NO_WRAP)
            val pubB64 = Base64.encodeToString(pubBytes, Base64.NO_WRAP)

            prefs.edit()
                .putString(KEY_PRIVATE_RAW_B64, privB64)
                .putString(KEY_PUBLIC_RAW_B64, pubB64)
                .putString(KEY_FINGERPRINT, fpHex)
                .apply()

            Log.i(TAG, "Generated standard Google Tink Ed25519 keypair for Android Agent. Fingerprint: $fpHex")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to generate Ed25519 keypair: ${e.message}", e)
        }
    }

    fun getPublicKeyBase64(): String {
        return prefs.getString(KEY_PUBLIC_RAW_B64, "") ?: ""
    }

    fun getFingerprint(): String {
        return prefs.getString(KEY_FINGERPRINT, "") ?: ""
    }

    fun signMessage(canonicalMessage: String): String {
        return try {
            val privB64 = prefs.getString(KEY_PRIVATE_RAW_B64, null) ?: return ""
            val privBytes = Base64.decode(privB64, Base64.NO_WRAP)
            val messageBytes = canonicalMessage.toByteArray(Charsets.UTF_8)

            val signer = Ed25519Sign(privBytes)
            val sigBytes = signer.sign(messageBytes)

            Base64.encodeToString(sigBytes, Base64.NO_WRAP)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to sign message with Tink Ed25519: ${e.message}", e)
            ""
        }
    }
}
