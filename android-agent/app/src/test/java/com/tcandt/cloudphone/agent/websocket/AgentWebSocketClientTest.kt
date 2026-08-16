package com.tcandt.cloudphone.agent.websocket

import android.content.Context
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.mockito.Mockito

class AgentWebSocketClientTest {

    private lateinit var dummyContext: Context
    private lateinit var client: AgentWebSocketClient

    @Before
    fun setUp() {
        val mockPrefs = Mockito.mock(android.content.SharedPreferences::class.java)
        val mockEditor = Mockito.mock(android.content.SharedPreferences.Editor::class.java)
        Mockito.`when`(mockPrefs.getString(Mockito.anyString(), Mockito.anyString())).thenAnswer { invocation ->
            invocation.arguments[1] as? String ?: ""
        }
        Mockito.`when`(mockPrefs.edit()).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.putString(Mockito.anyString(), Mockito.anyString())).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.putLong(Mockito.anyString(), Mockito.anyLong())).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.putInt(Mockito.anyString(), Mockito.anyInt())).thenReturn(mockEditor)

        dummyContext = Mockito.mock(Context::class.java)
        Mockito.`when`(dummyContext.applicationContext).thenReturn(dummyContext)
        Mockito.`when`(dummyContext.getSharedPreferences(Mockito.anyString(), Mockito.anyInt())).thenReturn(mockPrefs)
        Mockito.`when`(dummyContext.packageName).thenReturn("com.tcandt.cloudphone.agent")

        client = AgentWebSocketClient(dummyContext, "wss://localhost:8443", "agent_unit_01")
    }

    @Test
    fun testProductionSocketEpochIncrementingAndStaleCheck() {
        assertEquals(0L, client.currentSocketEpoch)
        assertFalse(client.isStopped)

        // Production connect attempt 1
        client.connect()
        val epoch1 = client.currentSocketEpoch
        assertEquals(1L, epoch1)

        // Production connect attempt 2 (supersedes attempt 1)
        client.connect()
        val epoch2 = client.currentSocketEpoch
        assertEquals(2L, epoch2)

        // Verify stale socket check logic on production instance
        assertTrue("Epoch 1 must be flagged stale after epoch 2", client.isSocketStale(epoch1))
        assertFalse("Epoch 2 must be active and not stale", client.isSocketStale(epoch2))
    }

    @Test
    fun testProductionExplicitDisconnectPreventsReconnect() {
        client.connect()
        val activeEpochBeforeDisconnect = client.currentSocketEpoch
        assertFalse(client.isStopped)

        // Invoke production disconnect()
        client.disconnect()

        assertTrue("isStopped must be true after disconnect()", client.isStopped)
        assertTrue("Previous active socket epoch must now be flagged stale", client.isSocketStale(activeEpochBeforeDisconnect))

        // Subsequent connect() calls must be blocked when explicitly stopped
        client.connect()
        assertEquals("Socket epoch must not increment when connect() is called after disconnect()", activeEpochBeforeDisconnect + 1, client.currentSocketEpoch)
    }
}
