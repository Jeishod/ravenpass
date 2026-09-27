package com.dortanes.ravenpass.autofill;

import android.content.Context;
import android.content.Intent;
import android.service.autofill.Dataset;
import android.view.autofill.AutofillId;

import androidx.core.content.IntentCompat;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

/** Travels in the fill and search screens' intents, which only this app's PendingIntents start. */
final class Destination {
    private static final String REQUESTER = "com.dortanes.ravenpass.autofill.REQUESTER";
    private static final String CODE = "com.dortanes.ravenpass.autofill.CODE";
    private static final String EMAIL = "com.dortanes.ravenpass.autofill.EMAIL";
    private static final String USERNAME_FIELD = "com.dortanes.ravenpass.autofill.USERNAME_FIELD";
    private static final String PASSWORD_FIELD = "com.dortanes.ravenpass.autofill.PASSWORD_FIELD";
    private static final String CODE_FIELDS = "com.dortanes.ravenpass.autofill.CODE_FIELDS";

    final JSONObject requester;
    /** The form takes a one-time code, not a sign-in. */
    final boolean code;
    /** The login field asks for an email address. */
    private final boolean email;
    private final AutofillId username;
    private final AutofillId password;
    /** The fields a code fills, in order: one field, or each box of a split code field. */
    private final ArrayList<AutofillId> codeFields;

    private Destination(JSONObject requester, boolean code, boolean email, AutofillId username,
            AutofillId password, ArrayList<AutofillId> codeFields) {
        this.requester = requester;
        this.code = code;
        this.email = email;
        this.username = username;
        this.password = password;
        this.codeFields = codeFields != null ? codeFields : new ArrayList<>();
    }

    /** null without a requester. */
    static Destination of(JSONObject answer, Screen screen) {
        JSONObject requester = answer.optJSONObject("requester");
        JSONObject form = answer.optJSONObject("form");
        if (requester == null || form == null) {
            return null;
        }
        return new Destination(requester, "code".equals(form.optString("kind")), form.optBoolean("email"),
                screen.id(form.optInt("username", -1)), screen.id(form.optInt("password", -1)),
                screen.ids(form.optJSONArray("code")));
    }

    /** null for an intent without one. */
    static Destination read(Intent intent) {
        String requester = intent.getStringExtra(REQUESTER);
        if (requester == null) {
            return null;
        }
        try {
            return new Destination(new JSONObject(requester), intent.getBooleanExtra(CODE, false),
                    intent.getBooleanExtra(EMAIL, false),
                    IntentCompat.getParcelableExtra(intent, USERNAME_FIELD, AutofillId.class),
                    IntentCompat.getParcelableExtra(intent, PASSWORD_FIELD, AutofillId.class),
                    IntentCompat.getParcelableArrayListExtra(intent, CODE_FIELDS, AutofillId.class));
        } catch (JSONException e) {
            return null;
        }
    }

    Intent write(Intent intent) {
        return intent.putExtra(REQUESTER, requester.toString())
                .putExtra(CODE, code)
                .putExtra(EMAIL, email)
                .putExtra(USERNAME_FIELD, username)
                .putExtra(PASSWORD_FIELD, password)
                .putParcelableArrayListExtra(CODE_FIELDS, codeFields);
    }

    AutofillId[] fields() {
        List<AutofillId> fields = new ArrayList<>();
        for (AutofillId field : code ? codeFields : Arrays.asList(username, password)) {
            if (field != null) {
                fields.add(field);
            }
        }
        return fields.toArray(new AutofillId[0]);
    }

    Map<String, Object> request(String op) {
        Map<String, Object> request = Core.request(op);
        request.put("requester", requester);
        return request;
    }

    /** The core checks that the credential still matches before it releases the sign-in or the code. */
    JSONObject release(String id) {
        Map<String, Object> request = request(code ? "code" : "fill");
        request.put("id", id);
        if (code) {
            request.put("fields", codeFields.size());
        } else {
            request.put("email", email);
        }
        return Core.call(request);
    }

    /** null when the core released nothing. */
    Dataset filled(Context context, JSONObject answer, String label) {
        if (answer == null || !Core.ok(answer)) {
            return null;
        }
        Responses.Filled filled = new Responses.Filled(context, label);
        if (code) {
            JSONArray entries = answer.optJSONArray("entries");
            for (int i = 0; entries != null && i < entries.length() && i < codeFields.size(); i++) {
                filled.set(codeFields.get(i), entries.optString(i));
            }
        } else {
            filled.set(username, answer.optString("username"));
            filled.set(password, answer.optString("password"));
        }
        return filled.build();
    }
}
