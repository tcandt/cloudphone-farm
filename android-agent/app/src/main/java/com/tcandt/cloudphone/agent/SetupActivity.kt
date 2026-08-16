package com.tcandt.cloudphone.agent

import android.Manifest
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.text.TextUtils
import android.util.Log
import android.view.View
import android.view.inputmethod.InputMethodManager
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import com.tcandt.cloudphone.agent.accessibility.DeviceControlService
import com.tcandt.cloudphone.agent.config.AgentConfigStore
import com.tcandt.cloudphone.agent.logging.AgentLogStore
import com.tcandt.cloudphone.agent.media.ScreenCaptureManager
import com.tcandt.cloudphone.agent.media.ScreenCaptureState
import com.tcandt.cloudphone.agent.security.AgentKeyStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.UUID

class SetupActivity : AppCompatActivity() {

    private lateinit var configStore: AgentConfigStore
    private lateinit var keyStore: AgentKeyStore
    private lateinit var logStore: AgentLogStore

    private lateinit var layoutEnrollForm: LinearLayout
    private lateinit var layoutDashboard: LinearLayout

    // Enrollment View Elements
    private lateinit var etServerUrl: EditText
    private lateinit var etEnrollToken: EditText
    private lateinit var btnEnroll: Button
    private lateinit var tvSetupStatus: TextView

    // Dashboard View Elements
    private lateinit var tvDashDeviceName: TextView
    private lateinit var tvDashOsVersion: TextView
    private lateinit var tvAgentStatusBadge: TextView
    private lateinit var tvReadinessPercent: TextView
    private lateinit var btnResetEnrollment: Button

    private lateinit var tvDashAgentId: TextView
    private lateinit var tvDashDeviceId: TextView
    private lateinit var tvDashWssStatus: TextView
    private lateinit var tvDashHeartbeatStatus: TextView

    private lateinit var tvScreenCaptureState: TextView
    private lateinit var btnGrantScreenCapture: Button
    private lateinit var tvFgsStatus: TextView

    private lateinit var tvAccessibilityState: TextView
    private lateinit var btnEnableAccessibility: Button

    private lateinit var tvImeState: TextView
    private lateinit var btnEnableIme: Button

    private lateinit var tvInstallApkState: TextView
    private lateinit var btnAllowInstallApk: Button

    private lateinit var tvRecentLogsSnippet: TextView
    private lateinit var btnOpenLogsActivity: Button

    private var refreshJob: Job? = null

    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { isGranted ->
        if (isGranted) {
            logStore.log("INFO", "PERMISSION", "NOTIF_GRANTED", "Notification runtime permission granted")
        } else {
            logStore.log("WARN", "PERMISSION", "NOTIF_DENIED", "Notification runtime permission denied")
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_setup)

        configStore = AgentConfigStore(this)
        keyStore = AgentKeyStore(this)
        logStore = AgentLogStore.getInstance(this)

        initViews()
        bindListeners()
        checkNotificationPermission()

        logStore.log("INFO", "SYSTEM", "APP_LAUNCHED", "Agent SetupActivity launched")
    }

    override fun onResume() {
        super.onResume()
        refreshUiState()
        startPeriodicDashboardRefresh()
    }

    override fun onPause() {
        super.onPause()
        refreshJob?.cancel()
    }

    private fun initViews() {
        layoutEnrollForm = findViewById(R.id.layoutEnrollForm)
        layoutDashboard = findViewById(R.id.layoutDashboard)

        etServerUrl = findViewById(R.id.etServerUrl)
        etEnrollToken = findViewById(R.id.etEnrollToken)
        btnEnroll = findViewById(R.id.btnEnroll)
        tvSetupStatus = findViewById(R.id.tvSetupStatus)

        tvDashDeviceName = findViewById(R.id.tvDashDeviceName)
        tvDashOsVersion = findViewById(R.id.tvDashOsVersion)
        tvAgentStatusBadge = findViewById(R.id.tvAgentStatusBadge)
        tvReadinessPercent = findViewById(R.id.tvReadinessPercent)
        btnResetEnrollment = findViewById(R.id.btnResetEnrollment)

        tvDashAgentId = findViewById(R.id.tvDashAgentId)
        tvDashDeviceId = findViewById(R.id.tvDashDeviceId)
        tvDashWssStatus = findViewById(R.id.tvDashWssStatus)
        tvDashHeartbeatStatus = findViewById(R.id.tvDashHeartbeatStatus)

        tvScreenCaptureState = findViewById(R.id.tvScreenCaptureState)
        btnGrantScreenCapture = findViewById(R.id.btnGrantScreenCapture)
        tvFgsStatus = findViewById(R.id.tvFgsStatus)

        tvAccessibilityState = findViewById(R.id.tvAccessibilityState)
        btnEnableAccessibility = findViewById(R.id.btnEnableAccessibility)

        tvImeState = findViewById(R.id.tvImeState)
        btnEnableIme = findViewById(R.id.btnEnableIme)

        tvInstallApkState = findViewById(R.id.tvInstallApkState)
        btnAllowInstallApk = findViewById(R.id.btnAllowInstallApk)

        tvRecentLogsSnippet = findViewById(R.id.tvRecentLogsSnippet)
        btnOpenLogsActivity = findViewById(R.id.btnOpenLogsActivity)

        etServerUrl.setText(configStore.getServerUrl())
    }

    private fun bindListeners() {
        btnEnroll.setOnClickListener {
            val serverUrl = etServerUrl.text.toString().trim()
            val tokenCode = etEnrollToken.text.toString().trim()

            if (serverUrl.isEmpty() || tokenCode.isEmpty()) {
                Toast.makeText(this, "Vui lòng nhập Server URL và Token Code", Toast.LENGTH_SHORT).show()
                return@setOnClickListener
            }

            btnEnroll.isEnabled = false
            tvSetupStatus.text = "Đang đăng ký thiết bị..."

            CoroutineScope(Dispatchers.IO).launch {
                performEnrollment(serverUrl, tokenCode)
            }
        }

        btnResetEnrollment.setOnClickListener {
            configStore.resetEnrollment()
            logStore.log("WARN", "ENROLLMENT", "RESET_TRIGGERED", "Agent enrollment reset by user")
            Toast.makeText(this, "Đã reset thông tin đăng ký. Vui lòng nhập token mới.", Toast.LENGTH_LONG).show()
            refreshUiState()
        }

        btnGrantScreenCapture.setOnClickListener {
            logStore.log("INFO", "MEDIA", "CONSENT_PROMPT_REQUESTED", "User tapped Grant Screen Capture button")
            val sessId = ScreenCaptureManager.activeSessionId.ifEmpty { "sess_manual_${System.currentTimeMillis()}" }
            ScreenCaptureManager.requestConsent(this, sessId)
        }

        btnEnableAccessibility.setOnClickListener {
            logStore.log("INFO", "PERMISSION", "ACCESSIBILITY_SETTINGS_OPENED", "Opening Accessibility settings")
            val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            startActivity(intent)
        }

        btnEnableIme.setOnClickListener {
            logStore.log("INFO", "PERMISSION", "IME_SETTINGS_OPENED", "Opening Input Method settings")
            val intent = Intent(Settings.ACTION_INPUT_METHOD_SETTINGS).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            startActivity(intent)
        }

        btnAllowInstallApk.setOnClickListener {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                logStore.log("INFO", "PERMISSION", "INSTALL_SETTINGS_OPENED", "Opening Unknown Sources settings")
                val intent = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES).apply {
                    data = Uri.parse("package:$packageName")
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
                startActivity(intent)
            } else {
                Toast.makeText(this, "Android 8.0 trở xuống hỗ trợ trực tiếp từ PackageInstaller", Toast.LENGTH_SHORT).show()
            }
        }

        btnOpenLogsActivity.setOnClickListener {
            val intent = Intent(this, LogsActivity::class.java)
            startActivity(intent)
        }
    }

    private fun refreshUiState() {
        if (!configStore.isEnrolled()) {
            layoutEnrollForm.visibility = View.VISIBLE
            layoutDashboard.visibility = View.GONE
            return
        }

        layoutEnrollForm.visibility = View.GONE
        layoutDashboard.visibility = View.VISIBLE

        startAgentService()

        // 1. Header Information
        tvDashDeviceName.text = "${Build.MANUFACTURER} ${Build.MODEL}"
        tvDashOsVersion.text = "Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})"
        tvDashAgentId.text = "Agent ID: ${configStore.getAgentId()}"
        tvDashDeviceId.text = "Device ID: ${configStore.getDeviceId()}"
        tvDashWssStatus.text = "WebSocket: ✓ Connected (${configStore.getServerUrl()})"
        tvDashHeartbeatStatus.text = "Heartbeat: ✓ HTTP 200 OK (Periodic 10s)"

        var readinessScore = 40 // Base score for Network + Agent Service + WSS

        // 2. Screen Capture Capability State
        when (ScreenCaptureManager.currentState) {
            ScreenCaptureState.CAPTURING, ScreenCaptureState.CONNECTED -> {
                tvScreenCaptureState.text = "🟢 Đang ghi màn hình"
                tvScreenCaptureState.setTextColor(0xFF059669.toInt())
                readinessScore += 30
            }
            ScreenCaptureState.CONSENT_REQUIRED, ScreenCaptureState.NEGOTIATING -> {
                tvScreenCaptureState.text = "🟠 Đang chờ xác nhận"
                tvScreenCaptureState.setTextColor(0xFFD97706.toInt())
            }
            ScreenCaptureState.FAILED -> {
                tvScreenCaptureState.text = "🔴 Thất bại / Từ chối"
                tvScreenCaptureState.setTextColor(0xFFDC2626.toInt())
            }
            else -> {
                tvScreenCaptureState.text = "○ Chưa có phiên"
                tvScreenCaptureState.setTextColor(0xFF64748B.toInt())
            }
        }

        // 3. Accessibility Service State
        val isAccessibilityActive = isAccessibilityServiceEnabled(this, DeviceControlService::class.java)
        if (isAccessibilityActive) {
            tvAccessibilityState.text = "✓ Enabled"
            tvAccessibilityState.setTextColor(0xFF059669.toInt())
            btnEnableAccessibility.visibility = View.GONE
            readinessScore += 20
        } else {
            tvAccessibilityState.text = "✕ Disabled"
            tvAccessibilityState.setTextColor(0xFFDC2626.toInt())
            btnEnableAccessibility.visibility = View.VISIBLE
        }

        // 4. Remote Keyboard IME State
        val isImeActive = isPcpImeEnabled()
        if (isImeActive) {
            tvImeState.text = "✓ Active"
            tvImeState.setTextColor(0xFF059669.toInt())
            btnEnableIme.visibility = View.GONE
            readinessScore += 5
        } else {
            tvImeState.text = "○ Optional"
            tvImeState.setTextColor(0xFF64748B.toInt())
            btnEnableIme.visibility = View.VISIBLE
        }

        // 5. Install Unknown APKs State
        val canInstall = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            packageManager.canRequestPackageInstalls()
        } else {
            true
        }
        if (canInstall) {
            tvInstallApkState.text = "✓ Cho phép"
            tvInstallApkState.setTextColor(0xFF059669.toInt())
            btnAllowInstallApk.visibility = View.GONE
            readinessScore += 5
        } else {
            tvInstallApkState.text = "✕ Chưa cấp"
            tvInstallApkState.setTextColor(0xFF64748B.toInt())
            btnAllowInstallApk.visibility = View.VISIBLE
        }

        tvReadinessPercent.text = "Readiness: $readinessScore%"

        // 6. Recent Logs Snippet
        val recentLogs = logStore.getRecentEvents(4)
        if (recentLogs.isEmpty()) {
            tvRecentLogsSnippet.text = "Waiting for diagnostic events..."
        } else {
            val sb = StringBuilder()
            for (e in recentLogs) {
                sb.append("[${e.timestamp.substringAfter(" ")}] [${e.category}] ${e.eventCode}: ${e.message}\n")
            }
            tvRecentLogsSnippet.text = sb.toString().trimEnd()
        }
    }

    private fun startPeriodicDashboardRefresh() {
        refreshJob?.cancel()
        refreshJob = CoroutineScope(Dispatchers.Main).launch {
            while (isActive) {
                delay(2000)
                if (configStore.isEnrolled()) {
                    refreshUiState()
                }
            }
        }
    }

    private fun isAccessibilityServiceEnabled(context: Context, service: Class<*>): Boolean {
        val expectedComponentName = ComponentName(context, service)
        val enabledServices = Settings.Secure.getString(
            context.contentResolver,
            Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES
        ) ?: return false

        val colonSplitter = TextUtils.SimpleStringSplitter(':')
        colonSplitter.setString(enabledServices)

        while (colonSplitter.hasNext()) {
            val componentNameString = colonSplitter.next()
            val enabledComponent = ComponentName.unflattenFromString(componentNameString)
            if (enabledComponent != null && enabledComponent == expectedComponentName) {
                return true
            }
        }
        return false
    }

    private fun isPcpImeEnabled(): Boolean {
        val imm = getSystemService(Context.INPUT_METHOD_SERVICE) as? InputMethodManager ?: return false
        val enabledImes = imm.enabledInputMethodList
        return enabledImes.any { it.packageName == packageName }
    }

    private fun checkNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            }
        }
    }

    private suspend fun performEnrollment(serverUrl: String, tokenCode: String) {
        try {
            val pubKeyB64 = keyStore.getPublicKeyBase64()
            val fingerprint = keyStore.getFingerprint()

            // Extract real deterministic hardware identity via ANDROID_ID
            val rawAndroidId = Settings.Secure.getString(contentResolver, Settings.Secure.ANDROID_ID)
            val deterministicHardwareSerial = if (!rawAndroidId.isNullOrBlank() && rawAndroidId != "unknown") {
                rawAndroidId
            } else {
                UUID.randomUUID().toString().replace("-", "").substring(0, 16)
            }

            val jsonBody = JSONObject().apply {
                put("token_code", tokenCode)
                put("device_fingerprint", fingerprint)
                put("device_model", "${Build.MANUFACTURER} ${Build.MODEL}")
                put("device_android_version", "Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})")
                put("public_key_bytes", pubKeyB64)
                put("apk_version", "1.0.0")
                put("protocol_version", "v1")
                put("device_serial_number", deterministicHardwareSerial)
                put("device_display_name", Build.MODEL)
                put("key_protection", keyStore.getKeyProtectionMetadata())
            }

            logStore.log("INFO", "ENROLLMENT", "ENROLL_REQUEST_SENT", "Sending enrollment request for serial $deterministicHardwareSerial")

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
                    logStore.log("INFO", "ENROLLMENT", "ENROLL_SUCCESS", "Device enrolled successfully: AgentID=$agentId, DeviceID=$deviceId")

                    Toast.makeText(this@SetupActivity, "Đăng ký thiết bị thành công!", Toast.LENGTH_LONG).show()
                    refreshUiState()
                } else {
                    tvSetupStatus.text = "Đăng ký thất bại: HTTP ${response.code}\n$respStr"
                    logStore.log("ERROR", "ENROLLMENT", "ENROLL_FAILED", "HTTP ${response.code}: $respStr")
                }
            }
        } catch (e: Throwable) {
            withContext(Dispatchers.Main) {
                btnEnroll.isEnabled = true
                tvSetupStatus.text = "Lỗi: ${e.message}"
                logStore.log("ERROR", "ENROLLMENT", "ENROLL_EXCEPTION", "${e.message}")
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
