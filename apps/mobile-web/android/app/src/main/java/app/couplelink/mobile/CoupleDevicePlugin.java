package app.couplelink.mobile;

import android.Manifest;
import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.provider.Settings;
import androidx.core.content.ContextCompat;
import com.getcapacitor.*;
import com.getcapacitor.annotation.*;

@CapacitorPlugin(name = "CoupleDevice")
public class CoupleDevicePlugin extends Plugin {
    @PluginMethod public void sessionRead(PluginCall call) {
        try { JSObject result = new JSObject(); result.put("value", SessionVault.read(getContext())); call.resolve(result); }
        catch (Exception e) { call.reject("Session storage unavailable", "storage_failed"); }
    }
    @PluginMethod public void sessionWrite(PluginCall call) {
        try { SessionVault.write(getContext(), call.getString("value")); call.resolve(); }
        catch (Exception e) { call.reject("Session save failed", "storage_failed"); }
    }
    @PluginMethod public void request(PluginCall call) {
        getBridge().execute(() -> {
            try { call.resolve(DeviceRuntime.request(call.getString("origin"), call.getString("path"), call.getString("method", "GET"), call.getObject("body"), call.getString("token"))); }
            catch (Exception e) { call.reject("Connection failed", "network_failed"); }
        });
    }
    @PluginMethod public void feedback(PluginCall call) {
        try { JSObject result = new JSObject(); result.put("result", DeviceRuntime.feedback(getContext(), call.getBoolean("sound", false))); call.resolve(result); }
        catch (Exception e) { call.reject("Device feedback failed", "feedback_failed"); }
    }
    @PluginMethod public void notify(PluginCall call) {
        try { DeviceRuntime.notify(getContext(), call.getString("kind", "messages"), call.getString("title", "CoupleLink"), call.getString("body", "")); call.resolve(); }
        catch (SecurityException e) { call.reject("Notifications denied", "notification_denied"); }
        catch (DeviceRuntime.ChannelBlockedException e) { call.reject("Notification category is blocked", "channel_blocked"); }
        catch (Exception e) { call.reject("Notification could not be published", "notification_failed"); }
    }
    @PluginMethod public void interaction(PluginCall call) {
        try { call.resolve(DeviceRuntime.interaction(getContext(), call.getString("action", ""), call.getString("title", "CoupleLink"), call.getString("body", ""), call.getBoolean("allowSound", false), call.getBoolean("allowVibrate", false), call.getLong("expiresAt", 0L))); }
        catch (Exception e) { call.reject("Interaction feedback could not be delivered", "feedback_failed"); }
    }
    @PluginMethod public void downloadAndInstall(PluginCall call) {
        String url = call.getString("url");
        if (url == null || !(url.startsWith("http://") || url.startsWith("https://"))) { call.reject("invalid_url"); return; }
        Context context = getContext();
        if (Build.VERSION.SDK_INT >= 26 && !context.getPackageManager().canRequestPackageInstalls()) {
            // The user must allow this app to install packages before anything else.
            try {
                Intent settings = new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, android.net.Uri.parse("package:" + context.getPackageName()));
                settings.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                context.startActivity(settings);
            } catch (Exception ignored) { }
            call.reject("install_permission_required");
            return;
        }
        JSObject started = new JSObject(); started.put("started", true); call.resolve(started);
        new Thread(() -> {
            String expected = call.getString("sha256", "");
            java.io.File target = new java.io.File(context.getCacheDir(), "update.apk");
            try {
                if (target.isFile() && !expected.isEmpty() && expected.equalsIgnoreCase(sha256(target))) {
                    // Already downloaded and verified: install without downloading again.
                } else {
                    java.io.File temp = new java.io.File(context.getCacheDir(), "update.apk.part");
                    try (java.io.InputStream input = new java.net.URL(url).openStream(); java.io.OutputStream output = new java.io.FileOutputStream(temp)) {
                        byte[] buffer = new byte[64 * 1024];
                        int read;
                        while ((read = input.read(buffer)) > 0) output.write(buffer, 0, read);
                    }
                    if (!expected.isEmpty() && !expected.equalsIgnoreCase(sha256(temp))) { temp.delete(); return; }
                    if (target.exists() && !target.delete()) { temp.delete(); return; }
                    if (!temp.renameTo(target)) { temp.delete(); return; }
                }
            } catch (Exception error) {
                return;
            }
            android.net.Uri apk;
            try {
                apk = androidx.core.content.FileProvider.getUriForFile(context, context.getPackageName() + ".fileprovider", target);
            } catch (Exception error) {
                return;
            }
            Intent install = new Intent(Intent.ACTION_VIEW)
                .setDataAndType(apk, "application/vnd.android.package-archive")
                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);
            try { context.startActivity(install); } catch (Exception ignored) { }
        }).start();
    }
    @PluginMethod public void openUrl(PluginCall call) {
        String url = call.getString("url");
        if (url == null || !(url.startsWith("http://") || url.startsWith("https://"))) { call.reject("invalid_url"); return; }
        // Hand the link to the system browser so a packaged update really downloads.
        try {
            Intent intent = new Intent(Intent.ACTION_VIEW, android.net.Uri.parse(url));
            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            getContext().startActivity(intent);
            call.resolve();
        } catch (Exception error) { call.reject("open_failed"); }
    }
    @PluginMethod public void diagnostics(PluginCall call) {
        try { DeviceRuntime.ensureChannels(getContext()); call.resolve(DeviceRuntime.diagnostics(getContext())); }
        catch (Exception e) { call.reject("Notification diagnostics unavailable", "notification_failed"); }
    }
    @PluginMethod public void permission(PluginCall call) {
        try { DeviceRuntime.ensureChannels(getContext()); }
        catch (Exception e) { call.reject("Notification categories unavailable", "notification_failed"); return; }
        ((MainActivity) getActivity()).requestNotifications(call);
    }
    @PluginMethod public void settings(PluginCall call) {
        try {
            String channel = call.getString("channel", "");
            Intent intent = new Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, getContext().getPackageName());
            if (!channel.isEmpty()) {
                if (!InteractionAlert.CHANNEL.equals(channel)) { call.reject("Unknown notification category", "invalid_input"); return; }
                DeviceRuntime.ensureChannels(getContext());
                if (Build.VERSION.SDK_INT >= 26) intent = new Intent(Settings.ACTION_CHANNEL_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, getContext().getPackageName()).putExtra(Settings.EXTRA_CHANNEL_ID, channel);
            }
            getActivity().startActivity(intent); call.resolve();
        }
        catch (Exception e) { call.reject("Cannot open notification settings", "settings_failed"); }
    }
    @PluginMethod public void status(PluginCall call) { JSObject result = new JSObject(); result.put("running", ReceptionService.running); result.put("notificationsEnabled", DeviceRuntime.allowed(getContext())); call.resolve(result); }
    @PluginMethod public void launch(PluginCall call) { JSObject result = new JSObject(); result.put("kind", getActivity().getIntent().getStringExtra("couplelink.kind")); getActivity().getIntent().removeExtra("couplelink.kind"); call.resolve(result); }
    @PluginMethod public void exitApp(PluginCall call) {
        // Capacitor runs plugin methods on its worker thread; finish() belongs on the UI thread.
        getBridge().executeOnMainThread(() -> { Activity activity = getActivity(); if (activity != null) activity.finish(); });
        call.resolve();
    }
    @PluginMethod public void baseline(PluginCall call) {
        try {
            synchronized (ReceptionService.class) {
            var prefs = getContext().getSharedPreferences(DeviceRuntime.PREFS, 0); String saved = prefs.getString("config", null);
            if (saved != null) {
                var config = new org.json.JSONObject(saved);
                if (config.optString("origin").equals(call.getString("origin")) && config.optString("user").equals(call.getString("user")) && config.optString("couple").equals(call.getString("couple"))) {
                    long cursor = Math.max(config.optLong("cursor"), call.getLong("cursor", 0L)); config.put("cursor", cursor);
                    if (!prefs.edit().putString("config", config.toString()).commit()) throw new IllegalStateException("Save failed");
                }
            }
            }
            call.resolve();
        } catch (Exception e) { call.reject("Background cursor save failed", "storage_failed"); }
    }
    @PluginMethod public void background(PluginCall call) {
        try {
            var prefs = getContext().getSharedPreferences(DeviceRuntime.PREFS, 0);
            if (!call.getBoolean("enabled", false)) {
                if (!prefs.edit().clear().commit()) throw new IllegalStateException("Save failed");
                getContext().stopService(new Intent(getContext(), ReceptionService.class)); call.resolve(); return;
            }
            if (!DeviceRuntime.allowed(getContext())) { call.reject("Allow system notifications first", "notification_denied"); return; }
            if (!DeviceRuntime.foreground) { call.reject("Open the app to enable background reception", "foreground_required"); return; }
            DeviceRuntime.origin(call.getString("origin"));
            if (!prefs.edit().putString("config", call.getData().toString()).commit()) throw new IllegalStateException("Save failed");
            ContextCompat.startForegroundService(getContext(), new Intent(getContext(), ReceptionService.class)); call.resolve();
        } catch (Exception e) { call.reject("Background reception could not start", "background_failed"); }
    }

    /** SHA-256 of a downloaded file, lower-case hex. */
    private static String sha256(java.io.File file) throws Exception {
        java.security.MessageDigest digest = java.security.MessageDigest.getInstance("SHA-256");
        try (java.io.InputStream input = new java.io.FileInputStream(file)) {
            byte[] buffer = new byte[64 * 1024];
            int read;
            while ((read = input.read(buffer)) > 0) digest.update(buffer, 0, read);
        }
        StringBuilder hex = new StringBuilder();
        for (byte value : digest.digest()) hex.append(String.format("%02x", value));
        return hex.toString();
    }

    /** Downloads one photo and returns it as base64 so the WebView can render it. */
    @PluginMethod public void fetchImage(PluginCall call) {
        String origin = call.getString("origin");
        String id = call.getString("id");
        String token = call.getString("token", "");
        if (origin == null || id == null || !(origin.startsWith("http://") || origin.startsWith("https://"))) {
            call.reject("invalid_input");
            return;
        }
        try {
            java.net.URL url = new java.net.URL(origin + "/images/" + id + "?token=" + java.net.URLEncoder.encode(token, "UTF-8"));
            java.net.HttpURLConnection connection = (java.net.HttpURLConnection) url.openConnection();
            connection.setConnectTimeout(15000);
            connection.setReadTimeout(20000);
            connection.setRequestMethod("GET");
            int status = connection.getResponseCode();
            if (status != 200) { call.reject("image_failed"); return; }
            String type = connection.getContentType();
            java.io.ByteArrayOutputStream buffer = new java.io.ByteArrayOutputStream();
            try (java.io.InputStream input = connection.getInputStream()) {
                byte[] chunk = new byte[64 * 1024];
                int read;
                while ((read = input.read(chunk)) > 0) buffer.write(chunk, 0, read);
            }
            JSObject result = new JSObject();
            result.put("contentType", type == null ? "image/jpeg" : type);
            result.put("data", android.util.Base64.encodeToString(buffer.toByteArray(), android.util.Base64.NO_WRAP));
            call.resolve(result);
        } catch (Exception error) {
            call.reject("image_failed");
        }
    }
}
