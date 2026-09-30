package com.dortanes.ravenpass.autofill;

import android.app.assist.AssistStructure;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.IntentSender;
import android.content.res.Resources;
import android.os.Bundle;
import android.os.CancellationSignal;
import android.service.autofill.AutofillService;
import android.service.autofill.FillCallback;
import android.service.autofill.FillContext;
import android.service.autofill.FillRequest;
import android.service.autofill.FillResponse;
import android.service.autofill.SaveCallback;
import android.service.autofill.SaveRequest;
import android.util.Log;
import android.view.autofill.AutofillId;

import androidx.annotation.NonNull;
import androidx.core.os.BundleCompat;

import com.dortanes.ravenpass.AppLanguage;
import com.wails.app.R;

import org.json.JSONException;
import org.json.JSONObject;

import java.util.List;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

/** Android shows nothing for an answer later than 5 seconds after it asked; Ravenpass's own screens get none. */
public final class FillService extends AutofillService {
    private static final String TAG = "RavenpassAutofill";

    private ExecutorService worker;
    private AppLanguage.Following language;

    @Override
    protected void attachBaseContext(Context base) {
        language = new AppLanguage.Following(base);
        super.attachBaseContext(AppLanguage.apply(base));
    }

    /** The service outlives a change of language, which its answers follow. */
    @Override
    public Resources getResources() {
        return language != null ? language.resources() : super.getResources();
    }

    @Override
    public void onCreate() {
        super.onCreate();
        worker = Executors.newSingleThreadExecutor();
    }

    @Override
    public void onDestroy() {
        worker.shutdownNow();
        super.onDestroy();
    }

    @Override
    public void onFillRequest(@NonNull FillRequest request, @NonNull CancellationSignal cancellation,
            @NonNull FillCallback callback) {
        List<FillContext> contexts = request.getFillContexts();
        AssistStructure structure = contexts.get(contexts.size() - 1).getStructure();
        ComponentName activity = structure.getActivityComponent();
        if (activity == null || getPackageName().equals(activity.getPackageName())) {
            callback.onSuccess(null);
            return;
        }
        Screen screen = Screen.of(structure);
        Future<?> answering = worker.submit(() -> {
            FillResponse response = null;
            try {
                JSONObject answer = Core.call(screen.suggest(getPackageManager()));
                response = Responses.fill(this, screen, answer, request.getInlineSuggestionsRequest());
            } catch (RuntimeException e) {
                Log.w(TAG, "The fill request was not answered: " + e.getClass().getSimpleName());
            }
            if (!cancellation.isCanceled()) {
                callback.onSuccess(response);
            }
        });
        cancellation.setOnCancelListener(() -> answering.cancel(true));
    }

    /**
     * The sign-in waits in the core's memory, where the save screen asks for it by token; the system keeps the screen's
     * intent, so the intent carries nothing but that token. With the vault locked, the save screen unlocks it first.
     */
    @Override
    public void onSaveRequest(@NonNull SaveRequest request, @NonNull SaveCallback callback) {
        Bundle state = request.getClientState();
        JSONObject requester = requester(state);
        if (requester == null) {
            callback.onFailure(getString(R.string.autofill_save_failed));
            return;
        }
        List<FillContext> contexts = request.getFillContexts();
        Map<String, Object> capture = Core.request("capture");
        capture.put("requester", requester);
        capture.put("account", Screen.text(contexts,
                BundleCompat.getParcelable(state, Responses.SAVE_USERNAME, AutofillId.class)));
        capture.put("password", Screen.text(contexts,
                BundleCompat.getParcelable(state, Responses.SAVE_PASSWORD, AutofillId.class)));
        // Android waits on the callback, so every path answers it, once.
        worker.execute(() -> {
            Runnable answered;
            try {
                answered = saveAnswer(Core.call(capture), callback);
            } catch (RuntimeException e) {
                Log.w(TAG, "The save request was not answered: " + e.getClass().getSimpleName());
                answered = () -> callback.onFailure(getString(R.string.autofill_save_failed));
            }
            answered.run();
        });
    }

    /** How the callback is answered for the core's answer to a capture. */
    private Runnable saveAnswer(JSONObject answer, SaveCallback callback) {
        String waiting = answer.optString("capture");
        if ((Core.ok(answer) || Core.LOCKED.equals(Core.status(answer))) && !waiting.isEmpty()) {
            Intent review = new Intent(this, SaveActivity.class).putExtra(SaveActivity.CAPTURE, waiting);
            IntentSender sender = Responses.sender(this, review, false);
            return () -> callback.onSuccess(sender);
        }
        if (Core.NONE.equals(Core.status(answer))) {
            return callback::onSuccess;
        }
        String failed = getString(R.string.autofill_save_failed);
        return () -> callback.onFailure(failed);
    }

    /** null where the client state records no requester. */
    private static JSONObject requester(Bundle state) {
        String requester = state != null ? state.getString(Responses.REQUESTER) : null;
        if (requester == null) {
            return null;
        }
        try {
            return new JSONObject(requester);
        } catch (JSONException e) {
            return null;
        }
    }
}
