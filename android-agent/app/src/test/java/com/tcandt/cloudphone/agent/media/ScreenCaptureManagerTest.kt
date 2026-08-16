package com.tcandt.cloudphone.agent.media

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ScreenCaptureManagerTest {

    private var startedCount = 0
    private var stoppedCount = 0
    private var failedCount = 0
    private var lastSessionId = ""
    private var lastReason = ""

    @Before
    fun setUp() {
        startedCount = 0
        stoppedCount = 0
        failedCount = 0
        lastSessionId = ""
        lastReason = ""

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
        assertEquals(ScreenCaptureState.FAILED, ScreenCaptureManager.currentState)
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

    @Test
    fun testFgsReadyListenerRegistrationAndCleanup() {
        var listenerInvoked = false
        MediaCaptureServiceNotifier.onFgsReadyListener = {
            listenerInvoked = true
        }

        assertTrue("onFgsReadyListener should be registered", MediaCaptureServiceNotifier.onFgsReadyListener != null)
        MediaCaptureServiceNotifier.onFgsReadyListener?.invoke()
        assertTrue("Listener should have executed", listenerInvoked)

        MediaCaptureServiceNotifier.onFgsReadyListener = null
        assertNull("onFgsReadyListener should be cleared", MediaCaptureServiceNotifier.onFgsReadyListener)
    }
}
