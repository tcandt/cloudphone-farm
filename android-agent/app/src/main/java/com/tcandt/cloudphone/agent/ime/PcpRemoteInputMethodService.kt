package com.tcandt.cloudphone.agent.ime

import android.inputmethodservice.InputMethodService
import android.util.Log
import android.view.View
import android.view.inputmethod.EditorInfo
import com.tcandt.cloudphone.agent.logging.AgentLogStore

class PcpRemoteInputMethodService : InputMethodService() {

    companion object {
        private const val TAG = "PcpRemoteIME"
        @Volatile
        var activeInstance: PcpRemoteInputMethodService? = null
            private set

        fun isImeActive(): Boolean = activeInstance != null
    }

    override fun onCreate() {
        super.onCreate()
        activeInstance = this
        AgentLogStore.getInstance(this).log(
            level = "INFO",
            category = "CONTROL",
            eventCode = "IME_SERVICE_CREATED",
            message = "PCP Remote Keyboard IME service initialized"
        )
    }

    override fun onStartInput(attribute: EditorInfo?, restarting: Boolean) {
        super.onStartInput(attribute, restarting)
        activeInstance = this
    }

    override fun onCreateInputView(): View? {
        // Transparent / Minimalist view for remote headless injection
        return View(this).apply {
            layoutParams = android.view.ViewGroup.LayoutParams(1, 1)
        }
    }

    fun injectText(text: String): Boolean {
        val ic = currentInputConnection ?: return false
        return try {
            ic.commitText(text, 1)
            AgentLogStore.getInstance(this).log(
                level = "INFO",
                category = "CONTROL",
                eventCode = "IME_TEXT_INJECTED",
                message = "Injected ${text.length} chars via active IME InputConnection"
            )
            true
        } catch (e: Throwable) {
            Log.e(TAG, "Failed to commit text via IME: ${e.message}")
            false
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        if (activeInstance === this) {
            activeInstance = null
        }
        AgentLogStore.getInstance(this).log(
            level = "INFO",
            category = "CONTROL",
            eventCode = "IME_SERVICE_DESTROYED",
            message = "PCP Remote Keyboard IME service destroyed"
        )
    }
}
