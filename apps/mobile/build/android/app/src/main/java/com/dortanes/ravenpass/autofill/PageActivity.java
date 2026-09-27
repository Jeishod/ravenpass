package com.dortanes.ravenpass.autofill;

import android.view.ViewGroup;
import android.widget.FrameLayout;

import androidx.core.graphics.Insets;
import androidx.core.view.ViewCompat;
import androidx.core.view.WindowCompat;
import androidx.core.view.WindowInsetsCompat;

import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Who is signing in stays with the screen and never comes from the page. */
abstract class PageActivity extends AutofillActivity implements AutofillPage.Receiver {
    /** As many as the icon service fetches at once. */
    private static final int READERS = 4;

    // Reads run beside the worker: a call waiting for the owner must not hold up icons or the language.
    private final ExecutorService reads = Executors.newFixedThreadPool(READERS);
    /** The page's requests for the screen, waiting for it to be known. */
    private final List<AutofillPage.Request> opening = new ArrayList<>();
    private AutofillPage page;
    private JSONObject screen;

    @Override
    protected void onDestroy() {
        reads.shutdownNow();
        if (page != null) {
            ((ViewGroup) page.view().getParent()).removeView(page.view());
            page.destroy();
        }
        super.onDestroy();
    }

    /** The page loads while the screen's first call runs. */
    protected final void showPage() {
        if (page != null) {
            return;
        }
        page = new AutofillPage(this, this);
        FrameLayout root = new FrameLayout(this);
        root.addView(page.view(), ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
        // The page pads itself for the system bars, but for the bottom one while the keyboard covers it.
        ViewCompat.setOnApplyWindowInsetsListener(root, (view, insets) -> {
            Insets keyboard = insets.getInsets(WindowInsetsCompat.Type.ime());
            view.setPadding(0, 0, 0, keyboard.bottom);
            if (keyboard.bottom == 0) {
                return insets;
            }
            Insets bars = insets.getInsets(WindowInsetsCompat.Type.systemBars());
            return new WindowInsetsCompat.Builder(insets)
                    .setInsets(WindowInsetsCompat.Type.systemBars(), Insets.of(bars.left, bars.top, bars.right, 0))
                    .build();
        });
        setContentView(root);
        page.load();
    }

    /** Hands the page the screen it opens on, now or once it asks. */
    protected final void open(JSONObject opened) {
        screen = opened;
        opening.forEach(request -> request.answer(opened));
        opening.clear();
    }

    /** Serves a request only this screen knows, returning false for any other. */
    protected abstract boolean serve(AutofillPage.Request request);

    @Override
    public final void receive(AutofillPage.Request request) {
        switch (request.op) {
            case "ready" -> {
            }
            case "open" -> {
                if (screen == null) {
                    opening.add(request);
                } else {
                    request.answer(screen);
                }
            }
            case "cancel" -> cancel();
            case "keyboard" -> showKeyboard();
            case "icon" -> read(request, Map.of("site", request.fields.optString("site")));
            case "language" -> read(request, Map.of());
            case "interface-size" -> read(request, Map.of());
            default -> {
                if (!serve(request)) {
                    request.answer(Core.FAILED);
                }
            }
        }
    }

    @Override
    public final void lost() {
        cancel();
    }

    private void read(AutofillPage.Request request, Map<String, Object> fields) {
        Map<String, Object> call = Core.request(request.op);
        call.putAll(fields);
        run(reads, () -> Core.call(call), answer -> request.answer(answer != null ? answer : Core.answer(Core.FAILED)));
    }

    private void showKeyboard() {
        page.view().requestFocus();
        WindowCompat.getInsetsController(getWindow(), page.view()).show(WindowInsetsCompat.Type.ime());
    }
}
