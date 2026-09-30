package com.dortanes.ravenpass;

import android.app.LocaleManager;
import android.content.Context;
import android.content.res.Configuration;
import android.content.res.Resources;
import android.os.Build;
import android.os.LocaleList;

/** Gives Android's own text in the app the language the owner chose in Ravenpass, else the device's. */
public final class AppLanguage {
    // Before API 33 Android keeps no per-app language, so the chosen tag is kept here for each new component.
    private static final String STORE = "language";
    private static final String TAG = "tag";

    private AppLanguage() {
    }

    /** An empty tag follows the device's language. */
    static void follow(Context context, String tag) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            LocaleManager locales = context.getSystemService(LocaleManager.class);
            LocaleList wanted = LocaleList.forLanguageTags(tag);
            if (locales != null && !locales.getApplicationLocales().equals(wanted)) {
                locales.setApplicationLocales(wanted);
            }
            return;
        }
        context.getSharedPreferences(STORE, Context.MODE_PRIVATE).edit().putString(TAG, tag).apply();
    }

    /** Returns base in the chosen language; a component passes its base context through this in attachBaseContext. */
    public static Context apply(Context base) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            return base;
        }
        String tag = base.getSharedPreferences(STORE, Context.MODE_PRIVATE).getString(TAG, "");
        if (tag.isEmpty()) {
            return base;
        }
        Configuration configuration = new Configuration(base.getResources().getConfiguration());
        configuration.setLocales(LocaleList.forLanguageTags(tag));
        return base.createConfigurationContext(configuration);
    }

    /**
     * Resources in the language chosen now, for a component that outlives a change of it: before API 33 nothing tells
     * a running service, and the autofill service keeps running while it is the device's choice.
     */
    public static final class Following {
        private final Context base;
        private String tag;
        private Resources resources;

        /** base is the component's context before apply. */
        public Following(Context base) {
            this.base = base;
        }

        public synchronized Resources resources() {
            String chosen = base.getSharedPreferences(STORE, Context.MODE_PRIVATE).getString(TAG, "");
            if (resources == null || !chosen.equals(tag)) {
                tag = chosen;
                resources = apply(base).getResources();
            }
            return resources;
        }
    }
}
