package com.dortanes.ravenpass.autofill;

import android.content.res.Resources;
import android.os.Build;
import android.os.Bundle;
import android.util.Log;
import android.widget.Toast;

import androidx.annotation.RequiresApi;

import org.json.JSONException;
import org.json.JSONObject;

import java.util.Set;

/** The owner confirms with the device's unlock over this screen, else with the vault's PIN on the page. */
@RequiresApi(Build.VERSION_CODES.UPSIDE_DOWN_CAKE)
public final class PasskeyActivity extends PageActivity {
    /** The answers the page's unlock screen shows while it waits for another PIN. */
    private static final Set<String> RETRIES = Set.of(Core.WRONG_PIN, Core.PIN_REMOVED, Core.TOO_SOON, Core.CANCELED, Core.PIN);

    private PasskeyRequest request;
    private JSONObject methods;
    private boolean asked;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        request = PasskeyRequest.read(getIntent());
        if (request == null) {
            cancel();
        }
    }

    // Bridge shows BiometricPrompt in the resumed activity: the request starts once this one resumes.
    @Override
    protected void onResume() {
        super.onResume();
        if (request != null && !asked) {
            asked = true;
            work(() -> Core.call(Core.request("methods")), found -> {
                methods = found;
                work(() -> Core.call(request.call("")), this::attempted);
            });
        }
    }

    /** Goes to the PIN where the core asks for it or the owner turned the device's prompt down. */
    private void attempted(JSONObject answer) {
        String status = answer != null ? Core.status(answer) : Core.FAILED;
        if (Core.PIN.equals(status) || Core.CANCELED.equals(status) && pinSet()) {
            askForPin();
        } else if (Core.CANCELED.equals(status)) {
            cancel();
        } else {
            end(answer);
        }
    }

    private boolean pinSet() {
        return methods != null && Core.ok(methods) && methods.optBoolean("pin");
    }

    /** The page's PIN screen also offers the device's unlock where the vault takes it. */
    private void askForPin() {
        if (!pinSet()) {
            end(null);
            return;
        }
        showPage();
        try {
            open(methods.put("screen", "unlock"));
        } catch (JSONException e) {
            end(null);
        }
    }

    @Override
    protected boolean serve(AutofillPage.Request page) {
        if (!"unlock".equals(page.op)) {
            return false;
        }
        String pin = page.fields.optString("pin");
        work(() -> Core.call(request.call(pin)), answer -> {
            JSONObject answered = answer != null ? answer : Core.answer(Core.FAILED);
            page.answer(answered);
            if (!RETRIES.contains(Core.status(answered))) {
                end(answered);
            }
        });
        return true;
    }

    /** A failed request is answered as failed, and the owner is told. */
    private void end(JSONObject answer) {
        String response = answer != null && Core.ok(answer) ? answer.optString("response") : "";
        if (!response.isEmpty()) {
            setResult(RESULT_OK, request.answered(response));
        } else {
            Log.w(TAG, "The passkey request failed: "
                    + (answer != null ? Core.status(answer) + " " + answer.optString("refusal") : "no answer"));
            int failure = request.failure(answer);
            if (failure != Resources.ID_NULL) {
                Toast.makeText(getApplicationContext(), failure, Toast.LENGTH_SHORT).show();
            }
            setResult(RESULT_OK, request.failed(answer));
        }
        finish();
    }
}
