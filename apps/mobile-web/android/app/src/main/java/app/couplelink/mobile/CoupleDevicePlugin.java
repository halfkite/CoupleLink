package app.couplelink.mobile;

import android.Manifest;
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
}
