package app.couplelink.mobile

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import android.app.NotificationManager
import android.media.AudioAttributes
import android.media.AudioManager
import android.media.Ringtone
import android.media.RingtoneManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.Vibrator
import android.os.VibratorManager
import android.os.VibrationEffect

class MainActivity : FlutterActivity() {
    private var feedbackChannel: MethodChannel? = null
    private var previewTone: Ringtone? = null
    private val handler = Handler(Looper.getMainLooper())
    private val stopTone = Runnable { stopFeedback() }

    companion object { private const val SOUND_PREVIEW_LIMIT_MS = 1800L }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        feedbackChannel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "app.couplelink/feedback")
        feedbackChannel?.setMethodCallHandler { call, result ->
            when (call.method) {
                "previewVibration" -> {
                    val audio = getSystemService(AUDIO_SERVICE) as AudioManager
                    val notifications = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
                    val vibrator = if (Build.VERSION.SDK_INT >= 31) {
                        (getSystemService(VIBRATOR_MANAGER_SERVICE) as VibratorManager).defaultVibrator
                    } else {
                        @Suppress("DEPRECATION")
                        getSystemService(VIBRATOR_SERVICE) as Vibrator
                    }
                    when {
                        !vibrator.hasVibrator() -> result.success("unsupported")
                        audio.ringerMode == AudioManager.RINGER_MODE_SILENT ||
                            notifications.currentInterruptionFilter != NotificationManager.INTERRUPTION_FILTER_ALL -> result.success("suppressed")
                        else -> try {
                            val attributes = AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_NOTIFICATION).build()
                            if (Build.VERSION.SDK_INT >= 26) {
                                vibrator.vibrate(VibrationEffect.createOneShot(200, VibrationEffect.DEFAULT_AMPLITUDE), attributes)
                            } else {
                                @Suppress("DEPRECATION")
                                vibrator.vibrate(200)
                            }
                            result.success("requested")
                        } catch (error: SecurityException) {
                            result.error("vibration_denied", "System denied vibration", null)
                        }
                    }
                }
                "previewSound" -> {
                    stopFeedback()
                    val audio = getSystemService(AUDIO_SERVICE) as AudioManager
                    val notifications = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
                    if (audio.ringerMode != AudioManager.RINGER_MODE_NORMAL ||
                        audio.getStreamVolume(AudioManager.STREAM_NOTIFICATION) == 0 ||
                        notifications.currentInterruptionFilter != NotificationManager.INTERRUPTION_FILTER_ALL) {
                        result.success("suppressed")
                    } else {
                        val uri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)
                        val tone = if (uri == null) null else RingtoneManager.getRingtone(this, uri)
                        if (tone == null) {
                            result.success("unsupported")
                        } else {
                            try {
                                tone.audioAttributes = AudioAttributes.Builder()
                                    .setUsage(AudioAttributes.USAGE_NOTIFICATION)
                                    .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION).build()
                                if (Build.VERSION.SDK_INT >= 28) tone.isLooping = false
                                previewTone = tone
                                tone.play()
                                handler.postDelayed(stopTone, SOUND_PREVIEW_LIMIT_MS)
                                result.success("requested")
                            } catch (error: SecurityException) {
                                stopFeedback()
                                result.error("sound_denied", "System denied notification sound preview", null)
                            }
                        }
                    }
                }
                "stop" -> { stopFeedback(); result.success(null) }
                else -> result.notImplemented()
            }
        }
    }
    private fun stopFeedback() {
        handler.removeCallbacks(stopTone)
        previewTone?.stop()
        previewTone = null
    }
    override fun onPause() { stopFeedback(); super.onPause() }
    override fun onDestroy() {
        stopFeedback()
        feedbackChannel?.setMethodCallHandler(null)
        super.onDestroy()
    }
}
