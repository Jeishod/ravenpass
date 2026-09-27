package com.dortanes.ravenpass.autofill;

import android.os.Bundle;
import android.service.autofill.Dataset;
import android.widget.Toast;

import com.wails.app.R;

/** Shows nothing while the vault is open; a vault that locked since the entry was offered is unlocked first. */
public final class FillActivity extends AutofillActivity {
    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        Destination destination = Destination.read(getIntent());
        String id = getIntent().getStringExtra(Responses.CREDENTIAL);
        String label = getIntent().getStringExtra(Responses.LABEL);
        if (destination == null || id == null) {
            cancel();
            return;
        }
        ask(() -> destination.release(id), answer -> {
            Dataset dataset = destination.filled(this, answer, label != null ? label : "");
            if (dataset != null) {
                authenticated(dataset);
                return;
            }
            // The activity's own text is in the chosen language; the application's is not before API 33.
            Toast.makeText(getApplicationContext(),
                    getText(destination.code ? R.string.autofill_code_failed : R.string.autofill_fill_failed),
                    Toast.LENGTH_SHORT).show();
            cancel();
        });
    }
}
