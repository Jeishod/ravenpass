package com.dortanes.ravenpass;

import android.content.ContentProvider;
import android.content.ContentResolver;
import android.content.ContentValues;
import android.content.Context;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import android.webkit.MimeTypeMap;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;

import java.io.FileNotFoundException;
import java.io.FileOutputStream;
import java.io.IOException;
import java.util.Arrays;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicReference;

/** Serves the last copied scan from memory, never a file, at an address of its own that an earlier grant cannot read. */
public final class ClipProvider extends ContentProvider {
    private static final String[] COLUMNS = {OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE};
    private static final AtomicReference<Copy> HELD = new AtomicReference<>();

    /** The provider keeps the array, which drop wipes. */
    static Uri hold(Context context, byte[] content, String mediaType, CharSequence label) {
        Uri address = new Uri.Builder()
                .scheme(ContentResolver.SCHEME_CONTENT)
                .authority(context.getPackageName() + ".clip")
                .appendPath(UUID.randomUUID().toString())
                .build();
        String extension = MimeTypeMap.getSingleton().getExtensionFromMimeType(mediaType);
        String name = extension != null ? label + "." + extension : label.toString();
        wipe(HELD.getAndSet(new Copy(address, content, mediaType, name)));
        return address;
    }

    static void drop() {
        wipe(HELD.getAndSet(null));
    }

    @Override
    public boolean onCreate() {
        return true;
    }

    @Nullable
    @Override
    public String getType(@NonNull Uri uri) {
        Copy copy = held(uri);
        return copy != null ? copy.mediaType : null;
    }

    @Nullable
    @Override
    public ParcelFileDescriptor openFile(@NonNull Uri uri, @NonNull String mode) throws FileNotFoundException {
        Copy copy = held(uri);
        if (copy == null || !"r".equals(mode)) {
            throw new FileNotFoundException("No copied scan at this address");
        }
        return openPipeHelper(uri, copy.mediaType, null, copy.content, (output, address, type, options, content) -> {
            // openPipeHelper closes the write end once this returns.
            try {
                new FileOutputStream(output.getFileDescriptor()).write(content);
            } catch (IOException e) {
                // The pasting app closed its end before reading the whole scan.
            }
        });
    }

    @Nullable
    @Override
    public Cursor query(@NonNull Uri uri, @Nullable String[] projection, @Nullable String selection,
            @Nullable String[] selectionArgs, @Nullable String sortOrder) {
        Copy copy = held(uri);
        if (copy == null) {
            return null;
        }
        MatrixCursor cursor = new MatrixCursor(projection != null ? projection : COLUMNS, 1);
        cursor.newRow()
                .add(OpenableColumns.DISPLAY_NAME, copy.name)
                .add(OpenableColumns.SIZE, copy.content.length);
        return cursor;
    }

    @Nullable
    @Override
    public Uri insert(@NonNull Uri uri, @Nullable ContentValues values) {
        return null;
    }

    @Override
    public int update(@NonNull Uri uri, @Nullable ContentValues values, @Nullable String selection,
            @Nullable String[] selectionArgs) {
        return 0;
    }

    @Override
    public int delete(@NonNull Uri uri, @Nullable String selection, @Nullable String[] selectionArgs) {
        return 0;
    }

    @Nullable
    private static Copy held(Uri uri) {
        Copy copy = HELD.get();
        return copy != null && copy.address.equals(uri) ? copy : null;
    }

    private static void wipe(@Nullable Copy copy) {
        if (copy != null) {
            Arrays.fill(copy.content, (byte) 0);
        }
    }

    private static final class Copy {
        final Uri address;
        final byte[] content;
        final String mediaType;
        final String name;

        Copy(Uri address, byte[] content, String mediaType, String name) {
            this.address = address;
            this.content = content;
            this.mediaType = mediaType;
            this.name = name;
        }
    }
}
