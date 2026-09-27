package com.dortanes.ravenpass.autofill;

import android.content.pm.Signature;
import android.content.pm.SigningInfo;
import android.util.Base64;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;

/** How the core names an app's signers: the SHA-256 digests, in base64, of its certificates. */
final class Signers {
    private Signers() {
    }

    /** The digests of the certificates an app is signed with now; none for null. */
    static List<String> of(SigningInfo signing) {
        List<String> digests = new ArrayList<>();
        if (signing == null) {
            return digests;
        }
        try {
            MessageDigest sha256 = MessageDigest.getInstance("SHA-256");
            for (Signature signer : signing.getApkContentsSigners()) {
                digests.add(Base64.encodeToString(sha256.digest(signer.toByteArray()), Base64.NO_WRAP));
            }
        } catch (NoSuchAlgorithmException e) {
            digests.clear();
        }
        return digests;
    }
}
