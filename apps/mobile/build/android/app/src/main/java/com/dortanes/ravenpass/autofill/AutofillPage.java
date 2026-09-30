package com.dortanes.ravenpass.autofill;

import android.annotation.SuppressLint;
import android.content.Context;
import android.graphics.Color;
import android.net.Uri;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.view.View;
import android.webkit.RenderProcessGoneDetail;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

import androidx.webkit.JavaScriptReplyProxy;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewAssetLoader;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;

import com.dortanes.ravenpass.WebViewPolicy;
import com.wails.app.R;

import org.json.JSONException;
import org.json.JSONObject;

import java.io.ByteArrayInputStream;
import java.util.Map;
import java.util.Set;

/** The autofill page from the APK's assets; any other request is answered empty and navigation is blocked. */
final class AutofillPage {
    /**
     * Takes {"id", "request"} as JSON text, where the request names "op"; the JSON answer to {@value #ANSWER} returns
     * the id. Only frames of the page's own origin see the object, and only its main frame's messages are taken.
     */
    private static final String BRIDGE = "ravenpassAutofill";
    private static final String ANSWER = "window.ravenpassAutofillAnswer";
    private static final String ORIGIN = "https://" + WebViewAssetLoader.DEFAULT_DOMAIN;
    private static final String ADDRESS = ORIGIN + "/assets/autofill/autofill.html";

    /** Receives the page's requests on the main thread while the page lives. */
    interface Receiver {
        void receive(Request request);

        /** The page did not load, or its renderer ended. */
        void lost();
    }

    /** Its fields are what the page sent besides the operation. */
    final class Request {
        final String op;
        final JSONObject fields;
        private final int id;

        private Request(int id, String op, JSONObject fields) {
            this.id = id;
            this.op = op;
            this.fields = fields;
        }

        /** A notice, sent with id 0, takes no answer. */
        void answer(JSONObject answer) {
            if (id == 0) {
                return;
            }
            String script = ANSWER + "(" + id + "," + JSONObject.quote(answer.toString()) + ")";
            main.post(() -> {
                if (!destroyed) {
                    view.evaluateJavascript(script, null);
                }
            });
        }

        void answer(String status) {
            answer(Core.answer(status));
        }
    }

    private final Handler main = new Handler(Looper.getMainLooper());
    private final WebView view;
    private final Receiver receiver;
    // Main thread only.
    private boolean destroyed;

    @SuppressLint("SetJavaScriptEnabled")
    AutofillPage(Context context, Receiver receiver) {
        this.receiver = receiver;
        view = new WebView(context);
        // RavenpassApplication sends the back gesture to the history of the WebView with this id.
        view.setId(R.id.webview);
        // No autofill service, Ravenpass's own among them, may see the PIN or a new item's fields.
        view.setImportantForAutofill(View.IMPORTANT_FOR_AUTOFILL_NO_EXCLUDE_DESCENDANTS);
        // The page scrolls inside its own areas; the WebView otherwise stretches it past its edge.
        view.setOverScrollMode(View.OVER_SCROLL_NEVER);
        view.setBackgroundColor(Color.TRANSPARENT);
        WebSettings settings = view.getSettings();
        WebViewPolicy.restrict(settings);
        settings.setJavaScriptEnabled(true);
        // The page's viewport tag, which draws it edge to edge, takes effect only with a wide viewport.
        settings.setUseWideViewPort(true);
        settings.setBlockNetworkLoads(true);
        settings.setDomStorageEnabled(false);
        WebViewAssetLoader assets = new WebViewAssetLoader.Builder()
                .addPathHandler("/assets/", new WebViewAssetLoader.AssetsPathHandler(context))
                .build();
        view.setWebViewClient(new WebViewClient() {
            @Override
            public WebResourceResponse shouldInterceptRequest(WebView page, WebResourceRequest request) {
                WebResourceResponse asset = assets.shouldInterceptRequest(request.getUrl());
                return asset != null ? asset : nothing();
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView page, WebResourceRequest request) {
                return true;
            }

            @Override
            public void onReceivedHttpError(WebView page, WebResourceRequest request, WebResourceResponse response) {
                if (request.isForMainFrame()) {
                    Log.w(AutofillActivity.TAG, "The autofill page is missing from the app's assets");
                    receiver.lost();
                }
            }

            @Override
            public boolean onRenderProcessGone(WebView page, RenderProcessGoneDetail detail) {
                Log.w(AutofillActivity.TAG, "The autofill page's renderer ended");
                receiver.lost();
                return true;
            }
        });
        if (WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) {
            WebViewCompat.addWebMessageListener(view, BRIDGE, Set.of(ORIGIN), this::posted);
        }
    }

    WebView view() {
        return view;
    }

    /** A WebView without web message listeners cannot reach the page's requests, so the page counts as lost. */
    void load() {
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) {
            Log.w(AutofillActivity.TAG, "The system WebView cannot carry the autofill page's requests");
            receiver.lost();
            return;
        }
        view.loadUrl(ADDRESS);
    }

    void destroy() {
        destroyed = true;
        if (WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) {
            WebViewCompat.removeWebMessageListener(view, BRIDGE);
        }
        view.destroy();
    }

    private static WebResourceResponse nothing() {
        return new WebResourceResponse("text/plain", "utf-8", 404, "Not Found", Map.of(),
                new ByteArrayInputStream(new byte[0]));
    }

    /** WebView calls it on the main thread, for the page's origin alone. */
    private void posted(WebView page, WebMessageCompat message, Uri origin, boolean mainFrame,
            JavaScriptReplyProxy reply) {
        String data = message.getType() == WebMessageCompat.TYPE_STRING ? message.getData() : null;
        if (destroyed || !mainFrame || data == null) {
            return;
        }
        JSONObject fields;
        int id;
        try {
            JSONObject posted = new JSONObject(data);
            id = posted.getInt("id");
            fields = new JSONObject(posted.getString("request"));
        } catch (JSONException e) {
            return;
        }
        Object op = fields.remove("op");
        if (op instanceof String) {
            receiver.receive(new Request(id, (String) op, fields));
        }
    }
}
