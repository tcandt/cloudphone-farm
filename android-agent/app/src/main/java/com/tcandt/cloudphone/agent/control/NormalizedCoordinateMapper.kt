package com.tcandt.cloudphone.agent.control

data class PhysicalPoint(
    val x: Float,
    val y: Float
)

object NormalizedCoordinateMapper {
    @Throws(IllegalArgumentException::class)
    fun map(
        normalizedX: Float,
        normalizedY: Float,
        widthPx: Int,
        heightPx: Int
    ): PhysicalPoint {
        if (normalizedX.isNaN() || normalizedY.isNaN() ||
            normalizedX < 0f || normalizedX > 1f ||
            normalizedY < 0f || normalizedY > 1f
        ) {
            throw IllegalArgumentException("Invalid normalized coordinates ($normalizedX, $normalizedY): must be finite numbers between 0.0 and 1.0")
        }

        if (widthPx <= 0 || heightPx <= 0) {
            throw IllegalArgumentException("Invalid display geometry: ${widthPx}x${heightPx}")
        }

        val xPx = normalizedX * (widthPx - 1).coerceAtLeast(1)
        val yPx = normalizedY * (heightPx - 1).coerceAtLeast(1)

        return PhysicalPoint(xPx, yPx)
    }
}
