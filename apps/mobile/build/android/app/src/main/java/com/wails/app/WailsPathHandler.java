// SPDX-License-Identifier: MIT
// Copyright (c) 2018-Present Lea Anthony
// Derived from the Wails v3 Android template; modified.

package com.wails.app;

import android.util.Log;
import android.webkit.WebResourceResponse;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import androidx.webkit.WebViewAssetLoader;

import java.io.ByteArrayInputStream;
import java.util.Map;

/** Serves the interface's assets from the Go library's asset server. */
public class WailsPathHandler implements WebViewAssetLoader.PathHandler {
    private static final String TAG = "WailsPathHandler";

    private final WailsBridge bridge;

    public WailsPathHandler(WailsBridge bridge) {
        this.bridge = bridge;
    }

    @Nullable
    @Override
    public WebResourceResponse handle(@NonNull String path) {
        if (path.isEmpty() || path.equals("/")) {
            path = "/index.html";
        }
        byte[] data = bridge.serveAsset(path, "GET", "{}");
        if (data == null || data.length == 0) {
            Log.w(TAG, "An asset was not found");
            return null;
        }
        return new WebResourceResponse(bridge.getAssetMimeType(path), "UTF-8", 200, "OK",
                Map.of("Cache-Control", "no-cache"), new ByteArrayInputStream(data));
    }
}
