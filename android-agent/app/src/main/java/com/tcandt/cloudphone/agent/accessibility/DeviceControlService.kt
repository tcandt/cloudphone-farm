package com.tcandt.cloudphone.agent.accessibility

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.os.Bundle
import android.util.Log
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo

class DeviceControlService : AccessibilityService() {

    companion object {
        private const val TAG = "DeviceControlService"
        var instance: DeviceControlService? = null
            private set
    }

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        Log.i(TAG, "DeviceControlService connected successfully")
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        // Event processing if needed
    }

    override fun onInterrupt() {
        Log.w(TAG, "DeviceControlService interrupted")
    }

    override fun onDestroy() {
        super.onDestroy()
        instance = null
    }

    /**
     * Physical Touch Gesture Dispatch
     */
    fun performTouch(x: Float, y: Float, callback: (Boolean) -> Unit) {
        val path = Path().apply {
            moveTo(x, y)
        }
        val stroke = GestureDescription.StrokeDescription(path, 0, 50)
        val gesture = GestureDescription.Builder().addStroke(stroke).build()

        dispatchGesture(gesture, object : GestureResultCallback() {
            override fun onCompleted(gestureDescription: GestureDescription?) {
                Log.d(TAG, "Touch gesture succeeded at ($x, $y)")
                callback(true)
            }

            override fun onCancelled(gestureDescription: GestureDescription?) {
                Log.e(TAG, "Touch gesture cancelled at ($x, $y)")
                callback(false)
            }
        }, null)
    }

    /**
     * Physical Swipe Gesture Dispatch
     */
    fun performSwipe(startX: Float, startY: Float, endX: Float, endY: Float, durationMs: Long, callback: (Boolean) -> Unit) {
        val path = Path().apply {
            moveTo(startX, startY)
            lineTo(endX, endY)
        }
        val stroke = GestureDescription.StrokeDescription(path, 0, durationMs.coerceIn(50, 5000))
        val gesture = GestureDescription.Builder().addStroke(stroke).build()

        dispatchGesture(gesture, object : GestureResultCallback() {
            override fun onCompleted(gestureDescription: GestureDescription?) {
                Log.d(TAG, "Swipe gesture succeeded from ($startX, $startY) to ($endX, $endY)")
                callback(true)
            }

            override fun onCancelled(gestureDescription: GestureDescription?) {
                Log.e(TAG, "Swipe gesture cancelled")
                callback(false)
            }
        }, null)
    }

    /**
     * Physical Text Input Dispatch to Focused Node
     */
    fun performTextInput(text: String): Boolean {
        val rootNode = rootInActiveWindow ?: return false
        val focusedNode = rootNode.findFocus(AccessibilityNodeInfo.FOCUS_INPUT) ?: return false

        val arguments = Bundle().apply {
            putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, text)
        }
        val result = focusedNode.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, arguments)
        focusedNode.recycle()
        rootNode.recycle()
        return result
    }

    /**
     * Physical Navigation Keycode Dispatch
     */
    fun performNavigation(actionType: String): Boolean {
        return when (actionType) {
            "global.back" -> performGlobalAction(GLOBAL_ACTION_BACK)
            "global.home" -> performGlobalAction(GLOBAL_ACTION_HOME)
            "global.recents" -> performGlobalAction(GLOBAL_ACTION_RECENTS)
            else -> false
        }
    }
}
