// SPDX-License-Identifier: MIT
// Copyright (c) 2018-Present Lea Anthony
// Derived from the Wails v3 Android template; modified.

package com.wails.app;

import android.app.Activity;
import android.content.Intent;
import android.content.res.Configuration;
import android.graphics.Insets;
import android.graphics.Rect;
import android.net.Uri;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.view.WindowInsets;
import android.view.WindowMetrics;
import android.webkit.WebView;

import org.json.JSONException;
import org.json.JSONObject;

/** Wails' Go side calls these public methods over JNI by name and signature; renaming one breaks it silently. */
public class WailsBridge {
    private static final String TAG = "WailsBridge";

    static {
        System.loadLibrary("wails");
    }

    private final Activity activity;
    private final Handler mainHandler = new Handler(Looper.getMainLooper());
    private WebView webView;
    private volatile boolean initialized = false;

    private static native void nativeInit(WailsBridge bridge);
    private static native void nativeShutdown();
    private static native void nativeOnStart();
    private static native void nativeOnResume();
    private static native void nativeOnPause();
    private static native void nativeOnStop();
    private static native void nativeOnLowMemory();
    private static native void nativeOnPageFinished(String url);
    private static native byte[] nativeServeAsset(String path, String method, String headers);
    private static native String nativeHandleMessage(String message);
    private static native String nativeHandleRuntimeCall(String payload);
    private static native String nativeGetAssetMimeType(String path);
    private static native void nativeFilePickerResult(int callbackID, String path);
    private static native void nativeFilePickerDone(int callbackID);
    private static native void nativeMainThreadCallback(int callbackID);
    private static native void nativeEmitSystemEvent(String name, String json);

    public WailsBridge(Activity activity) {
        this.activity = activity;
    }

    public void initialize() {
        if (initialized) {
            return;
        }
        nativeInit(this);
        initialized = true;
    }

    public void shutdown() {
        if (!initialized) {
            return;
        }
        nativeShutdown();
        initialized = false;
    }

    /** Must be set before the page loads. */
    public void setWebView(WebView webView) {
        this.webView = webView;
    }

    public void onStart() {
        if (initialized) nativeOnStart();
    }

    public void onResume() {
        if (initialized) nativeOnResume();
    }

    public void onPause() {
        if (initialized) nativeOnPause();
    }

    public void onStop() {
        if (initialized) nativeOnStop();
    }

    public void onLowMemory() {
        if (initialized) nativeOnLowMemory();
    }

    public void onPageFinished(String url) {
        if (initialized) nativeOnPageFinished(url);
    }

    public void emitSystemEvent(String name, String json) {
        if (initialized) nativeEmitSystemEvent(name, json);
    }

    public byte[] serveAsset(String path, String method, String headers) {
        if (!initialized) {
            Log.w(TAG, "An asset was requested before the Go library started");
            return null;
        }
        return nativeServeAsset(path, method, headers);
    }

    public String getAssetMimeType(String path) {
        if (!initialized) {
            return "application/octet-stream";
        }
        String mimeType = nativeGetAssetMimeType(path);
        return mimeType != null ? mimeType : "application/octet-stream";
    }

    public String handleMessage(String message) {
        if (!initialized) {
            Log.w(TAG, "A message arrived before the Go library started");
            return "{\"error\":\"Bridge not initialized\"}";
        }
        return nativeHandleMessage(message);
    }

    public String handleRuntimeCall(String payload) {
        if (!initialized) {
            return "{\"ok\":false,\"error\":\"Bridge not initialized\"}";
        }
        return nativeHandleRuntimeCall(payload);
    }

    /** Any thread. */
    public void executeJavaScript(final String js) {
        final WebView view = webView;
        if (view == null) {
            Log.w(TAG, "JavaScript arrived with no WebView attached");
            return;
        }
        mainHandler.post(() -> view.evaluateJavascript(js, null));
    }

    /** Hardware pixels, density and system bar insets as JSON; empty where they cannot be read. */
    public String getScreenInfoJson() {
        try {
            WindowMetrics metrics = activity.getWindowManager().getCurrentWindowMetrics();
            Rect bounds = metrics.getBounds();
            Insets insets = metrics.getWindowInsets().getInsetsIgnoringVisibility(WindowInsets.Type.systemBars());
            return new JSONObject()
                    .put("widthPx", bounds.width())
                    .put("heightPx", bounds.height())
                    .put("density", activity.getResources().getDisplayMetrics().density)
                    .put("insetTop", insets.top)
                    .put("insetBottom", insets.bottom)
                    .put("insetLeft", insets.left)
                    .put("insetRight", insets.right)
                    .toString();
        } catch (JSONException e) {
            Log.w(TAG, "The screen metrics were not encoded", e);
            return "";
        }
    }

    public boolean isDarkMode() {
        int mode = activity.getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        return mode == Configuration.UI_MODE_NIGHT_YES;
    }

    public boolean isMainThread() {
        return Looper.myLooper() == Looper.getMainLooper();
    }

    public void runOnMainThread(final int callbackID) {
        // The callback may run after shutdown() stopped the Go side.
        mainHandler.post(() -> {
            if (initialized) nativeMainThreadCallback(callbackID);
        });
    }

    public void openURL(final String url) {
        mainHandler.post(() -> {
            try {
                activity.startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(url))
                        .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
            } catch (RuntimeException e) {
                Log.w(TAG, "No app opened the link");
            }
        });
    }

    /** optionsJson: {"multiple": bool}; results arrive through filePickerResult and filePickerDone. */
    public void showFilePicker(final int callbackID, final String optionsJson) {
        boolean multiple = false;
        try {
            multiple = new JSONObject(optionsJson).optBoolean("multiple", false);
        } catch (JSONException e) {
            Log.w(TAG, "The file picker's options were not read");
        }
        final boolean allowMultiple = multiple;
        mainHandler.post(() -> {
            if (activity instanceof MainActivity) {
                ((MainActivity) activity).launchFilePicker(callbackID, allowMultiple);
            } else {
                Log.e(TAG, "The file picker needs the main activity");
                filePickerDone(callbackID);
            }
        });
    }

    // The picker finishes on a thread that may outlive shutdown().
    void filePickerResult(int callbackID, String path) {
        if (initialized) nativeFilePickerResult(callbackID, path);
    }

    void filePickerDone(int callbackID) {
        if (initialized) nativeFilePickerDone(callbackID);
    }
}
