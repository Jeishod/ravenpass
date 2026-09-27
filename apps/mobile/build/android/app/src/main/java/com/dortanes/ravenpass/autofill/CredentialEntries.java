package com.dortanes.ravenpass.autofill;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.os.Build;

import androidx.annotation.RequiresApi;
import androidx.credentials.provider.AuthenticationAction;
import androidx.credentials.provider.BeginCreateCredentialRequest;
import androidx.credentials.provider.BeginCreateCredentialResponse;
import androidx.credentials.provider.BeginCreatePublicKeyCredentialRequest;
import androidx.credentials.provider.BeginGetCredentialOption;
import androidx.credentials.provider.BeginGetCredentialRequest;
import androidx.credentials.provider.BeginGetCredentialResponse;
import androidx.credentials.provider.BeginGetPublicKeyCredentialOption;
import androidx.credentials.provider.CallingAppInfo;
import androidx.credentials.provider.CreateEntry;
import androidx.credentials.provider.PendingIntentHandler;
import androidx.credentials.provider.PublicKeyCredentialEntry;

import com.wails.app.R;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.Map;

/** The system adds the request to each entry's mutable PendingIntent. */
@RequiresApi(Build.VERSION_CODES.UPSIDE_DOWN_CAKE)
final class CredentialEntries {
    private CredentialEntries() {
    }

    /** Off the main thread only: it asks the core. A locked vault offers only the unlock action. */
    static BeginGetCredentialResponse signIn(Context context, BeginGetCredentialRequest request) {
        BeginGetCredentialResponse.Builder response = new BeginGetCredentialResponse.Builder();
        CallingAppInfo info = request.getCallingAppInfo();
        if (info == null) {
            return response.build();
        }
        Map<String, Object> caller = null;
        for (BeginGetCredentialOption option : request.getBeginGetCredentialOptions()) {
            if (!(option instanceof BeginGetPublicKeyCredentialOption passkey)) {
                continue;
            }
            if (caller == null) {
                caller = Caller.of(info);
            }
            Map<String, Object> call = Core.request("passkeys");
            call.put("caller", caller);
            call.put("request", passkey.getRequestJson());
            JSONObject answer = Core.call(call);
            if (Core.LOCKED.equals(Core.status(answer))) {
                return new BeginGetCredentialResponse.Builder().addAuthenticationAction(unlock(context)).build();
            }
            JSONArray passkeys = Core.ok(answer) ? answer.optJSONArray("passkeys") : null;
            for (int i = 0; passkeys != null && i < passkeys.length(); i++) {
                JSONObject found = passkeys.optJSONObject(i);
                if (found != null) {
                    response.addCredentialEntry(entry(context, passkey, found));
                }
            }
        }
        return response.build();
    }

    /** The unlock action's result: the entries for the sign-in it was chosen for. */
    static Intent unlocked(Context context, BeginGetCredentialRequest request) {
        Intent result = new Intent();
        PendingIntentHandler.setBeginGetCredentialResponse(result, signIn(context, request));
        return result;
    }

    /** One entry for a passkey, none for any other credential. */
    static BeginCreateCredentialResponse creation(Context context, BeginCreateCredentialRequest request) {
        BeginCreateCredentialResponse.Builder response = new BeginCreateCredentialResponse.Builder();
        if (request instanceof BeginCreatePublicKeyCredentialRequest) {
            response.addCreateEntry(new CreateEntry.Builder(context.getString(R.string.app_name),
                    pending(context, PasskeyRequest.creation(context))).build());
        }
        return response.build();
    }

    /** The core's name for the passkey over the relying party's display name. */
    private static PublicKeyCredentialEntry entry(Context context, BeginGetPublicKeyCredentialOption option,
            JSONObject passkey) {
        Intent signIn = PasskeyRequest.signIn(context, passkey.optString("id"), passkey.optString("credentialId"));
        PublicKeyCredentialEntry.Builder entry = new PublicKeyCredentialEntry.Builder(context,
                passkey.optString("account"), pending(context, signIn), option);
        String displayName = passkey.optString("displayName");
        if (!displayName.isEmpty()) {
            entry.setDisplayName(displayName);
        }
        return entry.build();
    }

    private static AuthenticationAction unlock(Context context) {
        return new AuthenticationAction.Builder(context.getString(R.string.autofill_unlock),
                pending(context, new Intent(context, UnlockActivity.class))).build();
    }

    private static PendingIntent pending(Context context, Intent intent) {
        return Responses.pending(context, intent, true);
    }
}
