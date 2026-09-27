// SPDX-License-Identifier: MIT
// Copyright (c) 2018-Present Lea Anthony
// Derived from the Wails v3 Android template; modified.

package com.wails.app;

import android.util.Log;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** The page's window.wails object, which @wailsio/runtime detects and calls. */
public class WailsJSBridge {
    private static final String TAG = "WailsJSBridge";
    private static final ExecutorService executor = Executors.newCachedThreadPool();

    private final WailsBridge bridge;
    private final WebView webView;

    public WailsJSBridge(WailsBridge bridge, WebView webView) {
        this.bridge = bridge;
        this.webView = webView;
    }

    @JavascriptInterface
    public String invoke(String message) {
        return bridge.handleMessage(message);
    }

    /** Answers through window._wailsAndroidCallback(callbackId, response, error). */
    @JavascriptInterface
    public void invokeAsync(final String callbackId, final String payload) {
        executor.execute(() -> {
            try {
                sendCallback(callbackId, bridge.handleRuntimeCall(payload), null);
            } catch (RuntimeException e) {
                Log.e(TAG, "A runtime call failed");
                sendCallback(callbackId, null, "runtime call failed");
            }
        });
    }

    private void sendCallback(String callbackId, String result, String error) {
        final String js;
        if (error != null) {
            js = String.format(
                    "window._wailsAndroidCallback && window._wailsAndroidCallback('%s', null, '%s');",
                    escapeJsString(callbackId),
                    escapeJsString(error));
        } else {
            js = String.format(
                    "window._wailsAndroidCallback && window._wailsAndroidCallback('%s', '%s', null);",
                    escapeJsString(callbackId),
                    escapeJsString(result != null ? result : ""));
        }
        webView.post(() -> webView.evaluateJavascript(js, null));
    }

    private static String escapeJsString(String str) {
        if (str == null) return "";
        return str.replace("\\", "\\\\")
                .replace("'", "\\'")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                // U+2028 and U+2029 end a JavaScript line; a literal of either would end this Java line too.
                .replace(String.valueOf((char) 0x2028), "\\u2028")
                .replace(String.valueOf((char) 0x2029), "\\u2029");
    }
}
