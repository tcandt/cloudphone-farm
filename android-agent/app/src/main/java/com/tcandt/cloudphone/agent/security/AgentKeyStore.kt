package com.tcandt.cloudphone.agent.security

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import android.util.Log
import com.google.crypto.tink.subtle.Ed25519Sign
import java.security.KeyStore
import java.security.MessageDigest
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

class AgentKeyStore(context: Context) {
    private val prefs = context.getSharedPreferences("pcp_agent_keystore", Context.MODE_PRIVATE)

    companion object {
        private const val TAG = "AgentKeyStore"
        private const val KEY_ALIAS_AES = "pcp_agent_seed_key"
        private const val KEY_ENCRYPTED_SEED_B64 = "agent_encrypted_seed_b64"
        private const val KEY_IV_B64 = "agent_iv_b64"
        private const val KEY_PUBLIC_RAW_B64 = "agent_public_raw_b64"
        private const val KEY_FINGERPRINT = "agent_fingerprint"
        private const val KEY_OLD_LEGACY_RAW_SEED_B64 = "agent_private_raw_b64"
    }

    private fun logD(tag: String, msg: String) { try { Log.d(tag, msg) } catch (_: Throwable) {} }
    private fun logI(tag: String, msg: String) { try { Log.i(tag, msg) } catch (_: Throwable) {} }
    private fun logW(tag: String, msg: String) { try { Log.w(tag, msg) } catch (_: Throwable) {} }
    private fun logE(tag: String, msg: String, t: Throwable? = null) { try { if (t != null) Log.e(tag, msg, t) else Log.e(tag, msg) } catch (_: Throwable) {} }

    init {
        try {
            ensureKeystoreKey()

            // Migration: Migrate legacy plaintext raw seed to Android KeyStore encrypted seed
            if (prefs.contains(KEY_OLD_LEGACY_RAW_SEED_B64) && !prefs.contains(KEY_ENCRYPTED_SEED_B64)) {
                migrateLegacyRawSeed()
            } else if (!prefs.contains(KEY_ENCRYPTED_SEED_B64)) {
                generateAndStoreKeys()
            }
        } catch (e: Throwable) {
            logE(TAG, "KeyStore initialization error (JVM test mode): ${e.message}")
        }
    }

    private fun ensureKeystoreKey() {
        val keyStore = KeyStore.getInstance("AndroidKeyStore")
        keyStore.load(null)
        if (!keyStore.containsAlias(KEY_ALIAS_AES)) {
            val keyGenerator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
            val keySpec = KeyGenParameterSpec.Builder(
                KEY_ALIAS_AES,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build()
            keyGenerator.init(keySpec)
            keyGenerator.generateKey()
            logI(TAG, "Generated new Android KeyStore AES-256-GCM key: $KEY_ALIAS_AES")
        }
    }

    private fun getSecretKey(): SecretKey {
        val keyStore = KeyStore.getInstance("AndroidKeyStore")
        keyStore.load(null)
        return (keyStore.getEntry(KEY_ALIAS_AES, null) as KeyStore.SecretKeyEntry).secretKey
    }

    private fun encryptSeed(rawSeed: ByteArray): Pair<String, String> {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, getSecretKey())
        val iv = cipher.iv
        val ciphertext = cipher.doFinal(rawSeed)
        return Pair(
            Base64.encodeToString(ciphertext, Base64.NO_WRAP),
            Base64.encodeToString(iv, Base64.NO_WRAP)
        )
    }

    private fun decryptSeed(encryptedB64: String, ivB64: String): ByteArray {
        val ciphertext = Base64.decode(encryptedB64, Base64.NO_WRAP)
        val iv = Base64.decode(ivB64, Base64.NO_WRAP)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        val spec = GCMParameterSpec(128, iv)
        cipher.init(Cipher.DECRYPT_MODE, getSecretKey(), spec)
        return cipher.doFinal(ciphertext)
    }

    private fun migrateLegacyRawSeed() {
        val legacyB64 = prefs.getString(KEY_OLD_LEGACY_RAW_SEED_B64, null) ?: return
        val rawSeed = Base64.decode(legacyB64, Base64.NO_WRAP)

        if (rawSeed.size != 32) {
            logE(TAG, "Legacy seed size invalid: ${rawSeed.size} bytes (expected 32)")
            throw IllegalStateException("Legacy raw seed size invalid: ${rawSeed.size}")
        }

        val existingPublicKey = prefs.getString(KEY_PUBLIC_RAW_B64, null)
        val existingFingerprint = prefs.getString(KEY_FINGERPRINT, null)

        if (existingPublicKey.isNullOrEmpty() || existingFingerprint.isNullOrEmpty()) {
            logE(TAG, "Cannot migrate seed: missing existing public key or fingerprint")
            throw IllegalStateException("Cannot migrate seed: missing existing public key or fingerprint")
        }

        val (ciphertext, iv) = encryptSeed(rawSeed)

        prefs.edit()
            .putString(KEY_ENCRYPTED_SEED_B64, ciphertext)
            .putString(KEY_IV_B64, iv)
            // PRESERVE existing public key Base64 and fingerprint Base64 UNTOUCHED!
            .remove(KEY_OLD_LEGACY_RAW_SEED_B64)
            .commit()

        logI(TAG, "Successfully migrated legacy seed to KeyStore AES-GCM. Machine identity preserved: $existingFingerprint")
    }

    private fun generateAndStoreKeys() {
        try {
            val keyPair = Ed25519Sign.KeyPair.newKeyPair()
            val privBytes = keyPair.privateKey
            val pubBytes = keyPair.publicKey

            val digest = MessageDigest.getInstance("SHA-256")
            val fpBytes = digest.digest(pubBytes)
            val fpHex = fpBytes.joinToString("") { "%02x".format(it) }

            val (encryptedSeedB64, ivB64) = encryptSeed(privBytes)
            val pubB64 = Base64.encodeToString(pubBytes, Base64.NO_WRAP)

            prefs.edit()
                .putString(KEY_ENCRYPTED_SEED_B64, encryptedSeedB64)
                .putString(KEY_IV_B64, ivB64)
                .putString(KEY_PUBLIC_RAW_B64, pubB64)
                .putString(KEY_FINGERPRINT, fpHex)
                .apply()

            logI(TAG, "Generated and Android Keystore AES-GCM encrypted Ed25519 seed. Fingerprint: $fpHex")
        } catch (e: Throwable) {
            logE(TAG, "Failed to generate encrypted Ed25519 keypair: ${e.message}", e)
        }
    }

    fun getPublicKeyBase64(): String {
        var key = prefs.getString(KEY_PUBLIC_RAW_B64, "") ?: ""
        if (key.isEmpty()) {
            regenerateKeys()
            key = prefs.getString(KEY_PUBLIC_RAW_B64, "") ?: ""
        }
        return key
    }

    fun getFingerprint(): String {
        var fp = prefs.getString(KEY_FINGERPRINT, "") ?: ""
        if (fp.isEmpty()) {
            regenerateKeys()
            fp = prefs.getString(KEY_FINGERPRINT, "") ?: ""
        }
        return fp
    }

    fun getKeySecurityLevel(): String {
        return try {
            val factory = javax.crypto.SecretKeyFactory.getInstance(getSecretKey().algorithm, "AndroidKeyStore")
            val keyInfo = factory.getKeySpec(getSecretKey(), android.security.keystore.KeyInfo::class.java) as android.security.keystore.KeyInfo
            if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.S) {
                when (keyInfo.securityLevel) {
                    KeyProperties.SECURITY_LEVEL_STRONGBOX -> "STRONGBOX"
                    KeyProperties.SECURITY_LEVEL_TRUSTED_ENVIRONMENT -> "TRUSTED_ENVIRONMENT"
                    KeyProperties.SECURITY_LEVEL_SOFTWARE -> "SOFTWARE"
                    else -> if (keyInfo.isInsideSecureHardware) "TRUSTED_ENVIRONMENT" else "SOFTWARE"
                }
            } else {
                if (keyInfo.isInsideSecureHardware) "TRUSTED_ENVIRONMENT" else "SOFTWARE"
            }
        } catch (e: Throwable) {
            "UNKNOWN"
        }
    }

    fun getKeyProtectionMetadata(): org.json.JSONObject {
        return org.json.JSONObject().apply {
            put("algorithm", "AES-256-GCM")
            put("provider", "AndroidKeyStore")
            put("security_level", getKeySecurityLevel())
        }
    }

    fun signMessage(canonicalMessage: String): String {
        return try {
            val encryptedSeedB64 = prefs.getString(KEY_ENCRYPTED_SEED_B64, null) ?: return ""
            val ivB64 = prefs.getString(KEY_IV_B64, null) ?: return ""
            val privBytes = decryptSeed(encryptedSeedB64, ivB64)
            val messageBytes = canonicalMessage.toByteArray(Charsets.UTF_8)

            val signer = Ed25519Sign(privBytes)
            val sigBytes = signer.sign(messageBytes)

            Base64.encodeToString(sigBytes, Base64.NO_WRAP)
        } catch (e: Throwable) {
            logE(TAG, "Failed to sign message with Tink Ed25519: ${e.message}")
            ""
        }
    }

    fun deleteKeys() {
        try {
            // 1. Delete AES-256 Key from AndroidKeyStore
            val keyStore = KeyStore.getInstance("AndroidKeyStore")
            keyStore.load(null)
            if (keyStore.containsAlias(KEY_ALIAS_AES)) {
                keyStore.deleteEntry(KEY_ALIAS_AES)
                logI(TAG, "Deleted Android KeyStore entry: $KEY_ALIAS_AES")
            }
        } catch (e: Throwable) {
            logW(TAG, "Error deleting Android KeyStore entry: ${e.message}")
        }

        // 2. Surgically remove only identity-related keys from SharedPreferences
        prefs.edit()
            .remove(KEY_ENCRYPTED_SEED_B64)
            .remove(KEY_IV_B64)
            .remove(KEY_PUBLIC_RAW_B64)
            .remove(KEY_FINGERPRINT)
            .remove(KEY_OLD_LEGACY_RAW_SEED_B64)
            .commit()

        logI(TAG, "Cryptographic identity keys surgically deleted from AgentKeyStore.")
    }

    fun regenerateKeys() {
        ensureKeystoreKey()
        generateAndStoreKeys()
    }
}


