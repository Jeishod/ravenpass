package com.dortanes.ravenpass.autofill;

import android.content.Context;
import android.content.Intent;
import android.os.Parcelable;
import android.util.Log;
import android.view.autofill.AutofillManager;

import androidx.activity.result.ActivityResult;
import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.fragment.app.FragmentActivity;

import com.dortanes.ravenpass.AppLanguage;

import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.function.Consumer;

/** A FragmentActivity for the BiometricPrompt Bridge shows in the resumed activity. */
abstract class AutofillActivity extends FragmentActivity {
    static final String TAG = "RavenpassAutofill";

    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private final ActivityResultLauncher<Intent> unlock = registerForActivityResult(
            new ActivityResultContracts.StartActivityForResult(), this::unlocked);
    /** The calls that found the vault locked, to run again once the unlock screen opens it. */
    private final List<Runnable> waiting = new ArrayList<>();

    @Override
    protected void attachBaseContext(Context base) {
        super.attachBaseContext(AppLanguage.apply(base));
    }

    @Override
    protected void onStart() {
        super.onStart();
        Core.call(Core.request("shown"));
    }

    @Override
    protected void onStop() {
        Core.call(Core.request("hidden"));
        super.onStop();
    }

    @Override
    protected void onDestroy() {
        worker.shutdownNow();
        super.onDestroy();
    }

    /** Runs task on the screen's one worker; done receives null where task threw. */
    protected final <T> void work(Callable<T> task, Consumer<T> done) {
        run(worker, task, done);
    }

    /** done receives null where task threw, on the main thread, and only while the screen lives. */
    protected final <T> void run(ExecutorService threads, Callable<T> task, Consumer<T> done) {
        threads.execute(() -> {
            T result = null;
            try {
                result = task.call();
            } catch (Exception e) {
                Log.w(TAG, "An autofill screen's call failed: " + e.getClass().getSimpleName());
            }
            T answered = result;
            runOnUiThread(() -> {
                if (!isFinishing() && !isDestroyed()) {
                    done.accept(answered);
                }
            });
        });
    }

    /** A locked vault brings up the unlock screen, then the call runs again; a dismissed unlock cancels. */
    protected final void ask(Callable<JSONObject> call, Consumer<JSONObject> done) {
        work(call, answer -> {
            if (answer == null || !Core.LOCKED.equals(Core.status(answer))) {
                done.accept(answer);
                return;
            }
            waiting.add(() -> work(call, done));
            if (waiting.size() == 1) {
                unlock.launch(new Intent(this, UnlockActivity.class));
            }
        });
    }

    private void unlocked(ActivityResult result) {
        List<Runnable> again = new ArrayList<>(waiting);
        waiting.clear();
        if (result.getResultCode() != RESULT_OK || again.isEmpty()) {
            cancel();
            return;
        }
        again.forEach(Runnable::run);
    }

    /** result is a dataset or a response that replaces the entry. */
    protected final void authenticated(Parcelable result) {
        setResult(RESULT_OK, new Intent().putExtra(AutofillManager.EXTRA_AUTHENTICATION_RESULT, result));
        finish();
    }

    protected final void cancel() {
        setResult(RESULT_CANCELED);
        finish();
    }
}
