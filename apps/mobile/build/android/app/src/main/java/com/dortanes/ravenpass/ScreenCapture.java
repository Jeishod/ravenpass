package com.dortanes.ravenpass;

import android.app.Activity;
import android.os.Bundle;
import android.view.Window;
import android.view.WindowManager;

import androidx.annotation.NonNull;

import com.wails.app.MainActivity;

import java.lang.ref.WeakReference;

/** Keeps every window out of screen captures from before its first frame; only the main window may let them in. */
final class ScreenCapture extends ActivityCallbacks {
    // Main thread only.
    private static boolean allowedInMainWindow;
    private static WeakReference<Activity> mainWindow = new WeakReference<>(null);

    static void allowInMainWindow(boolean allowed) {
        allowedInMainWindow = allowed;
        Activity activity = mainWindow.get();
        if (activity != null) {
            secure(activity);
        }
    }

    // The window exists once the activity is attached, before onCreate sets its content.
    @Override
    public void onActivityPreCreated(@NonNull Activity activity, Bundle state) {
        if (activity instanceof MainActivity) {
            mainWindow = new WeakReference<>(activity);
        }
        secure(activity);
    }

    private static void secure(Activity activity) {
        Window window = activity.getWindow();
        if (activity instanceof MainActivity && allowedInMainWindow) {
            window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE);
        } else {
            window.addFlags(WindowManager.LayoutParams.FLAG_SECURE);
        }
    }
}
