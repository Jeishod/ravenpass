package com.dortanes.ravenpass.autofill;

import android.content.Context;
import android.content.Intent;
import android.content.res.Resources;
import android.os.Build;
import android.util.Base64;

import androidx.annotation.RequiresApi;
import androidx.annotation.StringRes;
import androidx.credentials.CreatePublicKeyCredentialRequest;
import androidx.credentials.CreatePublicKeyCredentialResponse;
import androidx.credentials.CredentialOption;
import androidx.credentials.GetCredentialResponse;
import androidx.credentials.GetPublicKeyCredentialOption;
import androidx.credentials.PublicKeyCredential;
import androidx.credentials.exceptions.CreateCredentialUnknownException;
import androidx.credentials.exceptions.GetCredentialUnknownException;
import androidx.credentials.exceptions.domerrors.InvalidStateError;
import androidx.credentials.exceptions.publickeycredential.CreatePublicKeyCredentialDomException;
import androidx.credentials.provider.CallingAppInfo;
import androidx.credentials.provider.PendingIntentHandler;
import androidx.credentials.provider.ProviderCreateCredentialRequest;
import androidx.credentials.provider.ProviderGetCredentialRequest;

import com.wails.app.R;

import org.json.JSONObject;

import java.util.Map;

/** The system adds the request and its caller to the entry's intent; the entry adds only which passkey. */
@RequiresApi(Build.VERSION_CODES.UPSIDE_DOWN_CAKE)
abstract class PasskeyRequest {
    private static final String CREDENTIAL = "com.dortanes.ravenpass.autofill.CREDENTIAL";
    private static final String PASSKEY = "com.dortanes.ravenpass.autofill.PASSKEY";

    private final CallingAppInfo caller;
    private final String options;
    private final byte[] clientDataHash;
    /** Built by the first call, on the worker. */
    private Map<String, Object> named;

    private PasskeyRequest(CallingAppInfo caller, String options, byte[] clientDataHash) {
        this.caller = caller;
        this.options = options;
        this.clientDataHash = clientDataHash;
    }

    /** id is the vault credential that holds the passkey credentialId. */
    static Intent signIn(Context context, String id, String credentialId) {
        return new Intent(context, PasskeyActivity.class).putExtra(CREDENTIAL, id).putExtra(PASSKEY, credentialId);
    }

    static Intent creation(Context context) {
        return new Intent(context, PasskeyActivity.class);
    }

    /** null for an intent without one. */
    static PasskeyRequest read(Intent intent) {
        ProviderGetCredentialRequest get = PendingIntentHandler.retrieveProviderGetCredentialRequest(intent);
        String id = intent.getStringExtra(CREDENTIAL);
        String credentialId = intent.getStringExtra(PASSKEY);
        if (get != null && id != null && credentialId != null) {
            for (CredentialOption option : get.getCredentialOptions()) {
                if (option instanceof GetPublicKeyCredentialOption passkey) {
                    return new SignIn(get.getCallingAppInfo(), passkey, id, credentialId);
                }
            }
            return null;
        }
        ProviderCreateCredentialRequest create = PendingIntentHandler.retrieveProviderCreateCredentialRequest(intent);
        if (create != null && create.getCallingRequest() instanceof CreatePublicKeyCredentialRequest passkey) {
            return new Creation(create.getCallingAppInfo(), passkey);
        }
        return null;
    }

    /** pin is empty for the device's unlock. Worker only: the first call asks the core about the caller. */
    final Map<String, Object> call(String pin) {
        if (named == null) {
            named = Caller.of(caller);
        }
        Map<String, Object> call = Core.request(op());
        call.put("caller", named);
        call.put("request", options);
        if (clientDataHash != null) {
            call.put("clientDataHash", Base64.encodeToString(clientDataHash, Base64.NO_WRAP));
        }
        call.put("pin", pin);
        name(call);
        return call;
    }

    abstract String op();

    /** Adds to the core's request which passkey it is for. */
    void name(Map<String, Object> call) {
    }

    /** The result that hands the caller the core's response. */
    abstract Intent answered(String response);

    /** The result that tells the caller the request failed as the core's answer says. */
    abstract Intent failed(JSONObject answer);

    /** What the owner reads when the request failed as the core's answer says; ID_NULL for nothing. */
    @StringRes
    abstract int failure(JSONObject answer);

    private static final class SignIn extends PasskeyRequest {
        private final String id;
        private final String credentialId;

        SignIn(CallingAppInfo caller, GetPublicKeyCredentialOption option, String id, String credentialId) {
            super(caller, option.getRequestJson(), option.getClientDataHash());
            this.id = id;
            this.credentialId = credentialId;
        }

        @Override
        String op() {
            return "sign-passkey";
        }

        @Override
        void name(Map<String, Object> call) {
            call.put("id", id);
            call.put("credentialId", credentialId);
        }

        @Override
        Intent answered(String response) {
            Intent result = new Intent();
            PendingIntentHandler.setGetCredentialResponse(result, new GetCredentialResponse(new PublicKeyCredential(response)));
            return result;
        }

        @Override
        Intent failed(JSONObject answer) {
            Intent result = new Intent();
            PendingIntentHandler.setGetCredentialException(result, new GetCredentialUnknownException());
            return result;
        }

        @Override
        int failure(JSONObject answer) {
            return R.string.passkey_sign_in_failed;
        }
    }

    /** One meeting a passkey it excluded is refused as WebAuthn's InvalidStateError, which the relying party reports. */
    private static final class Creation extends PasskeyRequest {
        Creation(CallingAppInfo caller, CreatePublicKeyCredentialRequest request) {
            super(caller, request.getRequestJson(), request.getClientDataHash());
        }

        @Override
        String op() {
            return "create-passkey";
        }

        @Override
        Intent answered(String response) {
            Intent result = new Intent();
            PendingIntentHandler.setCreateCredentialResponse(result, new CreatePublicKeyCredentialResponse(response));
            return result;
        }

        @Override
        Intent failed(JSONObject answer) {
            Intent result = new Intent();
            PendingIntentHandler.setCreateCredentialException(result, excluded(answer)
                    ? new CreatePublicKeyCredentialDomException(new InvalidStateError())
                    : new CreateCredentialUnknownException());
            return result;
        }

        @Override
        int failure(JSONObject answer) {
            return excluded(answer) ? Resources.ID_NULL : R.string.passkey_save_failed;
        }

        private static boolean excluded(JSONObject answer) {
            return answer != null && Core.EXCLUDED.equals(Core.status(answer));
        }
    }
}
