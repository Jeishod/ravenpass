package com.dortanes.ravenpass.autofill;

import android.app.assist.AssistStructure;
import android.os.Build;
import android.view.autofill.AutofillManager;
import android.view.inputmethod.InlineSuggestionsRequest;

import androidx.core.content.IntentCompat;
import androidx.credentials.provider.BeginGetCredentialRequest;
import androidx.credentials.provider.PendingIntentHandler;

import org.json.JSONException;
import org.json.JSONObject;

import java.util.Map;

/** Answers with the fill entries, a sign-in's passkey entries, or only that the vault is open, as its starter needs. */
public final class UnlockActivity extends PageActivity {
    private boolean asked;

    // Bridge shows BiometricPrompt in the resumed activity: the unlock starts once this one resumes.
    @Override
    protected void onResume() {
        super.onResume();
        if (!asked) {
            asked = true;
            work(() -> Core.call(Core.request("methods")), this::offer);
        }
    }

    private void offer(JSONObject methods) {
        if (methods == null || !Core.ok(methods)) {
            cancel();
            return;
        }
        if (!methods.optBoolean("biometry")) {
            askForPin(methods);
            return;
        }
        work(() -> Core.call(Core.request("unlock")), answer -> {
            if (answer != null && Core.ok(answer)) {
                opened();
            } else {
                askForPin(methods);
            }
        });
    }

    /** Shown only here, the page's loading never delays the device's own unlock. */
    private void askForPin(JSONObject methods) {
        if (!methods.optBoolean("pin")) {
            cancel();
            return;
        }
        showPage();
        try {
            open(methods.put("screen", "unlock"));
        } catch (JSONException e) {
            cancel();
        }
    }

    @Override
    protected boolean serve(AutofillPage.Request request) {
        if (!"unlock".equals(request.op)) {
            return false;
        }
        String pin = request.fields.optString("pin");
        work(() -> {
            Map<String, Object> unlock = Core.request("unlock");
            if (!pin.isEmpty()) {
                unlock.put("pin", pin);
            }
            return Core.call(unlock);
        }, answer -> {
            JSONObject answered = answer != null ? answer : Core.answer(Core.FAILED);
            request.answer(answered);
            if (Core.ok(answered)) {
                opened();
            }
        });
        return true;
    }

    private void opened() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            BeginGetCredentialRequest signIn = PendingIntentHandler.retrieveBeginGetCredentialRequest(getIntent());
            if (signIn != null) {
                work(() -> CredentialEntries.unlocked(this, signIn), result -> {
                    if (result == null) {
                        cancel();
                        return;
                    }
                    setResult(RESULT_OK, result);
                    finish();
                });
                return;
            }
        }
        AssistStructure structure = IntentCompat.getParcelableExtra(getIntent(),
                AutofillManager.EXTRA_ASSIST_STRUCTURE, AssistStructure.class);
        if (structure == null) {
            setResult(RESULT_OK);
            finish();
            return;
        }
        InlineSuggestionsRequest inline = IntentCompat.getParcelableExtra(getIntent(),
                AutofillManager.EXTRA_INLINE_SUGGESTIONS_REQUEST, InlineSuggestionsRequest.class);
        Screen screen = Screen.of(structure);
        work(() -> Responses.fill(this, screen, Core.call(screen.suggest(getPackageManager())), inline), response -> {
            if (response != null) {
                authenticated(response);
            } else {
                cancel();
            }
        });
    }
}
