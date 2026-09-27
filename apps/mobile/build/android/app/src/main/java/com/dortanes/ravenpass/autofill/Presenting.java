package com.dortanes.ravenpass.autofill;

import android.annotation.SuppressLint;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.graphics.Bitmap;
import android.graphics.BlendMode;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.drawable.Icon;
import android.service.autofill.InlinePresentation;
import android.view.View;
import android.view.inputmethod.InlineSuggestionsRequest;
import android.widget.RemoteViews;
import android.widget.inline.InlinePresentationSpec;

import androidx.autofill.inline.UiVersions;
import androidx.autofill.inline.v1.InlineSuggestionUi;

import com.wails.app.MainActivity;
import com.wails.app.R;

import java.io.ByteArrayOutputStream;
import java.util.List;
import java.util.Locale;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** The keyboard takes at most its own count of chips, each styled by the spec at its position, the last for the rest. */
final class Presenting {
    // The interface's compact letter tile, in dp: its side, its corners' radius and its letter's size.
    private static final float TILE_SIDE = 24;
    private static final float TILE_RADIUS = 7;
    private static final float TILE_LETTER = 11;
    // A letter or a digit, as the interface's tiles take it.
    private static final Pattern LETTER = Pattern.compile("[\\p{L}\\p{N}]");

    private final Context context;
    private final List<InlinePresentationSpec> specs;
    private final int chips;
    private final PendingIntent attribution;

    private Presenting(Context context, List<InlinePresentationSpec> specs, int chips) {
        this.context = context;
        this.specs = specs;
        this.chips = chips;
        // Long-pressing a chip names who offered it and opens Ravenpass.
        this.attribution = PendingIntent.getActivity(context, 0, new Intent(context, MainActivity.class),
                PendingIntent.FLAG_IMMUTABLE);
    }

    /** A null request offers no chips. */
    static Presenting of(Context context, InlineSuggestionsRequest request) {
        if (request == null || request.getInlinePresentationSpecs().isEmpty()) {
            return new Presenting(context, List.of(), 0);
        }
        return new Presenting(context, request.getInlinePresentationSpecs(), request.getMaxSuggestionCount());
    }

    int chips() {
        return chips;
    }

    static RemoteViews entryMenu(Context context, CharSequence action) {
        return menu(context, context.getString(R.string.app_name), action);
    }

    static RemoteViews menu(Context context, CharSequence title, CharSequence subtitle) {
        RemoteViews line = new RemoteViews(context.getPackageName(), R.layout.autofill_item);
        line.setTextViewText(R.id.autofill_title, title);
        if (subtitle == null || subtitle.length() == 0) {
            line.setViewVisibility(R.id.autofill_subtitle, View.GONE);
        } else {
            line.setTextViewText(R.id.autofill_subtitle, subtitle);
        }
        return line;
    }

    static RemoteViews menu(Context context, Icon face, CharSequence title, CharSequence subtitle) {
        RemoteViews line = menu(context, title, subtitle);
        line.setImageViewIcon(R.id.autofill_icon, face);
        return line;
    }

    /** The site's PNG icon, else the label's first letter or digit on the interface's tile. */
    Icon face(CharSequence label, byte[] siteIcon) {
        return untinted(siteIcon != null ? Icon.createWithData(siteIcon, 0, siteIcon.length) : tile(label));
    }

    /** null where the keyboard takes no chip at position or cannot draw this style. */
    InlinePresentation chip(int position, Icon face, CharSequence title, CharSequence subtitle) {
        InlineSuggestionUi.Content.Builder content = InlineSuggestionUi.newContentBuilder(attribution)
                .setStartIcon(face)
                .setTitle(title)
                .setContentDescription(title);
        if (subtitle != null && subtitle.length() > 0) {
            content.setSubtitle(subtitle);
        }
        return chip(position, content.build(), false);
    }

    /** description is for screen readers. */
    InlinePresentation entry(int position, CharSequence action, CharSequence description) {
        InlineSuggestionUi.Content content = InlineSuggestionUi.newContentBuilder(attribution)
                .setStartIcon(mark())
                .setTitle(context.getString(R.string.app_name))
                .setSubtitle(action)
                .setContentDescription(description)
                .build();
        return chip(position, content, false);
    }

    /** Stays at the keyboard's edge. */
    InlinePresentation pinned(int position, CharSequence description) {
        InlineSuggestionUi.Content content = InlineSuggestionUi.newContentBuilder(attribution)
                .setStartIcon(mark())
                .setContentDescription(description)
                .build();
        return chip(position, content, true);
    }

    // Slice is deprecated since Android 15 and getSlice() is library-only, yet InlinePresentation takes nothing else.
    @SuppressWarnings("deprecation")
    @SuppressLint("RestrictedApi")
    private InlinePresentation chip(int position, InlineSuggestionUi.Content content, boolean pinned) {
        if (position >= chips) {
            return null;
        }
        InlinePresentationSpec spec = specs.get(Math.min(position, specs.size() - 1));
        if (!UiVersions.getVersions(spec.getStyle()).contains(UiVersions.INLINE_UI_VERSION_1)) {
            return null;
        }
        return new InlinePresentation(content.getSlice(), spec, pinned);
    }

    private Icon mark() {
        return untinted(Icon.createWithResource(context, R.drawable.autofill_mark));
    }

    /** Keeps a keyboard's style from tinting the icon. */
    private static Icon untinted(Icon icon) {
        return icon.setTintBlendMode(BlendMode.DST);
    }

    // An icon from a bitmap parcels its pixels uncompressed into every dataset; the tile travels as PNG.
    private Icon tile(CharSequence label) {
        float density = context.getResources().getDisplayMetrics().density;
        int side = Math.round(TILE_SIDE * density);
        float radius = TILE_RADIUS * density;
        Bitmap tile = Bitmap.createBitmap(side, side, Bitmap.Config.ARGB_8888);
        Canvas canvas = new Canvas(tile);
        Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
        paint.setColor(context.getColor(R.color.tile));
        canvas.drawRoundRect(0, 0, side, side, radius, radius, paint);
        paint.setColor(context.getColor(R.color.tile_letter));
        paint.setTextSize(TILE_LETTER * density);
        paint.setTextAlign(Paint.Align.CENTER);
        Paint.FontMetrics metrics = paint.getFontMetrics();
        canvas.drawText(initial(label), side / 2f, (side - metrics.ascent - metrics.descent) / 2f, paint);
        ByteArrayOutputStream png = new ByteArrayOutputStream();
        tile.compress(Bitmap.CompressFormat.PNG, 100, png);
        tile.recycle();
        return Icon.createWithData(png.toByteArray(), 0, png.size());
    }

    /** Empty for a label without a letter or digit. */
    private static String initial(CharSequence label) {
        Matcher letter = LETTER.matcher(label);
        return letter.find() ? letter.group().toUpperCase(Locale.getDefault()) : "";
    }
}
