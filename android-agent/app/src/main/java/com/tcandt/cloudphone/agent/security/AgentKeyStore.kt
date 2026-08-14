package com.tcandt.cloudphone.agent.security

import android.content.Context
import android.util.Base64
import android.util.Log
import java.security.KeyPair
import java.security.KeyPairGenerator
import java.security.MessageDigest
import java.security.PrivateKey
import java.security.PublicKey
import java.security.SecureRandom
import java.security.Signature

class AgentKeyStore(context: Context) {
    private val prefs = context.getSharedPreferences("pcp_agent_keystore", Context.MODE_PRIVATE)

    companion object {
        private const val TAG = "AgentKeyStore"
        private const val KEY_PRIVATE_B64 = "agent_private_key_b64"
        private const val KEY_PUBLIC_B64 = "agent_public_key_b64"
        private const val KEY_FINGERPRINT = "agent_fingerprint"
    }

    init {
        if (!prefs.contains(KEY_PRIVATE_B64)) {
            generateAndStoreKeys()
        }
    }

    private fun generateAndStoreKeys() {
        try {
            val keyPairGen = KeyPairGenerator.getInstance("Ed25519")
            val keyPair = keyPairGen.generateKeyPair()

            val pubBytes = keyPair.public.encoded
            val privBytes = keyPair.private.encoded

            val pubB64 = Base64.encodeToString(pubBytes, Base64.NO_WRAP)
            val privB64 = Base64.encodeToString(privBytes, Base64.NO_WRAP)

            val digest = MessageDigest.getInstance("SHA-256")
            val fpBytes = digest.digest(pubBytes)
            val fpHex = fpBytes.joinToString("") { "%02x".format(it) }

            prefs.edit()
                .putString(KEY_PUBLIC_B64, pubB64)
                .putString(KEY_PRIVATE_B64, privB64)
                .putString(KEY_FINGERPRINT, fpHex)
                .apply()

            Log.i(TAG, "Generated new Ed25519 keypair for Android Agent with fingerprint: $fpHex")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to generate Ed25519 keypair: ${e.message}", e)
        }
    }

    fun getFingerprint(): String {
        return prefs.getString(KEY_FINGERPRINT, "") ?: ""
    }

    fun signMessage(canonicalMessage: String): String {
        return try {
            val privB64 = prefs.getString(KEY_PRIVATE_B64, null) ?: return ""
            val privBytes = Base64.decode(privB64, Base64.NO_WRAP)

            val signature = Signature.getInstance("Ed25519")
            val keyFactory = java.security.KeyFactory.getInstance("Ed25519")
            val privKey = keyFactory.generatePrivate(java.security.spec.PKCS8EncodedKeySpec(privBytes))

            signature.initSign(privKey)
            signature.update(canonicalMessage.toByteArray(Charsets.UTF_8))
            val sigBytes = signature.sign()
            Base64.encodeToString(sigBytes, Base64.NO_WRAP)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to sign message with Ed25519 key: ${e.message}", e)
            ""
        }
    }
}
