package com.dortanes.ravenpass;

import android.webkit.WebSettings;

/** Closes the WebView capabilities that the app's local pages never use. */
public final class WebViewPolicy {
    private WebViewPolicy() {
    }

    @SuppressWarnings("deprecation")
    public static void restrict(WebSettings settings) {
        settings.setAllowFileAccess(false);
        settings.setAllowContentAccess(false);
        settings.setAllowFileAccessFromFileURLs(false);
        settings.setAllowUniversalAccessFromFileURLs(false);
        settings.setJavaScriptCanOpenWindowsAutomatically(false);
        settings.setSupportMultipleWindows(false);
        settings.setGeolocationEnabled(false);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
    }
}
