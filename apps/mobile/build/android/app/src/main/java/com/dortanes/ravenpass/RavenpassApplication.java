package com.dortanes.ravenpass;

import android.app.Activity;
import android.app.Application;
import android.os.Bundle;
import android.view.View;
import android.webkit.WebView;

import androidx.activity.ComponentActivity;
import androidx.activity.OnBackPressedCallback;
import androidx.activity.OnBackPressedDispatcher;
import androidx.annotation.NonNull;

import com.wails.app.MainActivity;
import com.wails.app.R;

/** Attaches the native bridge before any activity or service of the process runs. */
public final class RavenpassApplication extends Application {
    @Override
    public void onCreate() {
        super.onCreate();
        // WailsBridge loads the library again later; loading a loaded library is a no-op.
        System.loadLibrary("wails");
        Bridge.attach(this);
        MainActivity.discardPickedFiles(this);
        registerActivityLifecycleCallbacks(new ScreenCapture());
        registerActivityLifecycleCallbacks(new ActivityCallbacks() {
            @Override
            public void onActivityCreated(@NonNull Activity activity, Bundle state) {
                if (activity instanceof ComponentActivity) {
                    goBackInWebView((ComponentActivity) activity);
                }
            }

            // The activity creates its WebView in onCreate, after onActivityCreated runs.
            @Override
            public void onActivityPostCreated(@NonNull Activity activity, Bundle state) {
                WebView webView = activity.findViewById(R.id.webview);
                if (webView != null) {
                    // The page sets its viewport width and scale, which a WebView honours only with a wide viewport.
                    webView.getSettings().setUseWideViewPort(true);
                    // No autofill service, Ravenpass's own among them, may see the PIN, recovery key or passwords.
                    webView.setImportantForAutofill(View.IMPORTANT_FOR_AUTOFILL_NO_EXCLUDE_DESCENDANTS);
                    // The page scrolls inside its own areas; the WebView otherwise stretches it past its edge.
                    webView.setOverScrollMode(View.OVER_SCROLL_NEVER);
                }
            }
        });
    }

    // Under predictive back, from targetSdk 36, only the back dispatcher sees the gesture.
    private static void goBackInWebView(ComponentActivity activity) {
        OnBackPressedDispatcher dispatcher = activity.getOnBackPressedDispatcher();
        dispatcher.addCallback(activity, new OnBackPressedCallback(true) {
            @Override
            public void handleOnBackPressed() {
                WebView webView = activity.findViewById(R.id.webview);
                if (webView != null && webView.canGoBack()) {
                    webView.goBack();
                    return;
                }
                setEnabled(false);
                dispatcher.onBackPressed();
                setEnabled(true);
            }
        });
    }
}
