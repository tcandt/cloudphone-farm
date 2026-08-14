package com.tcandt.cloudphone.agent.media

import android.content.Intent
import android.media.projection.MediaProjectionManager
import android.os.Bundle
import android.util.Log
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity

class ConsentPromptActivity : AppCompatActivity() {

    companion object {
        private const val TAG = "ConsentPromptActivity"
    }

    private val projectionLauncher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        if (result.resultCode == RESULT_OK && result.data != null) {
            Log.i(TAG, "MediaProjection permission consent granted by user")
            ScreenCaptureManager.onConsentGranted(this, result.resultCode, result.data!!)
        } else {
            Log.w(TAG, "MediaProjection permission consent denied by user")
            ScreenCaptureManager.onConsentDenied()
        }
        finish()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        try {
            val projectionManager = getSystemService(MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
            projectionLauncher.launch(projectionManager.createScreenCaptureIntent())
        } catch (e: Exception) {
            Log.e(TAG, "Failed to launch MediaProjection consent intent: ${e.message}", e)
            ScreenCaptureManager.onConsentDenied()
            finish()
        }
    }
}
