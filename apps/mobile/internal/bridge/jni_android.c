#include "jni_android.h"

#include <stdlib.h>
#include <string.h>

// decrypt makes the most local references: three arguments and its result.
enum { LOCAL_REFERENCES = 4 };

// FindClass on a thread Go attached cannot see app classes; the class Bridge passed at attach stays referenced.
static JavaVM *vm;
static jclass bridgeClass;
static jmethodID createKeyMethod;
static jmethodID agreeMethod;
static jmethodID decryptMethod;
static jmethodID ownerAvailableMethod;
static jmethodID authenticateMethod;
static jmethodID cancelAuthenticationMethod;
static jmethodID writeTextMethod;
static jmethodID changeCountMethod;
static jmethodID clearClipboardMethod;
static jmethodID writeScanMethod;
static jmethodID allowScreenshotsMethod;
static jmethodID printMethod;
static jmethodID pickDocumentMethod;
static jmethodID createDocumentMethod;
static jmethodID readDocumentMethod;
static jmethodID writeDocumentMethod;
static jmethodID deleteDocumentMethod;
static jmethodID pickFolderMethod;
static jmethodID createInFolderMethod;
static jmethodID pickPhotoMethod;
static jmethodID autofillSelectedMethod;
static jmethodID passkeyProviderMethod;
static jmethodID selectAutofillMethod;
static jmethodID preferredLanguagesMethod;
static jmethodID setLanguageMethod;
static jmethodID systemVersionMethod;
static jmethodID thirdPartyNoticesMethod;

// Clears a pending Java exception and reports whether there was one.
static bool threw(JNIEnv *env) {
    if (!(*env)->ExceptionCheck(env)) {
        return false;
    }
    (*env)->ExceptionClear(env);
    return true;
}

static jmethodID staticMethod(JNIEnv *env, const char *name, const char *signature) {
    if ((*env)->ExceptionCheck(env)) {
        return NULL;
    }
    return (*env)->GetStaticMethodID(env, bridgeClass, name, signature);
}

int ravenpass_bridge_attach(JNIEnv *env, jclass bridge) {
    if ((*env)->GetJavaVM(env, &vm) != JNI_OK) {
        return -1;
    }
    bridgeClass = (*env)->NewGlobalRef(env, bridge);
    if (bridgeClass == NULL) {
        return -1;
    }
    createKeyMethod = staticMethod(env, "createKey", "([BZ)[B");
    agreeMethod = staticMethod(env, "agree", "([B[B)[B");
    decryptMethod = staticMethod(env, "decrypt", "([B[B[B)[B");
    ownerAvailableMethod = staticMethod(env, "ownerAvailable", "()Z");
    authenticateMethod = staticMethod(env, "authenticate", "([B)I");
    cancelAuthenticationMethod = staticMethod(env, "cancelAuthentication", "()V");
    writeTextMethod = staticMethod(env, "writeText", "([B)J");
    changeCountMethod = staticMethod(env, "changeCount", "()J");
    clearClipboardMethod = staticMethod(env, "clearClipboard", "()V");
    writeScanMethod = staticMethod(env, "writeScan", "([B[B)J");
    allowScreenshotsMethod = staticMethod(env, "allowScreenshots", "(Z)V");
    printMethod = staticMethod(env, "print", "([B[B)I");
    pickDocumentMethod = staticMethod(env, "pickDocument", "(Z[B)[B");
    createDocumentMethod = staticMethod(env, "createDocument", "([B[B)[B");
    readDocumentMethod = staticMethod(env, "readDocument", "([BJ)[B");
    writeDocumentMethod = staticMethod(env, "writeDocument", "([B[B)I");
    deleteDocumentMethod = staticMethod(env, "deleteDocument", "([B)I");
    pickFolderMethod = staticMethod(env, "pickFolder", "()[B");
    createInFolderMethod = staticMethod(env, "createInFolder", "([B[B[B)[B");
    pickPhotoMethod = staticMethod(env, "pickPhoto", "()[B");
    autofillSelectedMethod = staticMethod(env, "autofillSelected", "()Z");
    passkeyProviderMethod = staticMethod(env, "passkeyProvider", "()I");
    selectAutofillMethod = staticMethod(env, "selectAutofill", "()I");
    preferredLanguagesMethod = staticMethod(env, "preferredLanguages", "()[B");
    setLanguageMethod = staticMethod(env, "setLanguage", "([B)V");
    systemVersionMethod = staticMethod(env, "systemVersion", "()[B");
    thirdPartyNoticesMethod = staticMethod(env, "thirdPartyNotices", "()[B");
    return (*env)->ExceptionCheck(env) ? -1 : 0;
}

JNIEnv *ravenpass_bridge_enter(bool *attachedHere) {
    JNIEnv *env = NULL;
    *attachedHere = false;
    jint state = (*vm)->GetEnv(vm, (void **)&env, JNI_VERSION_1_6);
    if (state == JNI_EDETACHED) {
        if ((*vm)->AttachCurrentThread(vm, &env, NULL) != JNI_OK) {
            return NULL;
        }
        *attachedHere = true;
    } else if (state != JNI_OK) {
        return NULL;
    }
    if ((*env)->PushLocalFrame(env, LOCAL_REFERENCES) != JNI_OK) {
        (*env)->ExceptionClear(env);
        if (*attachedHere) {
            (*vm)->DetachCurrentThread(vm);
        }
        return NULL;
    }
    return env;
}

void ravenpass_bridge_leave(JNIEnv *env, bool attachedHere) {
    (*env)->PopLocalFrame(env, NULL);
    if (attachedHere) {
        (*vm)->DetachCurrentThread(vm);
    }
}

// A pending exception makes this return NULL and the following threw() skip the Java call.
static jbyteArray newArray(JNIEnv *env, const void *data, jsize length) {
    if ((*env)->ExceptionCheck(env)) {
        return NULL;
    }
    jbyteArray array = (*env)->NewByteArray(env, length);
    if (array != NULL && length > 0) {
        (*env)->SetByteArrayRegion(env, array, 0, length, (const jbyte *)data);
    }
    return array;
}

static void wipe(JNIEnv *env, jbyteArray array) {
    jsize length = (*env)->GetArrayLength(env, array);
    jbyte *elements = (*env)->GetPrimitiveArrayCritical(env, array, NULL);
    if (elements == NULL) {
        threw(env);
        return;
    }
    memset(elements, 0, (size_t)length);
    (*env)->ReleasePrimitiveArrayCritical(env, array, elements, 0);
}

// Returns the status byte that heads result, copies the rest into *payload, and wipes result.
static int32_t takeResult(JNIEnv *env, jbyteArray result, uint8_t **payload, size_t *payloadLength) {
    *payload = NULL;
    *payloadLength = 0;
    if (threw(env) || result == NULL) {
        return -1;
    }
    jsize length = (*env)->GetArrayLength(env, result);
    if (length < 1) {
        return -1;
    }
    jbyte status = 0;
    (*env)->GetByteArrayRegion(env, result, 0, 1, &status);
    int32_t outcome = status;
    if (length > 1) {
        uint8_t *copy = malloc((size_t)length - 1);
        if (copy == NULL) {
            outcome = -1;
        } else {
            (*env)->GetByteArrayRegion(env, result, 1, length - 1, (jbyte *)copy);
            *payload = copy;
            *payloadLength = (size_t)length - 1;
        }
    }
    wipe(env, result);
    return outcome;
}

int32_t ravenpass_bridge_create_key(JNIEnv *env, const void *alias, jsize aliasLength, bool presence,
    uint8_t **payload, size_t *payloadLength) {
    jbyteArray aliasBytes = newArray(env, alias, aliasLength);
    if (threw(env)) {
        return -1;
    }
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, createKeyMethod, aliasBytes,
        (jboolean)presence);
    return takeResult(env, result, payload, payloadLength);
}

int32_t ravenpass_bridge_agree(JNIEnv *env, const void *alias, jsize aliasLength,
    const void *peer, jsize peerLength, uint8_t **payload, size_t *payloadLength) {
    jbyteArray aliasBytes = newArray(env, alias, aliasLength);
    jbyteArray peerBytes = newArray(env, peer, peerLength);
    if (threw(env)) {
        return -1;
    }
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, agreeMethod, aliasBytes, peerBytes);
    return takeResult(env, result, payload, payloadLength);
}

int32_t ravenpass_bridge_decrypt(JNIEnv *env, const void *alias, jsize aliasLength,
    const void *ciphertext, jsize ciphertextLength, const void *reason, jsize reasonLength,
    uint8_t **payload, size_t *payloadLength) {
    jbyteArray aliasBytes = newArray(env, alias, aliasLength);
    jbyteArray ciphertextBytes = newArray(env, ciphertext, ciphertextLength);
    jbyteArray reasonBytes = newArray(env, reason, reasonLength);
    if (threw(env)) {
        return -1;
    }
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, decryptMethod, aliasBytes,
        ciphertextBytes, reasonBytes);
    return takeResult(env, result, payload, payloadLength);
}

bool ravenpass_bridge_owner_available(JNIEnv *env) {
    jboolean available = (*env)->CallStaticBooleanMethod(env, bridgeClass, ownerAvailableMethod);
    return !threw(env) && available == JNI_TRUE;
}

int32_t ravenpass_bridge_authenticate(JNIEnv *env, const void *reason, jsize reasonLength) {
    jbyteArray reasonBytes = newArray(env, reason, reasonLength);
    if (threw(env)) {
        return -1;
    }
    jint status = (*env)->CallStaticIntMethod(env, bridgeClass, authenticateMethod, reasonBytes);
    return threw(env) ? -1 : status;
}

void ravenpass_bridge_cancel_authentication(JNIEnv *env) {
    (*env)->CallStaticVoidMethod(env, bridgeClass, cancelAuthenticationMethod);
    threw(env);
}

int64_t ravenpass_bridge_write_text(JNIEnv *env, const void *text, jsize textLength) {
    jbyteArray textBytes = newArray(env, text, textLength);
    if (threw(env)) {
        return -1;
    }
    jlong stamp = (*env)->CallStaticLongMethod(env, bridgeClass, writeTextMethod, textBytes);
    bool failed = threw(env);
    wipe(env, textBytes);
    return failed ? -1 : stamp;
}

int64_t ravenpass_bridge_change_count(JNIEnv *env) {
    jlong stamp = (*env)->CallStaticLongMethod(env, bridgeClass, changeCountMethod);
    return threw(env) ? -1 : stamp;
}

void ravenpass_bridge_clear_clipboard(JNIEnv *env) {
    (*env)->CallStaticVoidMethod(env, bridgeClass, clearClipboardMethod);
    threw(env);
}

int64_t ravenpass_bridge_write_scan(JNIEnv *env, const void *content, jsize contentLength,
    const void *mediaType, jsize mediaTypeLength) {
    jbyteArray contentBytes = newArray(env, content, contentLength);
    jbyteArray typeBytes = newArray(env, mediaType, mediaTypeLength);
    if (threw(env)) {
        return -1;
    }
    jlong stamp = (*env)->CallStaticLongMethod(env, bridgeClass, writeScanMethod, contentBytes, typeBytes);
    bool failed = threw(env);
    wipe(env, contentBytes);
    return failed ? -1 : stamp;
}

void ravenpass_bridge_allow_screenshots(JNIEnv *env, bool allowed) {
    (*env)->CallStaticVoidMethod(env, bridgeClass, allowScreenshotsMethod, (jboolean)allowed);
    threw(env);
}

int32_t ravenpass_bridge_print(JNIEnv *env, const void *job, jsize jobLength, const void *page, jsize pageLength) {
    jbyteArray jobBytes = newArray(env, job, jobLength);
    jbyteArray pageBytes = newArray(env, page, pageLength);
    if (threw(env)) {
        return -1;
    }
    jint status = (*env)->CallStaticIntMethod(env, bridgeClass, printMethod, jobBytes, pageBytes);
    bool failed = threw(env);
    wipe(env, pageBytes);
    return failed ? -1 : status;
}

int32_t ravenpass_bridge_pick_document(JNIEnv *env, bool create, const void *name, jsize nameLength,
    uint8_t **payload, size_t *payloadLength) {
    jbyteArray nameBytes = newArray(env, name, nameLength);
    if (threw(env)) {
        return -1;
    }
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, pickDocumentMethod, (jboolean)create,
        nameBytes);
    return takeResult(env, result, payload, payloadLength);
}

int32_t ravenpass_bridge_create_document(JNIEnv *env, const void *name, jsize nameLength,
    const void *mediaType, jsize mediaTypeLength, uint8_t **payload, size_t *payloadLength) {
    jbyteArray nameBytes = newArray(env, name, nameLength);
    jbyteArray typeBytes = newArray(env, mediaType, mediaTypeLength);
    if (threw(env)) {
        return -1;
    }
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, createDocumentMethod, nameBytes,
        typeBytes);
    return takeResult(env, result, payload, payloadLength);
}

int32_t ravenpass_bridge_read_document(JNIEnv *env, const void *address, jsize addressLength, int64_t limit,
    uint8_t **payload, size_t *payloadLength) {
    jbyteArray addressBytes = newArray(env, address, addressLength);
    if (threw(env)) {
        return -1;
    }
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, readDocumentMethod, addressBytes,
        (jlong)limit);
    return takeResult(env, result, payload, payloadLength);
}

int32_t ravenpass_bridge_write_document(JNIEnv *env, const void *address, jsize addressLength,
    const void *data, jsize dataLength) {
    jbyteArray addressBytes = newArray(env, address, addressLength);
    jbyteArray dataBytes = newArray(env, data, dataLength);
    if (threw(env)) {
        return -1;
    }
    jint status = (*env)->CallStaticIntMethod(env, bridgeClass, writeDocumentMethod, addressBytes, dataBytes);
    bool failed = threw(env);
    wipe(env, dataBytes);
    return failed ? -1 : status;
}

int32_t ravenpass_bridge_delete_document(JNIEnv *env, const void *address, jsize addressLength) {
    jbyteArray addressBytes = newArray(env, address, addressLength);
    if (threw(env)) {
        return -1;
    }
    jint status = (*env)->CallStaticIntMethod(env, bridgeClass, deleteDocumentMethod, addressBytes);
    return threw(env) ? -1 : status;
}

int32_t ravenpass_bridge_pick_folder(JNIEnv *env, uint8_t **payload, size_t *payloadLength) {
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, pickFolderMethod);
    return takeResult(env, result, payload, payloadLength);
}

int32_t ravenpass_bridge_create_in_folder(JNIEnv *env, const void *folder, jsize folderLength,
    const void *name, jsize nameLength, const void *mediaType, jsize mediaTypeLength,
    uint8_t **payload, size_t *payloadLength) {
    jbyteArray folderBytes = newArray(env, folder, folderLength);
    jbyteArray nameBytes = newArray(env, name, nameLength);
    jbyteArray typeBytes = newArray(env, mediaType, mediaTypeLength);
    if (threw(env)) {
        return -1;
    }
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, createInFolderMethod, folderBytes,
        nameBytes, typeBytes);
    return takeResult(env, result, payload, payloadLength);
}

int32_t ravenpass_bridge_pick_photo(JNIEnv *env, uint8_t **payload, size_t *payloadLength) {
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, pickPhotoMethod);
    return takeResult(env, result, payload, payloadLength);
}

bool ravenpass_bridge_autofill_selected(JNIEnv *env) {
    jboolean selected = (*env)->CallStaticBooleanMethod(env, bridgeClass, autofillSelectedMethod);
    return !threw(env) && selected == JNI_TRUE;
}

int32_t ravenpass_bridge_passkey_provider(JNIEnv *env) {
    jint state = (*env)->CallStaticIntMethod(env, bridgeClass, passkeyProviderMethod);
    return threw(env) ? -1 : state;
}

int32_t ravenpass_bridge_select_autofill(JNIEnv *env) {
    jint status = (*env)->CallStaticIntMethod(env, bridgeClass, selectAutofillMethod);
    return threw(env) ? -1 : status;
}

uint8_t *ravenpass_bridge_preferred_languages(JNIEnv *env, size_t *length) {
    *length = 0;
    jbyteArray tags = (*env)->CallStaticObjectMethod(env, bridgeClass, preferredLanguagesMethod);
    if (threw(env)) {
        return NULL;
    }
    return ravenpass_bridge_copy(env, tags, length);
}

void ravenpass_bridge_set_language(JNIEnv *env, const void *tag, jsize tagLength) {
    jbyteArray tagBytes = newArray(env, tag, tagLength);
    if (threw(env)) {
        return;
    }
    (*env)->CallStaticVoidMethod(env, bridgeClass, setLanguageMethod, tagBytes);
    threw(env);
}

uint8_t *ravenpass_bridge_system_version(JNIEnv *env, size_t *length) {
    *length = 0;
    jbyteArray version = (*env)->CallStaticObjectMethod(env, bridgeClass, systemVersionMethod);
    if (threw(env)) {
        return NULL;
    }
    return ravenpass_bridge_copy(env, version, length);
}

int32_t ravenpass_bridge_third_party_notices(JNIEnv *env, uint8_t **payload, size_t *payloadLength) {
    jbyteArray result = (*env)->CallStaticObjectMethod(env, bridgeClass, thirdPartyNoticesMethod);
    return takeResult(env, result, payload, payloadLength);
}

uint8_t *ravenpass_bridge_copy(JNIEnv *env, jbyteArray array, size_t *length) {
    *length = 0;
    if (array == NULL) {
        return NULL;
    }
    jsize size = (*env)->GetArrayLength(env, array);
    if (size < 1) {
        return NULL;
    }
    uint8_t *copy = malloc((size_t)size);
    if (copy == NULL) {
        return NULL;
    }
    (*env)->GetByteArrayRegion(env, array, 0, size, (jbyte *)copy);
    if (threw(env)) {
        ravenpass_bridge_release(copy, (size_t)size);
        return NULL;
    }
    *length = (size_t)size;
    return copy;
}

jbyteArray ravenpass_bridge_array(JNIEnv *env, const void *data, jsize length) {
    return newArray(env, data, length);
}

// memset_explicit needs API 34; the volatile store keeps the compiler from eliding the wipe.
void ravenpass_bridge_release(uint8_t *payload, size_t length) {
    volatile uint8_t *bytes = payload;
    for (size_t i = 0; i < length; i++) {
        bytes[i] = 0;
    }
    free(payload);
}
