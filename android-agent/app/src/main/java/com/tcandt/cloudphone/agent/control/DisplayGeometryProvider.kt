package com.tcandt.cloudphone.agent.control

import android.content.Context
import android.content.res.Configuration
import android.graphics.Point
import android.os.Build
import android.view.WindowManager

enum class DisplayOrientation {
    PORTRAIT,
    LANDSCAPE
}

data class DisplayGeometry(
    val widthPx: Int,
    val heightPx: Int,
    val orientation: DisplayOrientation
)

object DisplayGeometryProvider {
    @Throws(IllegalStateException::class)
    fun getGeometry(context: Context): DisplayGeometry {
        val wm = context.getSystemService(Context.WINDOW_SERVICE) as? WindowManager
            ?: throw IllegalStateException("WindowManager service is not available")

        var widthPx = 0
        var heightPx = 0

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            val metrics = wm.currentWindowMetrics
            val bounds = metrics.bounds
            widthPx = bounds.width()
            heightPx = bounds.height()
        } else {
            @Suppress("DEPRECATION")
            val display = wm.defaultDisplay ?: throw IllegalStateException("Default display is null")
            val realSize = Point()
            @Suppress("DEPRECATION")
            display.getRealSize(realSize)
            widthPx = realSize.x
            heightPx = realSize.y
        }

        if (widthPx <= 0 || heightPx <= 0) {
            throw IllegalStateException("Invalid display dimensions (${widthPx}x${heightPx})")
        }

        val configOrientation = context.resources.configuration.orientation
        val orientation = if (configOrientation == Configuration.ORIENTATION_LANDSCAPE || widthPx > heightPx) {
            DisplayOrientation.LANDSCAPE
        } else {
            DisplayOrientation.PORTRAIT
        }

        return DisplayGeometry(widthPx, heightPx, orientation)
    }
}
