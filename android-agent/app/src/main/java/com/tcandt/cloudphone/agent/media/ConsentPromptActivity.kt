package com.tcandt.cloudphone.agent.media

import android.content.Intent
import android.media.projection.MediaProjectionManager
import android.os.Bundle
import android.util.Log
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity

class ConsentPromptActivity : AppCompatActivity() {

    private var requestGeneration: Long = 0L

    companion object {
        private const val TAG = "ConsentPromptActivity"
    }

    private val projectionLauncher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        if (result.resultCode == RESULT_OK && result.data != null) {
            Log.i(TAG, "MediaProjection permission consent granted by user (Gen=$requestGeneration)")
            ScreenCaptureManager.onConsentGranted(this, requestGeneration, result.resultCode, result.data!!)
        } else {
            Log.w(TAG, "MediaProjection permission consent denied by user (Gen=$requestGeneration)")
            ScreenCaptureManager.onConsentDenied(this, requestGeneration)
        }
        finish()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestGeneration = intent.getLongExtra("extra_generation", 0L)

        // Verify request generation is still valid
        if (requestGeneration != ScreenCaptureManager.getRequestGeneration() ||
            ScreenCaptureManager.getState() != ScreenCaptureState.CONSENT_REQUIRED
        ) {
            Log.w(TAG, "ConsentPromptActivity launched for stale/canceled generation ($requestGeneration vs current ${ScreenCaptureManager.getRequestGeneration()}). Dismissing.")
            finish()
            return
        }

        try {
            val projectionManager = getSystemService(MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
            projectionLauncher.launch(projectionManager.createScreenCaptureIntent())
        } catch (e: Exception) {
            Log.e(TAG, "Failed to launch MediaProjection consent intent: ${e.message}", e)
            ScreenCaptureManager.onConsentDenied(this, requestGeneration)
            finish()
        }
    }
}
