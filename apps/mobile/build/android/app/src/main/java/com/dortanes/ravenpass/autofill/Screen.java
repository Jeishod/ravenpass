package com.dortanes.ravenpass.autofill;

import android.app.assist.AssistStructure;
import android.app.assist.AssistStructure.ViewNode;
import android.content.ComponentName;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.os.Build;
import android.service.autofill.FillContext;
import android.util.Pair;
import android.view.View;
import android.view.ViewStructure;
import android.view.autofill.AutofillId;
import android.view.autofill.AutofillValue;

import org.json.JSONArray;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/** Answers name a field by its place in the walk; its web domain and visibility follow the nodes above it. */
final class Screen {
    final String packageName;
    private final List<Map<String, Object>> fields = new ArrayList<>();
    private final List<AutofillId> ids = new ArrayList<>();

    private Screen(String packageName) {
        this.packageName = packageName;
    }

    /** Walks the structure of the app the screen belongs to; one that names no app is answered nothing. */
    static Screen of(AssistStructure structure) {
        ComponentName activity = structure.getActivityComponent();
        Screen screen = new Screen(activity != null ? activity.getPackageName() : "");
        for (int i = 0; i < structure.getWindowNodeCount(); i++) {
            screen.walk(structure.getWindowNodeAt(i).getRootViewNode(), "", "", true);
        }
        return screen;
    }

    /** Off the main thread only: it reads the app's signing certificates from the package manager. */
    Map<String, Object> suggest(PackageManager packages) {
        Map<String, Object> app = new HashMap<>();
        app.put("package", packageName);
        app.put("signers", signers(packages, packageName));
        Map<String, Object> request = Core.request("suggest");
        request.put("app", app);
        request.put("fields", fields);
        return request;
    }

    /** The field numbered index, null for -1 or a number the walk did not give. */
    AutofillId id(int index) {
        return index >= 0 && index < ids.size() ? ids.get(index) : null;
    }

    /** The fields numbered by indexes, in their order, without numbers the walk did not give; none for null. */
    ArrayList<AutofillId> ids(JSONArray indexes) {
        ArrayList<AutofillId> found = new ArrayList<>();
        for (int i = 0; indexes != null && i < indexes.length(); i++) {
            AutofillId field = id(indexes.optInt(i, -1));
            if (field != null) {
                found.add(field);
            }
        }
        return found;
    }

    /** The text the latest of the contexts holds in the field id, empty for none. */
    static String text(List<FillContext> contexts, AutofillId id) {
        if (id == null) {
            return "";
        }
        for (int i = contexts.size() - 1; i >= 0; i--) {
            AssistStructure structure = contexts.get(i).getStructure();
            for (int w = 0; w < structure.getWindowNodeCount(); w++) {
                ViewNode node = find(structure.getWindowNodeAt(w).getRootViewNode(), id);
                AutofillValue value = node != null ? node.getAutofillValue() : null;
                if (value != null && value.isText()) {
                    return value.getTextValue().toString();
                }
            }
        }
        return "";
    }

    private static ViewNode find(ViewNode node, AutofillId id) {
        if (id.equals(node.getAutofillId())) {
            return node;
        }
        for (int i = 0; i < node.getChildCount(); i++) {
            ViewNode found = find(node.getChildAt(i), id);
            if (found != null) {
                return found;
            }
        }
        return null;
    }

    private void walk(ViewNode node, String domain, String scheme, boolean visible) {
        String nodeDomain = node.getWebDomain();
        if (nodeDomain != null && !nodeDomain.isEmpty()) {
            domain = nodeDomain;
            scheme = node.getWebScheme() != null ? node.getWebScheme() : "";
        }
        visible = visible && node.getVisibility() == View.VISIBLE;
        if (node.getAutofillType() == View.AUTOFILL_TYPE_TEXT && node.getAutofillId() != null) {
            fields.add(describe(node, domain, scheme, visible));
            ids.add(node.getAutofillId());
        }
        for (int i = 0; i < node.getChildCount(); i++) {
            walk(node.getChildAt(i), domain, scheme, visible);
        }
    }

    private Map<String, Object> describe(ViewNode node, String domain, String scheme, boolean visible) {
        Map<String, Object> field = new HashMap<>();
        field.put("index", ids.size());
        String[] hints = node.getAutofillHints();
        field.put("hints", hints != null ? Arrays.asList(hints) : List.of());
        field.put("inputType", node.getInputType());
        field.put("maxLength", node.getMaxTextLength());
        ViewStructure.HtmlInfo html = node.getHtmlInfo();
        if (html != null) {
            Map<String, String> attributes = new HashMap<>();
            List<Pair<String, String>> pairs = html.getAttributes();
            if (pairs != null) {
                for (Pair<String, String> pair : pairs) {
                    if (pair.first != null && pair.second != null) {
                        attributes.put(pair.first.toLowerCase(Locale.ROOT), pair.second);
                    }
                }
            }
            Map<String, Object> info = new HashMap<>();
            info.put("tag", html.getTag());
            info.put("attributes", attributes);
            field.put("html", info);
        }
        field.put("webDomain", domain);
        field.put("webScheme", scheme);
        field.put("idEntry", node.getIdEntry() != null ? node.getIdEntry() : "");
        field.put("hint", node.getHint() != null ? node.getHint() : "");
        field.put("focused", node.isFocused());
        field.put("visible", visible);
        return field;
    }

    /** An app the service cannot see has no signers, and the core answers it nothing. */
    private static List<String> signers(PackageManager packages, String packageName) {
        try {
            return Signers.of(packageInfo(packages, packageName).signingInfo);
        } catch (PackageManager.NameNotFoundException e) {
            return new ArrayList<>();
        }
    }

    @SuppressWarnings("deprecation")
    private static PackageInfo packageInfo(PackageManager packages, String packageName)
            throws PackageManager.NameNotFoundException {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            return packages.getPackageInfo(packageName,
                    PackageManager.PackageInfoFlags.of(PackageManager.GET_SIGNING_CERTIFICATES));
        }
        return packages.getPackageInfo(packageName, PackageManager.GET_SIGNING_CERTIFICATES);
    }
}
