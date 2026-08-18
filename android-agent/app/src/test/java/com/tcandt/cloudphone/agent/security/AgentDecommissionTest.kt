package com.tcandt.cloudphone.agent.security

import android.content.Context
import android.content.SharedPreferences
import com.tcandt.cloudphone.agent.websocket.AgentWebSocketClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.mockito.Mockito

class AgentDecommissionTest {

    private lateinit var mockContext: Context
    private lateinit var mockPrefs: SharedPreferences
    private lateinit var mockEditor: SharedPreferences.Editor

    @Before
    fun setUp() {
        mockPrefs = Mockito.mock(SharedPreferences::class.java)
        mockEditor = Mockito.mock(SharedPreferences.Editor::class.java)

        Mockito.`when`(mockPrefs.getString(Mockito.any(), Mockito.any())).thenAnswer { invocation ->
            invocation.arguments[1]
        }
        Mockito.`when`(mockPrefs.edit()).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.remove(Mockito.anyString())).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.putString(Mockito.any(), Mockito.any())).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.putLong(Mockito.any(), Mockito.anyLong())).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.putInt(Mockito.any(), Mockito.anyInt())).thenReturn(mockEditor)
        Mockito.`when`(mockEditor.commit()).thenReturn(true)

        mockContext = Mockito.mock(Context::class.java)
        Mockito.`when`(mockContext.applicationContext).thenReturn(mockContext)
        Mockito.`when`(mockContext.getSharedPreferences(Mockito.any(), Mockito.anyInt())).thenReturn(mockPrefs)
        Mockito.`when`(mockContext.packageName).thenReturn("com.tcandt.cloudphone.agent")
    }

    @Test
    fun testAgentKeyStoreDeleteKeysSurgicallyRemovesIdentityKeys() {
        val keyStore = AgentKeyStore(mockContext)
        keyStore.deleteKeys()

        // Verify surgical deletion of the 5 identity keys from SharedPreferences
        Mockito.verify(mockEditor).remove("agent_encrypted_seed_b64")
        Mockito.verify(mockEditor).remove("agent_iv_b64")
        Mockito.verify(mockEditor).remove("agent_public_raw_b64")
        Mockito.verify(mockEditor).remove("agent_fingerprint")
        Mockito.verify(mockEditor).remove("agent_private_raw_b64")
        Mockito.verify(mockEditor).commit()
    }

    @Test
    fun testDecommissionAndDisconnectTerminatesWebSocketClient() {
        val client = AgentWebSocketClient(mockContext, "wss://localhost:8443", "agt_test_001")
        assertFalse("Client should initially not be stopped", client.isStopped)

        client.connect()
        val epochBefore = client.currentSocketEpoch

        client.decommissionAndDisconnect()

        assertTrue("Client must be explicitly stopped after decommission", client.isStopped)
        assertTrue("Epoch must be incremented and flagged stale", client.isSocketStale(epochBefore))
    }
}
