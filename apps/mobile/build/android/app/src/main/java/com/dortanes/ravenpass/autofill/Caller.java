package com.dortanes.ravenpass.autofill;

import androidx.credentials.provider.CallingAppInfo;

import java.util.HashMap;
import java.util.Map;

/** CallingAppInfo returns a web origin only for an app on the allowlist, which the core supplies. */
final class Caller {
    private Caller() {
    }

    /** Off the main thread only: it asks the core for the allowlist. */
    static Map<String, Object> of(CallingAppInfo info) {
        Map<String, Object> caller = new HashMap<>();
        caller.put("package", info.getPackageName());
        caller.put("signers", Signers.of(info.getSigningInfo()));
        String origin = origin(info);
        if (origin != null) {
            caller.put("origin", origin);
        }
        return caller;
    }

    /** null for an app off the core's allowlist. */
    private static String origin(CallingAppInfo info) {
        if (!info.isOriginPopulated()) {
            return null;
        }
        String allowlist = Core.call(Core.request("privileged-apps")).optString("allowlist");
        try {
            return info.getOrigin(allowlist);
        } catch (IllegalArgumentException | IllegalStateException e) {
            return null;
        }
    }
}
