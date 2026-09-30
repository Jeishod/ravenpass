#ifndef RAVENPASS_BRIDGE_JNI_ANDROID_H
#define RAVENPASS_BRIDGE_JNI_ANDROID_H

#include <jni.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

// Returns 0, or -1 with NoSuchMethodError pending for the Java caller where Bridge lacks a method.
int ravenpass_bridge_attach(JNIEnv *env, jclass bridge);

// Returns the thread's JNI environment inside a new local frame, or NULL where the thread cannot attach.
JNIEnv *ravenpass_bridge_enter(bool *attachedHere);
void ravenpass_bridge_leave(JNIEnv *env, bool attachedHere);

// These return -1 where Bridge throws or JNI fails; ravenpass_bridge_release wipes and frees a payload.
int32_t ravenpass_bridge_create_key(JNIEnv *env, const void *alias, jsize aliasLength, bool presence,
    uint8_t **payload, size_t *payloadLength);
int32_t ravenpass_bridge_agree(JNIEnv *env, const void *alias, jsize aliasLength,
    const void *peer, jsize peerLength, uint8_t **payload, size_t *payloadLength);
int32_t ravenpass_bridge_decrypt(JNIEnv *env, const void *alias, jsize aliasLength,
    const void *ciphertext, jsize ciphertextLength, const void *reason, jsize reasonLength,
    uint8_t **payload, size_t *payloadLength);
bool ravenpass_bridge_owner_available(JNIEnv *env);
int32_t ravenpass_bridge_authenticate(JNIEnv *env, const void *reason, jsize reasonLength);
void ravenpass_bridge_cancel_authentication(JNIEnv *env);
int64_t ravenpass_bridge_write_text(JNIEnv *env, const void *text, jsize textLength);
int64_t ravenpass_bridge_change_count(JNIEnv *env);
void ravenpass_bridge_clear_clipboard(JNIEnv *env);
int64_t ravenpass_bridge_write_scan(JNIEnv *env, const void *content, jsize contentLength,
    const void *mediaType, jsize mediaTypeLength);
void ravenpass_bridge_allow_screenshots(JNIEnv *env, bool allowed);
// Wipes the Java copy of the page once Bridge returns.
int32_t ravenpass_bridge_print(JNIEnv *env, const void *job, jsize jobLength, const void *page, jsize pageLength);
int32_t ravenpass_bridge_pick_document(JNIEnv *env, bool create, const void *name, jsize nameLength,
    uint8_t **payload, size_t *payloadLength);
int32_t ravenpass_bridge_create_document(JNIEnv *env, const void *name, jsize nameLength,
    const void *mediaType, jsize mediaTypeLength, uint8_t **payload, size_t *payloadLength);
int32_t ravenpass_bridge_read_document(JNIEnv *env, const void *address, jsize addressLength, int64_t limit,
    uint8_t **payload, size_t *payloadLength);
int32_t ravenpass_bridge_write_document(JNIEnv *env, const void *address, jsize addressLength,
    const void *data, jsize dataLength);
int32_t ravenpass_bridge_delete_document(JNIEnv *env, const void *address, jsize addressLength);
int32_t ravenpass_bridge_pick_folder(JNIEnv *env, uint8_t **payload, size_t *payloadLength);
int32_t ravenpass_bridge_create_in_folder(JNIEnv *env, const void *folder, jsize folderLength,
    const void *name, jsize nameLength, const void *mediaType, jsize mediaTypeLength,
    uint8_t **payload, size_t *payloadLength);
int32_t ravenpass_bridge_pick_photo(JNIEnv *env, uint8_t **payload, size_t *payloadLength);
bool ravenpass_bridge_autofill_selected(JNIEnv *env);
int32_t ravenpass_bridge_passkey_provider(JNIEnv *env);
int32_t ravenpass_bridge_select_autofill(JNIEnv *env);
// Returns NULL where Bridge reports no language tags or throws.
uint8_t *ravenpass_bridge_preferred_languages(JNIEnv *env, size_t *length);
void ravenpass_bridge_set_language(JNIEnv *env, const void *tag, jsize tagLength);
void ravenpass_bridge_set_appearance(JNIEnv *env, const void *appearance, jsize appearanceLength);
// Returns NULL where Bridge throws.
uint8_t *ravenpass_bridge_system_version(JNIEnv *env, size_t *length);
// Returns -1 where Bridge throws or JNI fails.
int32_t ravenpass_bridge_third_party_notices(JNIEnv *env, uint8_t **payload, size_t *payloadLength);
void ravenpass_bridge_release(uint8_t *payload, size_t length);

// Returns NULL for a missing or empty array or a failed copy.
uint8_t *ravenpass_bridge_copy(JNIEnv *env, jbyteArray array, size_t *length);
// Returns NULL with an exception pending where JNI cannot allocate the array.
jbyteArray ravenpass_bridge_array(JNIEnv *env, const void *data, jsize length);

#endif
