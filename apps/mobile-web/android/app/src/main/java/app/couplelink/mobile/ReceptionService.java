package app.couplelink.mobile;

import android.app.*;
import android.content.*;
import android.content.pm.ServiceInfo;
import android.os.*;
import org.json.*;
import java.util.concurrent.*;

public class ReceptionService extends Service {
    static volatile boolean running = false;
    private ScheduledExecutorService worker;
    private boolean disconnected = false;
    @Override public void onCreate() {
        super.onCreate();
        var notification = DeviceRuntime.notification(this, "connection", "CoupleLink", getString(R.string.background_enabled), true);
        if (Build.VERSION.SDK_INT >= 29) startForeground(951, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC); else startForeground(951, notification);
        running = true; worker = Executors.newSingleThreadScheduledExecutor(); worker.scheduleWithFixedDelay(this::poll, 0, 5, TimeUnit.SECONDS);
    }
    @Override public int onStartCommand(Intent intent, int flags, int startId) { return START_NOT_STICKY; }
    private JSONObject data(String origin, String path, String method, JSONObject body, String token) throws Exception {
        var result = DeviceRuntime.request(origin, path, method, body, token); int status = result.getInt("status");
        if (status == 401 || status == 403) { stopSelf(); throw new SecurityException("Access revoked"); }
        if (status < 200 || status >= 300) throw new java.io.IOException("Server unavailable");
        return result.getJSONObject("data");
    }
    private boolean current(String session, String config) throws Exception {
        return running && !DeviceRuntime.foreground && session.equals(SessionVault.read(this)) && config.equals(getSharedPreferences(DeviceRuntime.PREFS, 0).getString("config", null)) && DeviceRuntime.allowed(this);
    }
    private void status(String text) { getSystemService(NotificationManager.class).notify(951, DeviceRuntime.notification(this, "connection", "CoupleLink", text, true)); }
    private void poll() {
        if (DeviceRuntime.foreground || !running) return;
        try {
            var prefs = getSharedPreferences(DeviceRuntime.PREFS, 0);
            String raw = prefs.getString("config", null), saved = SessionVault.read(this);
            if (raw == null || saved == null || !DeviceRuntime.allowed(this)) { stopSelf(); return; }
            JSONObject config = new JSONObject(raw), session = new JSONObject(saved);
            String origin = config.getString("origin"), token = session.getString("token"), couple = config.getString("couple");
            if (!origin.equals(session.getString("origin"))) { stopSelf(); return; }
            JSONObject account = data(origin, "/account", "GET", null, token);
            if (!account.getJSONObject("user").getString("id").equals(config.getString("user")) || account.isNull("couple") || !account.getJSONObject("couple").getString("id").equals(couple)) { stopSelf(); return; }
            if (!current(saved, raw)) return;
            if (account.optJSONArray("features") != null && account.getJSONArray("features").toString().contains("\"memories_v1\"")) {
                JSONArray reminders = data(origin, "/anniversaries/reminders?couple_id=" + couple, "GET", null, token).getJSONArray("reminders");
                for (int i = 0; i < reminders.length(); i++) {
                    if (!current(saved, raw)) return;
                    JSONObject reminder = reminders.getJSONObject(i);
                    DeviceRuntime.notify(this, "anniversary", getString(R.string.anniversary_reminder), reminder.getString("title"));
                    if (!current(saved, raw)) return;
                    data(origin, "/anniversaries/reminders/receipts", "POST", new JSONObject().put("couple_id", couple).put("id", reminder.getString("id")), token);
                }
            }
            if (disconnected) { status(getString(R.string.background_connected)); disconnected = false; }
            long cursor = config.optLong("cursor");
            JSONObject response = data(origin, "/messages?couple_id=" + couple + "&after=" + cursor, "GET", null, token);
            JSONArray messages = response.getJSONArray("messages"); boolean incoming = false;
            long next = response.getLong("next_cursor");
            if (next < cursor) throw new JSONException("Invalid cursor");
            for (int i = 0; i < messages.length(); i++) {
                JSONObject m = messages.getJSONObject(i);
                if (!couple.equals(m.getString("couple_id"))) throw new JSONException("Invalid message scope");
                if (!m.getString("sender_id").equals(config.getString("user")) && m.getLong("seq") > cursor) incoming = true;
            }
            if (!current(saved, raw)) return;
            if (incoming) DeviceRuntime.notify(this, "messages", getString(R.string.new_message_title), getString(R.string.new_message_body));
            config.put("cursor", next);
            // Foreground baseline writes and this cursor update must not overwrite each other.
            synchronized (ReceptionService.class) {
                if (!current(saved, raw)) return;
                if (!prefs.edit().putString("config", config.toString()).commit()) throw new java.io.IOException("Cursor save failed");
                raw = config.toString();
            }
            long started = SystemClock.elapsedRealtime();
            JSONObject claim = data(origin, "/interactions/claim", "POST", new JSONObject().put("couple_id", couple), token);
            if (claim.isNull("interaction") || !current(saved, raw)) return;
            JSONObject item = claim.getJSONObject("interaction"); String action = item.getString("action");
            String label = switch (action) { case "poke" -> getString(R.string.action_poke); case "miss_you" -> getString(R.string.action_miss); case "hug" -> getString(R.string.action_hug); case "vibrate" -> getString(R.string.action_vibrate); case "ring" -> getString(R.string.action_ring); default -> throw new JSONException("Unknown interaction"); };
            long remaining = Math.max(0, claim.getLong("remaining_ms") - (SystemClock.elapsedRealtime() - started));
            String result = DeviceRuntime.interaction(this, action, getString(R.string.interaction_title), label, config.optBoolean("sound"), config.optBoolean("vibrate"), System.currentTimeMillis() + remaining).getString("result");
            data(origin, "/interactions/result", "POST", new JSONObject().put("id", item.getString("id")).put("couple_id", couple).put("result", result), token);
        } catch (SecurityException e) { stopSelf(); }
        catch (Exception e) { if (running) { disconnected = true; status(getString(R.string.background_retry)); } }
    }
    @Override public void onTimeout(int startId, int fgsType) { stopSelf(); }
    @Override public void onDestroy() { running = false; if (worker != null) worker.shutdownNow(); DeviceRuntime.stopSound(); stopForeground(STOP_FOREGROUND_REMOVE); super.onDestroy(); }
    @Override public IBinder onBind(Intent intent) { return null; }
}
