package com.dortanes.ravenpass;

import android.app.Activity;
import android.app.UiModeManager;
import android.content.Context;
import android.content.res.Configuration;
import android.view.Window;

import androidx.core.view.WindowCompat;
import androidx.core.view.WindowInsetsControllerCompat;

/** Gives Android's own screens and the app's windows the appearance the owner chose in Ravenpass. */
public final class AppAppearance {
    private AppAppearance() {
    }

    /** "light" or "dark"; anything else follows the device. Android keeps the choice for the app. */
    static void follow(Context context, String appearance) {
        UiModeManager modes = context.getSystemService(UiModeManager.class);
        if (modes == null) {
            return;
        }
        int wanted = switch (appearance) {
            case "light" -> UiModeManager.MODE_NIGHT_NO;
            case "dark" -> UiModeManager.MODE_NIGHT_YES;
            // For an app, AUTO leaves night mode undefined, which follows the device.
            default -> UiModeManager.MODE_NIGHT_AUTO;
        };
        modes.setApplicationNightMode(wanted);
    }

    /** Dark bar icons on a light screen; an activity that handles uiMode itself calls this again on each change. */
    public static void paintSystemBars(Activity activity) {
        int night = activity.getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        boolean light = night != Configuration.UI_MODE_NIGHT_YES;
        Window window = activity.getWindow();
        WindowInsetsControllerCompat bars = WindowCompat.getInsetsController(window, window.getDecorView());
        bars.setAppearanceLightStatusBars(light);
        bars.setAppearanceLightNavigationBars(light);
    }
}
