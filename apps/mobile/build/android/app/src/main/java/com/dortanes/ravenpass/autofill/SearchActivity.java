package com.dortanes.ravenpass.autofill;

import android.os.Bundle;
import android.service.autofill.Dataset;

import org.json.JSONException;
import org.json.JSONObject;

import java.util.Map;

/** A credential that does not match yet gets the requester added once the owner agrees on the page. */
public final class SearchActivity extends PageActivity {
    private Destination destination;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        destination = Destination.read(getIntent());
        if (destination == null) {
            cancel();
            return;
        }
        showPage();
        ask(() -> search(""), answer -> {
            if (answer == null || !Core.ok(answer)) {
                cancel();
                return;
            }
            try {
                open(answer.put("screen", "search").put("code", destination.code));
            } catch (JSONException e) {
                cancel();
            }
        });
    }

    @Override
    protected boolean serve(AutofillPage.Request request) {
        switch (request.op) {
            case "search" -> {
                String query = request.fields.optString("query");
                ask(() -> search(query), answer -> request.answer(answer != null ? answer : Core.answer(Core.FAILED)));
                return true;
            }
            case "fill" -> {
                fill(request, request.fields.optString("id"), request.fields.optString("label"),
                        request.fields.optBoolean("add"));
                return true;
            }
            default -> {
                return false;
            }
        }
    }

    private JSONObject search(String query) {
        Map<String, Object> request = destination.request("search");
        request.put("query", query);
        request.put("code", destination.code);
        return Core.call(request);
    }

    /** Fills the credential id, first adding the requester to it when add is set. */
    private void fill(AutofillPage.Request request, String id, String label, boolean add) {
        ask(() -> {
            if (add) {
                Map<String, Object> link = destination.request("link");
                link.put("id", id);
                JSONObject linked = Core.call(link);
                if (!Core.ok(linked)) {
                    return Core.LOCKED.equals(Core.status(linked)) ? linked : Core.answer(Core.NOT_ADDED);
                }
            }
            return destination.release(id);
        }, answer -> {
            Dataset dataset = destination.filled(this, answer, label);
            if (dataset != null) {
                request.answer(Core.OK);
                authenticated(dataset);
                return;
            }
            String status = answer != null ? Core.status(answer) : Core.FAILED;
            request.answer(Core.NOT_ADDED.equals(status) || Core.NO_CODE.equals(status) ? status : Core.FAILED);
        });
    }
}
