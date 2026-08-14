package com.tcandt.cloudphone.agent.control

data class PhysicalPoint(
    val x: Float,
    val y: Float
)

object NormalizedCoordinateMapper {
    fun map(
        normalizedX: Float,
        normalizedY: Float,
        widthPx: Int,
        heightPx: Int
    ): PhysicalPoint {
        val clampedX = normalizedX.coerceIn(0f, 1f)
        val clampedY = normalizedY.coerceIn(0f, 1f)

        val xPx = clampedX * (widthPx - 1).coerceAtLeast(1)
        val yPx = clampedY * (heightPx - 1).coerceAtLeast(1)

        return PhysicalPoint(xPx, yPx)
    }
}
