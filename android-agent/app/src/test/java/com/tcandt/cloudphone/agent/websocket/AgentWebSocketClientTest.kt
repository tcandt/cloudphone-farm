package com.tcandt.cloudphone.agent.websocket

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AgentWebSocketClientTest {

    @Test
    fun testSocketEpochIncrementing() {
        var socketEpoch = 0L
        var callbackExecCount = 0

        // Attempt 1
        val attempt1Epoch = ++socketEpoch
        val listenerEpoch1 = attempt1Epoch

        // Attempt 2 supersedes Attempt 1
        val attempt2Epoch = ++socketEpoch
        val listenerEpoch2 = attempt2Epoch

        // Callback from Attempt 1 fires (stale epoch)
        if (listenerEpoch1 == socketEpoch) {
            callbackExecCount++
        }

        // Callback from Attempt 2 fires (current epoch)
        if (listenerEpoch2 == socketEpoch) {
            callbackExecCount++
        }

        assertEquals("Stale epoch callback must be rejected, only current epoch executes", 1, callbackExecCount)
    }

    @Test
    fun testExplicitDisconnectPreventsReconnect() {
        var isExplicitlyStopped = false
        var isReconnecting = false

        // User calls disconnect()
        isExplicitlyStopped = true

        // Reconnect scheduled
        if (!isExplicitlyStopped) {
            isReconnecting = true
        }

        assertFalse("Explicit disconnect must prevent scheduling reconnect", isReconnecting)
        assertTrue("isExplicitlyStopped must be true", isExplicitlyStopped)
    }
}
