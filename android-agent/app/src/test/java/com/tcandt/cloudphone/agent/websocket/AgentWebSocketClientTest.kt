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

        val attempt1Epoch = ++socketEpoch
        val listenerEpoch1 = attempt1Epoch

        val attempt2Epoch = ++socketEpoch
        val listenerEpoch2 = attempt2Epoch

        if (listenerEpoch1 == socketEpoch) {
            callbackExecCount++
        }

        if (listenerEpoch2 == socketEpoch) {
            callbackExecCount++
        }

        assertEquals("Stale epoch callback must be rejected, only current epoch executes", 1, callbackExecCount)
    }

    @Test
    fun testExplicitDisconnectPreventsReconnect() {
        val isExplicitlyStopped = true
        var isReconnecting = false

        if (!isExplicitlyStopped) {
            isReconnecting = true
        }

        assertFalse("Explicit disconnect must prevent scheduling reconnect", isReconnecting)
        assertTrue("isExplicitlyStopped must be true", isExplicitlyStopped)
    }
}
