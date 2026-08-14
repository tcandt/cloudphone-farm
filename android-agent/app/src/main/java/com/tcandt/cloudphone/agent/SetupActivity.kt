package com.tcandt.cloudphone.agent

import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.util.Log
import android.widget.Button
import android.widget.EditText
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.tcandt.cloudphone.agent.config.AgentConfigStore
import com.tcandt.cloudphone.agent.security.AgentKeyStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

class SetupActivity : AppCompatActivity() {

    private lateinit var configStore: AgentConfigStore
    private lateinit var keyStore: AgentKeyStore

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_setup)

        configStore = AgentConfigStore(this)
        keyStore = AgentKeyStore(this)

        val etServerUrl = findViewById<EditText>(R.id.etServerUrl)
        val etEnrollToken = findViewById<EditText>(R.id.etEnrollToken)
        val btnEnroll = findViewById<Button>(R.id.btnEnroll)
        val tvStatus = findViewById<TextView>(R.id.tvStatus)

        etServerUrl.setText(configStore.getServerUrl())

        if (configStore.isEnrolled()) {
            tvStatus.text = "Device Enrolled!\nAgent ID: ${configStore.getAgentId()}\nDevice ID: ${configStore.getDeviceId()}"
            startAgentService()
        }

        btnEnroll.setOnClickListener {
            val serverUrl = etServerUrl.text.toString().trim()
            val tokenCode = etEnrollToken.text.toString().trim()

            if (serverUrl.isEmpty() || tokenCode.isEmpty()) {
                Toast.makeText(this, "Please enter Server URL and Token Code", Toast.LENGTH_SHORT).show()
                return@setOnClickListener
            }

            btnEnroll.isEnabled = false
            tvStatus.text = "Enrolling device..."

            CoroutineScope(Dispatchers.IO).launch {
                performEnrollment(serverUrl, tokenCode, tvStatus, btnEnroll)
            }
        }
    }

    private suspend fun performEnrollment(serverUrl: String, tokenCode: String, tvStatus: TextView, btnEnroll: Button) {
        try {
            val pubKeyB64 = keyStore.getPublicKeyBase64()
            val fingerprint = keyStore.getFingerprint()

            val jsonBody = JSONObject().apply {
                put("token_code", tokenCode)
                put("device_fingerprint", fingerprint)
                put("model", "${Build.MANUFACTURER} ${Build.MODEL}")
                put("os_version", "Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})")
                put("public_key", pubKeyB64)
            }

            val client = OkHttpClient()
            val enrollUrl = "${serverUrl.trimEnd('/')}/api/v1/agents/enroll"

            val body = jsonBody.toString().toRequestBody("application/json".toMediaType())
            val request = Request.Builder().url(enrollUrl).post(body).build()

            val response = client.newCall(request).execute()
            val respStr = response.body?.string() ?: ""

            withContext(Dispatchers.Main) {
                btnEnroll.isEnabled = true
                if (response.isSuccessful) {
                    val respJson = JSONObject(respStr)
                    val agentId = respJson.optString("agent_id")
                    val deviceId = respJson.optString("device_id")
                    val orgId = respJson.optString("organization_id")

                    configStore.saveEnrollment(serverUrl, agentId, deviceId, orgId)

                    tvStatus.text = "Enrollment Successful!\nAgent ID: $agentId\nDevice ID: $deviceId"
                    Toast.makeText(this@SetupActivity, "Enrollment Successful!", Toast.LENGTH_LONG).show()

                    startAgentService()
                } else {
                    tvStatus.text = "Enrollment Failed: HTTP ${response.code}\n$respStr"
                    Log.e("SetupActivity", "Enrollment failed: $respStr")
                }
            }
        } catch (e: Exception) {
            withContext(Dispatchers.Main) {
                btnEnroll.isEnabled = true
                tvStatus.text = "Error: ${e.message}"
                Log.e("SetupActivity", "Enrollment exception: ${e.message}", e)
            }
        }
    }

    private fun startAgentService() {
        val intent = Intent(this, AgentService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }
}
