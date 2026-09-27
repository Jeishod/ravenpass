package com.dortanes.ravenpass.autofill;

import android.os.Bundle;
import android.widget.Toast;

import com.wails.app.R;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.util.HashMap;
import java.util.Map;

/** The token that names the sign-in the core holds stays here and never reaches the page. */
public final class SaveActivity extends PageActivity {
    /** The core's offer for a sign-in it holds, as JSON. */
    static final String OFFER = "com.dortanes.ravenpass.autofill.OFFER";
    /** A sign-in waiting in the core for the vault to open. */
    static final String CAPTURE = "com.dortanes.ravenpass.autofill.CAPTURE";

    private static final String UPDATE = "update";

    /** What saving to each offered target does, by the target's id. */
    private final Map<String, String> actions = new HashMap<>();
    private String token;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        // A screen recreated after its process ended has lost the sign-in the core held.
        if (state != null) {
            finish();
            return;
        }
        String offer = getIntent().getStringExtra(OFFER);
        if (offer != null) {
            review(offer);
        } else if (getIntent().hasExtra(CAPTURE)) {
            offerWaiting();
        } else {
            finish();
        }
    }

    private void offerWaiting() {
        String capture = getIntent().getStringExtra(CAPTURE);
        ask(() -> {
            Map<String, Object> request = Core.request("offer");
            request.put("capture", capture);
            return Core.call(request);
        }, answer -> {
            JSONObject offer = answer != null && Core.ok(answer) ? answer.optJSONObject("offer") : null;
            if (offer != null) {
                review(offer.toString());
            } else {
                finish();
            }
        });
    }

    /** Shows the page on the offer, which it receives without the token. */
    private void review(String text) {
        showPage();
        try {
            JSONObject offer = new JSONObject(text);
            token = offer.optString("token");
            offer.remove("token");
            JSONArray targets = offer.optJSONArray("targets");
            for (int i = 0; targets != null && i < targets.length(); i++) {
                JSONObject target = targets.optJSONObject(i);
                if (target != null) {
                    actions.put(target.optString("id"), target.optString("action"));
                }
            }
            open(Core.answer(Core.OK).put("screen", "save").put("offer", offer));
        } catch (JSONException e) {
            finish();
        }
    }

    @Override
    protected boolean serve(AutofillPage.Request request) {
        if (!"save".equals(request.op)) {
            return false;
        }
        String target = request.fields.optString("target");
        Map<String, Object> choice = new HashMap<>();
        choice.put("target", target);
        choice.put("name", target.isEmpty() ? request.fields.optString("name").trim() : "");
        choice.put("account", request.fields.optString("account").trim());
        ask(() -> {
            Map<String, Object> save = Core.request("save");
            save.put("token", token);
            save.put("choice", choice);
            return Core.call(save);
        }, answer -> {
            JSONObject answered = answer != null ? answer : Core.answer(Core.FAILED);
            request.answer(answered);
            if (Core.ok(answered)) {
                // The activity's own text is in the chosen language; the application's is not before API 33.
                Toast.makeText(getApplicationContext(), getText(saved(answered.optBoolean("created"), target)),
                        Toast.LENGTH_SHORT).show();
                finish();
            }
        });
        return true;
    }

    /** What a save did: made a new item, updated a password, or added the site to an item. */
    private int saved(boolean created, String target) {
        if (created) {
            return R.string.autofill_saved_created;
        }
        return UPDATE.equals(actions.get(target)) ? R.string.autofill_saved_updated : R.string.autofill_saved_site_added;
    }
}
