package app.couplelink.mobile;

import com.getcapacitor.BridgeActivity;
import android.os.Bundle;
import android.content.Intent;
import android.Manifest;
import android.os.Build;
import android.content.pm.PackageManager;
import androidx.activity.OnBackPressedCallback;
import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.core.content.ContextCompat;
import com.getcapacitor.PluginCall;
import com.getcapacitor.JSObject;

public class MainActivity extends BridgeActivity {
    private PluginCall permissionCall;
    private final ActivityResultLauncher<String> notificationPermission = registerForActivityResult(
        new ActivityResultContracts.RequestPermission(), granted -> finishPermission());
    void requestNotifications(PluginCall call) {
        runOnUiThread(() -> {
            if (isFinishing() || isDestroyed()) { call.reject("Open the app before granting permission", "foreground_required"); return; }
            if (permissionCall != null) { call.reject("A permission request is already open", "foreground_required"); return; }
            permissionCall = call;
            if (Build.VERSION.SDK_INT < 33 || ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) finishPermission();
            else notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS);
        });
    }
    private void finishPermission() {
        PluginCall call = permissionCall; permissionCall = null;
        if (call != null) { JSObject result = new JSObject(); result.put("granted", DeviceRuntime.allowed(this)); call.resolve(result); }
    }
    @Override public void onBackPressed() {
        if (bridge != null) bridge.triggerWindowJSEvent("couplelink-back", "{}");
        else super.onBackPressed();
    }
    @Override public void onCreate(Bundle savedInstanceState) {
        registerPlugin(CoupleDevicePlugin.class);
        super.onCreate(savedInstanceState);
        // Predictive back (on by default for targetSdk 35+) never reaches onBackPressed(); the
        // AndroidX dispatcher is the only path that still reports the gesture, so register it too.
        getOnBackPressedDispatcher().addCallback(this, new OnBackPressedCallback(true) {
            @Override public void handleOnBackPressed() {
                if (bridge != null) bridge.triggerWindowJSEvent("couplelink-back", "{}");
                else finish();
            }
        });
        if ((getApplicationInfo().flags & android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0) android.webkit.WebView.setWebContentsDebuggingEnabled(true);
    }
    @Override public void onResume() { super.onResume(); DeviceRuntime.foreground = true; }
    @Override public void onPause() { DeviceRuntime.foreground = false; DeviceRuntime.stopSound(); super.onPause(); }
    @Override public void onDestroy() {
        if (permissionCall != null) { permissionCall.reject("Activity closed before permission result", "foreground_required"); permissionCall = null; }
        super.onDestroy();
    }
    @Override protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent); setIntent(intent);
        if (intent.hasExtra("couplelink.kind")) bridge.triggerWindowJSEvent("couplelink-notification", "{}");
    }
}
