package com.tcandt.cloudphone.agent

import com.tcandt.cloudphone.agent.control.NormalizedCoordinateMapper
import org.junit.Assert.assertEquals
import org.junit.Test

class NormalizedCoordinateMapperTest {

    @Test
    fun testTopLeftMapping() {
        val point = NormalizedCoordinateMapper.map(0f, 0f, 720, 1280)
        assertEquals(0f, point.x, 0.001f)
        assertEquals(0f, point.y, 0.001f)
    }

    @Test
    fun testBottomRightMapping() {
        val point = NormalizedCoordinateMapper.map(1f, 1f, 720, 1280)
        assertEquals(719f, point.x, 0.001f)
        assertEquals(1279f, point.y, 0.001f)
    }

    @Test
    fun testCenterMapping() {
        val point = NormalizedCoordinateMapper.map(0.5f, 0.5f, 1440, 2560)
        assertEquals(719.5f, point.x, 0.5f)
        assertEquals(1279.5f, point.y, 0.5f)
    }

    @Test
    fun testLandscapeMapping() {
        val point = NormalizedCoordinateMapper.map(0.5f, 0.5f, 1280, 720)
        assertEquals(639.5f, point.x, 0.5f)
        assertEquals(359.5f, point.y, 0.5f)
    }

    @Test(expected = IllegalArgumentException::class)
    fun testNegativeXRejected() {
        NormalizedCoordinateMapper.map(-0.1f, 0.5f, 720, 1280)
    }

    @Test(expected = IllegalArgumentException::class)
    fun testOverboundXRejected() {
        NormalizedCoordinateMapper.map(1.001f, 0.5f, 720, 1280)
    }

    @Test(expected = IllegalArgumentException::class)
    fun testNegativeYRejected() {
        NormalizedCoordinateMapper.map(0.5f, -0.05f, 720, 1280)
    }

    @Test(expected = IllegalArgumentException::class)
    fun testOverboundYRejected() {
        NormalizedCoordinateMapper.map(0.5f, 1.2f, 720, 1280)
    }

    @Test(expected = IllegalArgumentException::class)
    fun testZeroDisplayDimensionRejected() {
        NormalizedCoordinateMapper.map(0.5f, 0.5f, 0, 1280)
    }
}
