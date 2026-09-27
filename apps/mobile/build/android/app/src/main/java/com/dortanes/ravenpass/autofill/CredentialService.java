package com.dortanes.ravenpass.autofill;

import android.os.Build;
import android.os.CancellationSignal;
import android.os.OutcomeReceiver;
import android.util.Log;

import androidx.annotation.NonNull;
import androidx.annotation.RequiresApi;
import androidx.credentials.exceptions.ClearCredentialException;
import androidx.credentials.exceptions.CreateCredentialException;
import androidx.credentials.exceptions.GetCredentialException;
import androidx.credentials.provider.BeginCreateCredentialRequest;
import androidx.credentials.provider.BeginCreateCredentialResponse;
import androidx.credentials.provider.BeginGetCredentialRequest;
import androidx.credentials.provider.BeginGetCredentialResponse;
import androidx.credentials.provider.CredentialProviderService;
import androidx.credentials.provider.ProviderClearCredentialStateRequest;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

/** The system waits about 3 seconds for a sign-in's entries, which may start the core and fetch Asset Links. */
@RequiresApi(Build.VERSION_CODES.UPSIDE_DOWN_CAKE)
public final class CredentialService extends CredentialProviderService {
    private static final String TAG = "RavenpassCredentials";

    private ExecutorService worker;

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
    public void onBeginGetCredentialRequest(@NonNull BeginGetCredentialRequest request,
            @NonNull CancellationSignal cancellation,
            @NonNull OutcomeReceiver<BeginGetCredentialResponse, GetCredentialException> callback) {
        Future<?> answering = worker.submit(() -> {
            BeginGetCredentialResponse response = entries(request);
            if (!cancellation.isCanceled()) {
                callback.onResult(response);
            }
        });
        cancellation.setOnCancelListener(() -> answering.cancel(true));
    }

    @Override
    public void onBeginCreateCredentialRequest(@NonNull BeginCreateCredentialRequest request,
            @NonNull CancellationSignal cancellation,
            @NonNull OutcomeReceiver<BeginCreateCredentialResponse, CreateCredentialException> callback) {
        callback.onResult(CredentialEntries.creation(this, request));
    }

    @Override
    public void onClearCredentialStateRequest(@NonNull ProviderClearCredentialStateRequest request,
            @NonNull CancellationSignal cancellation,
            @NonNull OutcomeReceiver<Void, ClearCredentialException> callback) {
        callback.onResult(null);
    }

    private BeginGetCredentialResponse entries(BeginGetCredentialRequest request) {
        try {
            return CredentialEntries.signIn(this, request);
        } catch (RuntimeException e) {
            Log.w(TAG, "The sign-in request was not answered: " + e.getClass().getSimpleName());
            return new BeginGetCredentialResponse.Builder().build();
        }
    }
}
