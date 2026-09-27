package com.dortanes.ravenpass.autofill;

import org.json.JSONException;
import org.json.JSONObject;

import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/** Requests name "op" and answers "status", both UTF-8 JSON across JNI, whose byte copies are wiped once read. */
final class Core {
    // Statuses shared with the Go handler and the screens' page.
    static final String OK = "ok";
    static final String NONE = "none";
    static final String LOCKED = "locked";
    static final String CANCELED = "canceled";
    static final String NO_CODE = "no-code";
    static final String WRONG_PIN = "wrong-pin";
    static final String PIN_REMOVED = "pin-removed";
    static final String TOO_SOON = "too-soon";
    /** A passkey request the owner confirms with the vault's PIN, which it did not carry. */
    static final String PIN = "pin";
    /** A passkey creation meeting a passkey the vault holds that it excluded. */
    static final String EXCLUDED = "excluded";
    static final String FAILED = "failed";
    /** The screens' page only: a credential the site or app could not be added to. */
    static final String NOT_ADDED = "not-added";

    private Core() {
    }

    private static native byte[] call(byte[] request);

    static Map<String, Object> request(String op) {
        Map<String, Object> request = new HashMap<>();
        request.put("op", op);
        return request;
    }

    /** The main thread sends only ops that wait on neither the owner nor the network; a missing status reads "failed". */
    static JSONObject call(Map<String, Object> request) {
        byte[] sent = new JSONObject(request).toString().getBytes(StandardCharsets.UTF_8);
        byte[] received = null;
        try {
            received = call(sent);
            return received != null ? new JSONObject(new String(received, StandardCharsets.UTF_8)) : answer(FAILED);
        } catch (JSONException e) {
            return answer(FAILED);
        } finally {
            Arrays.fill(sent, (byte) 0);
            if (received != null) {
                Arrays.fill(received, (byte) 0);
            }
        }
    }

    static String status(JSONObject answer) {
        return answer.optString("status", FAILED);
    }

    static boolean ok(JSONObject answer) {
        return OK.equals(status(answer));
    }

    static JSONObject answer(String status) {
        Map<String, Object> answer = new HashMap<>();
        answer.put("status", status);
        return new JSONObject(answer);
    }
}
