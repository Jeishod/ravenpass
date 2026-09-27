package com.dortanes.ravenpass;

import android.app.Activity;
import android.app.Application;
import android.os.Bundle;

import androidx.annotation.NonNull;

/** Activity lifecycle callbacks that ignore every event a subclass does not override. */
abstract class ActivityCallbacks implements Application.ActivityLifecycleCallbacks {
    @Override
    public void onActivityCreated(@NonNull Activity activity, Bundle state) {
    }

    @Override
    public void onActivityPostCreated(@NonNull Activity activity, Bundle state) {
    }

    @Override
    public void onActivityStarted(@NonNull Activity activity) {
    }

    @Override
    public void onActivityResumed(@NonNull Activity activity) {
    }

    @Override
    public void onActivityPaused(@NonNull Activity activity) {
    }

    @Override
    public void onActivityStopped(@NonNull Activity activity) {
    }

    @Override
    public void onActivitySaveInstanceState(@NonNull Activity activity, @NonNull Bundle state) {
    }

    @Override
    public void onActivityDestroyed(@NonNull Activity activity) {
    }
}
