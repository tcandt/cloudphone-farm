package com.tcandt.cloudphone.agent.media

import android.content.Context
import android.content.Intent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.mockito.Mockito

class ScreenCaptureManagerTest {

    private var startedCount = 0
    private var stoppedCount = 0
    private var failedCount = 0
    private var lastSessionId = ""
    private var lastReason = ""
    private lateinit var dummyContext: Context

    @Before
    fun setUp() {
        startedCount = 0
        stoppedCount = 0
        failedCount = 0
        lastSessionId = ""
        lastReason = ""
        dummyContext = Mockito.mock(Context::class.java, Mockito.RETURNS_DEEP_STUBS)

        ScreenCaptureManager.terminateMediaSession(null, ScreenCaptureManager.SessionOutcome.STOPPED, "setup_reset")
        ScreenCaptureManager.sessionListener = object : ScreenCaptureManager.SessionStateListener {
            override fun onSessionStarted(sessionId: String) {
                startedCount++
                lastSessionId = sessionId
            }

            override fun onSessionStopped(sessionId: String, reason: String) {
                stoppedCount++
                lastSessionId = sessionId
                lastReason = reason
            }

            override fun onSessionFailed(sessionId: String, error: String) {
                failedCount++
                lastSessionId = sessionId
                lastReason = error
            }
        }
    }

    @Test
    fun testInitialStateIsIdle() {
        assertEquals(ScreenCaptureState.IDLE, ScreenCaptureManager.currentState)
        assertEquals("", ScreenCaptureManager.activeSessionId)
    }

    @Test
    fun testStaleConsentGenerationIgnored() {
        val gen1 = ScreenCaptureManager.requestConsent(dummyContext, "sess_1")
        val gen2 = ScreenCaptureManager.requestConsent(dummyContext, "sess_2")
        assertTrue("gen2 must be greater than gen1", gen2 > gen1)

        // Invoke production onConsentGranted with stale gen1
        ScreenCaptureManager.onConsentGranted(dummyContext, gen1, -1, Intent())

        assertEquals("activeSessionId must remain sess_2", "sess_2", ScreenCaptureManager.activeSessionId)
        assertEquals("currentState must remain CONSENT_REQUIRED", ScreenCaptureState.CONSENT_REQUIRED, ScreenCaptureManager.currentState)
    }

    @Test
    fun testStaleFgsReadyListenerRejected() {
        val gen1 = ScreenCaptureManager.requestConsent(dummyContext, "sess_1")
        ScreenCaptureManager.onConsentGranted(dummyContext, gen1, -1, Intent())

        val registeredListener = MediaCaptureServiceNotifier.onFgsReadyListener
        assertTrue("onFgsReadyListener should be registered for gen1", registeredListener != null)

        // Supersede with sess_2
        ScreenCaptureManager.requestConsent(dummyContext, "sess_2")

        // Fire stale FGS_READY callback registered for gen1
        registeredListener?.invoke()

        assertEquals("sess_1", lastSessionId)
        assertEquals("stale_fgs_ready", lastReason)
    }

    @Test
    fun testDuplicateTerminationEmitsExactlyOnce() {
        ScreenCaptureManager.markReady("sess_dup")
        assertEquals(ScreenCaptureState.READY, ScreenCaptureManager.currentState)

        // First termination call
        ScreenCaptureManager.terminateMediaSession(null, ScreenCaptureManager.SessionOutcome.STOPPED, "test_reason")
        assertEquals(1, stoppedCount)
        assertEquals(0, failedCount)
        assertEquals("sess_dup", lastSessionId)
        assertEquals("test_reason", lastReason)

        // Second duplicate termination call
        ScreenCaptureManager.terminateMediaSession(null, ScreenCaptureManager.SessionOutcome.STOPPED, "test_reason_2")
        assertEquals("Duplicate termination call must not emit second stopped callback", 1, stoppedCount)
        assertEquals(0, failedCount)
    }

    @Test
    fun testFailureOutcomeEmitsFailedCallbackOnly() {
        ScreenCaptureManager.markReady("sess_fail")
        ScreenCaptureManager.terminateMediaSession(null, ScreenCaptureManager.SessionOutcome.FAILED, "encoder_error")

        assertEquals(0, stoppedCount)
        assertEquals(1, failedCount)
        assertEquals("sess_fail", lastSessionId)
        assertEquals("encoder_error", lastReason)
        assertEquals(ScreenCaptureState.IDLE, ScreenCaptureManager.currentState)
    }

    @Test
    fun testSystemProjectionStoppedEmitsSingleStoppedCallback() {
        ScreenCaptureManager.markReady("sess_sys")
        ScreenCaptureManager.onProjectionStoppedBySystem(null)

        assertEquals(1, stoppedCount)
        assertEquals(0, failedCount)
        assertEquals("sess_sys", lastSessionId)
        assertEquals("system_projection_stopped", lastReason)
    }
}
