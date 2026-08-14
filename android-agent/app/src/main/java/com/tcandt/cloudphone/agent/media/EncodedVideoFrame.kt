package com.tcandt.cloudphone.agent.media

import android.media.MediaFormat

data class EncodedVideoFrame(
    val data: ByteArray,
    val ptsUs: Long,
    val isKeyFrame: Boolean,
    val isCodecConfig: Boolean
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false
        other as EncodedVideoFrame
        return data.contentEquals(other.data) && ptsUs == other.ptsUs && isKeyFrame == other.isKeyFrame && isCodecConfig == other.isCodecConfig
    }

    override fun hashCode(): Int {
        var result = data.contentHashCode()
        result = 31 * result + ptsUs.hashCode()
        result = 31 * result + isKeyFrame.hashCode()
        result = 31 * result + isCodecConfig.hashCode()
        return result
    }
}

interface EncodedVideoSink {
    fun onFormatChanged(format: MediaFormat)
    fun onEncodedFrame(frame: EncodedVideoFrame)
}
