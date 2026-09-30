// SPDX-License-Identifier: MIT
// Copyright (c) 2018-Present Lea Anthony
// Derived from the Wails v3 Android template; modified.

package com.wails.app;

import android.annotation.SuppressLint;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.res.Configuration;
import android.database.Cursor;
import android.net.Uri;
import android.os.Bundle;
import android.provider.OpenableColumns;
import android.util.Log;
import android.view.View;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

import androidx.activity.result.ActivityResult;
import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.WebViewAssetLoader;

import com.dortanes.ravenpass.AppAppearance;
import com.dortanes.ravenpass.WebViewPolicy;

import java.io.ByteArrayInputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

public class MainActivity extends AppCompatActivity {
    private static final String TAG = "WailsActivity";
    private static final String WAILS_HOST = "wails.localhost";
    private static final String WAILS_API = "/wails/";
    private static final String PICKED_FILES = "wails-picker";

    private final ActivityResultLauncher<Intent> filePicker =
            registerForActivityResult(new ActivityResultContracts.StartActivityForResult(), this::filesPicked);
    private WebView webView;
    private WailsBridge bridge;
    private WebViewAssetLoader assetLoader;
    private BroadcastReceiver screenOffReceiver;
    // Main thread only; -1 while no picker is open.
    private int pendingFilePickerCallbackID = -1;

    /** Call only as the process starts: the Go side may read a picked copy at any time during the process. */
    public static void discardPickedFiles(Context context) {
        deleteTree(new File(context.getCacheDir(), PICKED_FILES));
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        AppAppearance.paintSystemBars(this);
        bridge = new WailsBridge(this);
        bridge.initialize();
        setupWebView();
        webView.loadUrl("https://" + WAILS_HOST + "/");
    }

    @SuppressLint("SetJavaScriptEnabled")
    private void setupWebView() {
        webView = findViewById(R.id.webview);
        bridge.setWebView(webView);
        // The page never scrolls, only its panes, which draw their own bars; the WebView's viewport can still exceed
        // the page by a fraction of a pixel, which a swipe scrolls and shows a scroll bar for.
        webView.setVerticalScrollBarEnabled(false);
        webView.setHorizontalScrollBarEnabled(false);
        webView.setOverScrollMode(View.OVER_SCROLL_NEVER);

        WebSettings settings = webView.getSettings();
        WebViewPolicy.restrict(settings);
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG);

        assetLoader = new WebViewAssetLoader.Builder()
                .setDomain(WAILS_HOST)
                .addPathHandler("/", new WailsPathHandler(bridge))
                .build();

        webView.setWebViewClient(new WebViewClient() {
            @Nullable
            @Override
            public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                Uri url = request.getUrl();
                // Photos and site icons are data: URLs, which WebView decodes without the network.
                if ("data".equals(url.getScheme())) {
                    return null;
                }
                if (!isAppAddress(url)) {
                    return refused();
                }
                String path = url.getPath();
                if (path != null && path.startsWith(WAILS_API)) {
                    return serveRuntime(url, request.getMethod());
                }
                WebResourceResponse asset = assetLoader.shouldInterceptRequest(url);
                return asset != null ? asset : refused();
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                return !request.isForMainFrame() || !isAppAddress(request.getUrl());
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                super.onPageFinished(view, url);
                bridge.onPageFinished(url);
            }
        });

        // Every frame sees window.wails, so no origin but the app's may ever load in this WebView, and the page's
        // Content-Security-Policy refuses every frame, a data: frame included.
        // @wailsio/runtime calls window.wails.invoke synchronously, which only addJavascriptInterface answers.
        webView.addJavascriptInterface(new WailsJSBridge(bridge, webView), "wails");
    }

    private static boolean isAppAddress(Uri url) {
        return "https".equals(url.getScheme()) && WAILS_HOST.equals(url.getHost());
    }

    private static WebResourceResponse refused() {
        return new WebResourceResponse("text/plain", "UTF-8", 403, "Forbidden", Map.of(),
                new ByteArrayInputStream(new byte[0]));
    }

    // WebViewAssetLoader.PathHandler drops the query string, which runtime requests need.
    private WebResourceResponse serveRuntime(Uri url, String method) {
        String query = url.getQuery();
        String path = query == null || query.isEmpty() ? url.getPath() : url.getPath() + "?" + query;
        byte[] data = bridge.serveAsset(path, method, "{}");
        Map<String, String> headers = new HashMap<>();
        headers.put("Cache-Control", "no-cache");
        if (data == null || data.length == 0) {
            return new WebResourceResponse("application/json", "UTF-8", 500, "Internal Error", headers,
                    new ByteArrayInputStream("{}".getBytes(StandardCharsets.UTF_8)));
        }
        return new WebResourceResponse("application/json", "UTF-8", 200, "OK", headers,
                new ByteArrayInputStream(data));
    }

    /** The Go side receives each picked document as a copy in the cache. */
    public void launchFilePicker(int callbackID, boolean multiple) {
        if (pendingFilePickerCallbackID != -1) {
            bridge.filePickerDone(callbackID);
            return;
        }
        pendingFilePickerCallbackID = callbackID;
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT)
                .addCategory(Intent.CATEGORY_OPENABLE)
                .setType("*/*")
                .putExtra(Intent.EXTRA_ALLOW_MULTIPLE, multiple);
        try {
            filePicker.launch(intent);
        } catch (RuntimeException e) {
            Log.e(TAG, "The file picker did not open", e);
            pendingFilePickerCallbackID = -1;
            bridge.filePickerDone(callbackID);
        }
    }

    private void filesPicked(ActivityResult result) {
        final int callbackID = pendingFilePickerCallbackID;
        pendingFilePickerCallbackID = -1;
        if (callbackID == -1) {
            return;
        }

        Intent data = result.getData();
        final List<Uri> uris = new ArrayList<>();
        if (result.getResultCode() == RESULT_OK && data != null) {
            if (data.getClipData() != null) {
                for (int i = 0; i < data.getClipData().getItemCount(); i++) {
                    uris.add(data.getClipData().getItemAt(i).getUri());
                }
            } else if (data.getData() != null) {
                uris.add(data.getData());
            }
        }

        new Thread(() -> {
            for (Uri uri : uris) {
                String path = copyToCache(uri);
                if (path != null) {
                    bridge.filePickerResult(callbackID, path);
                }
            }
            bridge.filePickerDone(callbackID);
        }).start();
    }

    @Nullable
    private String copyToCache(Uri uri) {
        File dir = new File(getCacheDir(), PICKED_FILES + "/" + System.nanoTime());
        if (!dir.mkdirs()) {
            Log.w(TAG, "The picked document's cache folder was not created");
            return null;
        }
        File out = new File(dir, displayName(uri));
        try (InputStream in = getContentResolver().openInputStream(uri);
             OutputStream os = new FileOutputStream(out)) {
            if (in == null) {
                return null;
            }
            byte[] buffer = new byte[64 * 1024];
            for (int n = in.read(buffer); n != -1; n = in.read(buffer)) {
                os.write(buffer, 0, n);
            }
            return out.getAbsolutePath();
        } catch (IOException | RuntimeException e) {
            Log.e(TAG, "The picked document was not copied");
            return null;
        }
    }

    private String displayName(Uri uri) {
        try (Cursor cursor = getContentResolver().query(uri, new String[] {OpenableColumns.DISPLAY_NAME},
                null, null, null)) {
            if (cursor != null && cursor.moveToFirst() && !cursor.isNull(0)) {
                String name = new File(cursor.getString(0)).getName();
                if (!name.isEmpty() && !name.equals(".") && !name.equals("..")) {
                    return name;
                }
            }
        } catch (RuntimeException e) {
            Log.w(TAG, "The picked document's name was not read");
        }
        return "document";
    }

    private static void deleteTree(File file) {
        File[] children = file.listFiles();
        if (children != null) {
            for (File child : children) {
                deleteTree(child);
            }
        }
        if (file.exists() && !file.delete()) {
            Log.w(TAG, "A picked document's copy was not deleted");
        }
    }

    // ACTION_SCREEN_OFF is a protected broadcast: registering for it needs no export flag.
    private void registerScreenOffReceiver() {
        screenOffReceiver = new BroadcastReceiver() {
            @Override
            public void onReceive(Context context, Intent intent) {
                bridge.emitSystemEvent("android:ScreenLocked", "{}");
            }
        };
        registerReceiver(screenOffReceiver, new IntentFilter(Intent.ACTION_SCREEN_OFF));
    }

    private void unregisterScreenOffReceiver() {
        if (screenOffReceiver == null) {
            return;
        }
        try {
            unregisterReceiver(screenOffReceiver);
        } catch (IllegalArgumentException e) {
            Log.w(TAG, "The screen-off receiver was not registered", e);
        }
        screenOffReceiver = null;
    }

    @Override
    protected void onStart() {
        super.onStart();
        registerScreenOffReceiver();
        bridge.onStart();
    }

    @Override
    protected void onResume() {
        super.onResume();
        bridge.onResume();
    }

    @Override
    public void onConfigurationChanged(@NonNull Configuration configuration) {
        super.onConfigurationChanged(configuration);
        AppAppearance.paintSystemBars(this);
    }

    @Override
    protected void onPause() {
        super.onPause();
        bridge.onPause();
    }

    @Override
    protected void onStop() {
        super.onStop();
        unregisterScreenOffReceiver();
        bridge.onStop();
    }

    @Override
    public void onLowMemory() {
        super.onLowMemory();
        bridge.onLowMemory();
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();
        unregisterScreenOffReceiver();
        bridge.shutdown();
        webView.destroy();
    }
}
