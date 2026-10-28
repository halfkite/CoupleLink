package app.couplelink.mobile;

import android.Manifest;
import android.app.*;
import android.content.*;
import android.content.pm.PackageManager;
import android.media.*;
import android.os.*;
import android.app.AppOpsManager;
import androidx.core.app.NotificationCompat;
import androidx.core.app.NotificationManagerCompat;
import androidx.core.content.ContextCompat;
import com.getcapacitor.JSObject;
import org.json.JSONObject;
import java.net.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;

final class DeviceRuntime {
    static final class ChannelBlockedException extends Exception {}
    static volatile boolean foreground = false;
    static final String PREFS = "couplelink.web.background.v1";
    static final long VIBRATION_DURATION_MS = 200;
    private static Ringtone ringtone;
    private static final Handler handler = new Handler(Looper.getMainLooper());
    static String origin(String value) throws Exception {
        URI uri = new URI(value);
        String scheme = uri.getScheme(), host = uri.getHost();
        if (host == null || uri.getUserInfo() != null || uri.getRawQuery() != null || uri.getRawFragment() != null || (uri.getPath() != null && !uri.getPath().isEmpty()) || !("https".equals(scheme) || "http".equals(scheme))) throw new IOException("Invalid origin");
        if ("http".equals(scheme) && !Arrays.asList("localhost", "127.0.0.1", "10.0.2.2", "10.74.205.253", "192.168.137.1").contains(host)) throw new IOException("HTTP host not enabled");
        return uri.toString();
    }
    static JSObject request(String origin, String path, String method, JSONObject body, String token) throws Exception {
        origin(origin);
        if (path == null || !path.startsWith("/") || path.startsWith("//") || path.contains("://") || path.contains("\\") || path.contains("#")) throw new IOException("Invalid path");
        if (!Arrays.asList("GET", "POST", "DELETE", "PUT").contains(method)) throw new IOException("Invalid method");
        HttpURLConnection connection = (HttpURLConnection) new URL(origin + path).openConnection();
        try {
            connection.setInstanceFollowRedirects(false); connection.setConnectTimeout(8000); connection.setReadTimeout(10000);
            connection.setRequestMethod(method); connection.setRequestProperty("Content-Type", "application/json");
            if (token != null) connection.setRequestProperty("Authorization", "Bearer " + token);
            if (body != null) { connection.setDoOutput(true); try (OutputStream out = connection.getOutputStream()) { out.write(body.toString().getBytes(StandardCharsets.UTF_8)); } }
            int status = connection.getResponseCode();
            if (status >= 300 && status < 400) throw new IOException("Redirect refused");
            InputStream input = status >= 400 ? connection.getErrorStream() : connection.getInputStream();
            if (input == null) throw new IOException("Missing response");
            ByteArrayOutputStream bytes = new ByteArrayOutputStream();
            try (input) { byte[] buffer = new byte[8192]; int n; while ((n = input.read(buffer)) != -1) { if (bytes.size() + n > 2 * 1024 * 1024) throw new IOException("Response too large"); bytes.write(buffer, 0, n); } }
            JSObject result = new JSObject(); result.put("status", status); result.put("data", new JSObject(new String(bytes.toByteArray(), StandardCharsets.UTF_8))); return result;
        } finally { connection.disconnect(); }
    }
    static boolean allowed(Context context) { return NotificationManagerCompat.from(context).areNotificationsEnabled() && (Build.VERSION.SDK_INT < 33 || ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED); }
    private static final String[] LEGACY_CHANNELS = {
        "couplelink_messages_v1", "couplelink_interactions_quiet_v1",
        "couplelink_interaction_vibrate_v1", "couplelink_interaction_sound_v1"
    };
    static void ensureChannels(Context context) {
        if (Build.VERSION.SDK_INT < 26) return;
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager.getNotificationChannel(InteractionAlert.CHANNEL) == null) {
            int importance = NotificationManager.IMPORTANCE_HIGH;
            int visibility = NotificationCompat.VISIBILITY_PRIVATE;
            for (String id : LEGACY_CHANNELS) {
                NotificationChannel old = manager.getNotificationChannel(id);
                if (old != null) {
                    importance = Math.min(importance, old.getImportance());
                    if (old.getLockscreenVisibility() >= Notification.VISIBILITY_SECRET && old.getLockscreenVisibility() <= Notification.VISIBILITY_PUBLIC)
                        visibility = Math.min(visibility, old.getLockscreenVisibility());
                }
            }
            NotificationChannel messages = manager.getNotificationChannel(LEGACY_CHANNELS[0]);
            NotificationChannel sound = manager.getNotificationChannel(LEGACY_CHANNELS[3]);
            NotificationChannel vibration = manager.getNotificationChannel(LEGACY_CHANNELS[2]);
            if (sound == null) sound = messages;
            if (vibration == null) vibration = messages;
            NotificationChannel channel = new NotificationChannel(InteractionAlert.CHANNEL, context.getString(R.string.channel_alerts), importance);
            channel.setSound(sound == null ? RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION) : sound.getSound(),
                sound == null ? new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_NOTIFICATION).build() : sound.getAudioAttributes());
            channel.enableVibration(vibration == null || vibration.shouldVibrate());
            if (vibration != null && vibration.getVibrationPattern() != null) channel.setVibrationPattern(vibration.getVibrationPattern());
            channel.setLockscreenVisibility(visibility); channel.setShowBadge(true);
            manager.createNotificationChannel(channel);
        }
        if (manager.getNotificationChannel(InteractionAlert.CHANNEL) == null) throw new IllegalStateException("Notification channel missing");
        for (String id : LEGACY_CHANNELS) manager.deleteNotificationChannel(id);
        if (!ReceptionService.running) manager.deleteNotificationChannel("couplelink_connection_v1");
    }
    static Notification notification(Context context, String kind, String title, String body, boolean ongoing) {
        return notification(context, kind, title, body, ongoing, ongoing);
    }
    private static Notification notification(Context context, String kind, String title, String body, boolean ongoing, boolean silent) {
        ensureChannels(context);
        Intent launch = new Intent(context, MainActivity.class).putExtra("couplelink.kind", kind).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        PendingIntent tap = PendingIntent.getActivity(context, ongoing ? 10 : kind.equals("interactions") ? 11 : 12, launch, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        NotificationCompat.Builder builder = new NotificationCompat.Builder(context, InteractionAlert.CHANNEL).setSmallIcon(R.drawable.ic_couplelink_notification).setContentTitle(title).setContentText(body)
                .setPriority(ongoing ? NotificationCompat.PRIORITY_LOW : NotificationCompat.PRIORITY_HIGH).setCategory(ongoing ? NotificationCompat.CATEGORY_SERVICE : NotificationCompat.CATEGORY_MESSAGE)
                .setVisibility(NotificationCompat.VISIBILITY_PRIVATE).setContentIntent(tap).setAutoCancel(!ongoing).setOngoing(ongoing).setSilent(silent).setOnlyAlertOnce(ongoing);
        if (Build.VERSION.SDK_INT < 26 && !silent) {
            builder.setSound(RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION));
            builder.setVibrate(new long[]{0, VIBRATION_DURATION_MS});
        }
        return builder.build();
    }
    static JSObject interaction(Context context, String action, String title, String body, boolean allowSound, boolean allowVibrate, long expiresAt) throws Exception {
        long remaining = expiresAt - System.currentTimeMillis();
        InteractionAlert policy = InteractionAlert.of(action, allowSound, allowVibrate, remaining > 0 && remaining <= (action.equals("ring") ? 30_000 : 120_000));
        JSObject response = new JSObject();
        if (!allowed(context)) {
            response.put("result", "suppressed"); response.put("notificationError", "notification_denied"); return response;
        }
        Notification notification = notification(context, "interactions", title, body, false, !policy.sound && !policy.vibrate);
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        boolean effective = policy.sound || policy.vibrate;
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel actual = manager.getNotificationChannel(InteractionAlert.CHANNEL);
            if (actual == null || actual.getImportance() == NotificationManager.IMPORTANCE_NONE) {
                response.put("notificationError", "channel_blocked"); response.put("result", "suppressed"); return response;
            }
            effective = effective && (actual.getSound() != null || actual.shouldVibrate());
        }
        manager.notify(953, notification);
        AudioManager audio = context.getSystemService(AudioManager.class);
        int filter = manager.getCurrentInterruptionFilter();
        if (audio.getRingerMode() == AudioManager.RINGER_MODE_SILENT || filter == NotificationManager.INTERRUPTION_FILTER_NONE || filter == NotificationManager.INTERRUPTION_FILTER_ALARMS || filter == NotificationManager.INTERRUPTION_FILTER_PRIORITY) effective = false;
        response.put("result", effective ? "requested" : "suppressed");
        return response;
    }
    static void notify(Context context, String kind, String title, String body) throws Exception {
        if (!allowed(context)) throw new SecurityException("Notifications denied");
        Notification notification = notification(context, kind, title, body, false);
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (Build.VERSION.SDK_INT >= 26 && manager.getNotificationChannel(InteractionAlert.CHANNEL).getImportance() == NotificationManager.IMPORTANCE_NONE) throw new ChannelBlockedException();
        manager.notify(kind.equals("interactions") ? 953 : kind.equals("test") ? 954 : kind.equals("anniversary") ? 955 : 952, notification);
    }
    static String feedback(Context context, boolean sound) throws Exception {
        if (!foreground) return "foreground_required";
        AudioManager audio = context.getSystemService(AudioManager.class);
        NotificationManager notifications = context.getSystemService(NotificationManager.class);
        int filter = notifications.getCurrentInterruptionFilter();
        if (sound) {
            if (filter == NotificationManager.INTERRUPTION_FILTER_NONE || filter == NotificationManager.INTERRUPTION_FILTER_ALARMS || filter == NotificationManager.INTERRUPTION_FILTER_PRIORITY) return "suppressed";
            if (audio.getRingerMode() != AudioManager.RINGER_MODE_NORMAL || audio.getStreamVolume(AudioManager.STREAM_NOTIFICATION) == 0) return "suppressed";
            var uri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION);
            if (uri == null) return "unsupported";
            stopSound(); ringtone = RingtoneManager.getRingtone(context, uri); if (ringtone == null) return "unsupported";
            ringtone.setAudioAttributes(new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_NOTIFICATION).build());
            if (Build.VERSION.SDK_INT >= 28) ringtone.setLooping(false);
            ringtone.play(); handler.postDelayed(DeviceRuntime::stopSound, 1800); return "requested";
        }
        Vibrator vibrator = vibrator(context);
        if (vibrator == null || !vibrator.hasVibrator()) return "unsupported";
        if ("blocked".equals(vibrationOperation(context))) return "suppressed";
        // This is an explicit foreground button press, not an incoming alert.
        // Let Android apply touch-feedback settings instead of notification settings.
        if (Build.VERSION.SDK_INT >= 33) vibrator.vibrate(
            VibrationEffect.createOneShot(VIBRATION_DURATION_MS, VibrationEffect.DEFAULT_AMPLITUDE),
            new VibrationAttributes.Builder().setUsage(VibrationAttributes.USAGE_TOUCH).build());
        else {
            AudioAttributes attributes = new AudioAttributes.Builder()
                .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                .setUsage(AudioAttributes.USAGE_ASSISTANCE_SONIFICATION).build();
            if (Build.VERSION.SDK_INT >= 26) vibrator.vibrate(VibrationEffect.createOneShot(VIBRATION_DURATION_MS, VibrationEffect.DEFAULT_AMPLITUDE), attributes);
            else vibrator.vibrate(VIBRATION_DURATION_MS, attributes);
        }
        return "requested";
    }
    static JSObject diagnostics(Context context) {
        JSObject result = new JSObject();
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        Vibrator vibrator = vibrator(context);
        result.put("sdk", Build.VERSION.SDK_INT); result.put("manufacturer", Build.MANUFACTURER); result.put("model", Build.MODEL);
        result.put("notificationsEnabled", allowed(context)); result.put("hasVibrator", vibrator != null && vibrator.hasVibrator());
        result.put("ringerMode", context.getSystemService(AudioManager.class).getRingerMode()); result.put("interruptionFilter", manager.getCurrentInterruptionFilter());
        result.put("running", ReceptionService.running);
        result.put("vibrationOperation", vibrationOperation(context));
        org.json.JSONArray channels = new org.json.JSONArray();
        if (Build.VERSION.SDK_INT >= 26) for (String id : new String[]{InteractionAlert.CHANNEL}) {
            NotificationChannel channel = manager.getNotificationChannel(id);
            JSObject entry = new JSObject(); entry.put("id", id); entry.put("exists", channel != null);
            if (channel != null) { entry.put("importance", channel.getImportance()); entry.put("sound", channel.getSound() != null); entry.put("vibration", channel.shouldVibrate()); }
            channels.put(entry);
        }
        result.put("channels", channels); return result;
    }
    private static Vibrator vibrator(Context context) {
        if (Build.VERSION.SDK_INT >= 31) {
            VibratorManager manager = context.getSystemService(VibratorManager.class);
            if (manager != null) return manager.getDefaultVibrator();
        }
        return context.getSystemService(Vibrator.class);
    }
    private static String vibrationOperation(Context context) {
        try {
            String operation = AppOpsManager.permissionToOp(Manifest.permission.VIBRATE);
            if (operation == null) return "unknown";
            int mode = context.getSystemService(AppOpsManager.class).checkOpNoThrow(operation, android.os.Process.myUid(), context.getPackageName());
            if (mode == AppOpsManager.MODE_ALLOWED || mode == AppOpsManager.MODE_DEFAULT) return "allowed";
            return mode == AppOpsManager.MODE_IGNORED || mode == AppOpsManager.MODE_ERRORED ? "blocked" : "unknown";
        } catch (SecurityException e) { return "unknown"; }
    }
    static synchronized void stopSound() { handler.removeCallbacksAndMessages(null); if (ringtone != null) { ringtone.stop(); ringtone = null; } }
}
