package com.tcandt.cloudphone.agent.media

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ScreenCaptureManagerTest {

    @Before
    fun setUp() {
        ScreenCaptureManager.isFgsRunning = false
        MediaCaptureServiceNotifier.onFgsReadyListener = null
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

    @Test
    fun testFgsRunningFlagTrackedUnconditionally() {
        ScreenCaptureManager.isFgsRunning = true
        assertTrue("isFgsRunning should be true", ScreenCaptureManager.isFgsRunning)
    }
}
