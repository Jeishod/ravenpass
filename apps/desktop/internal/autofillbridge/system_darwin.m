//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <Security/Security.h>
#include <notify.h>
#include <sys/socket.h>
#include <sys/un.h>

#include "system_darwin.h"

static int ravenpass_code_satisfies(SecCodeRef code, const char *requirementText) {
    CFStringRef text = CFStringCreateWithCString(NULL, requirementText, kCFStringEncodingUTF8);
    if (text == NULL) {
        return 0;
    }
    SecRequirementRef requirement = NULL;
    OSStatus status = SecRequirementCreateWithString(text, kSecCSDefaultFlags, &requirement);
    CFRelease(text);
    if (status != errSecSuccess) {
        return 0;
    }
    status = SecCodeCheckValidity(code, kSecCSDefaultFlags, requirement);
    CFRelease(requirement);
    return status == errSecSuccess;
}

int ravenpass_self_satisfies(const char *requirement) {
    SecCodeRef code = NULL;
    if (SecCodeCopySelf(kSecCSDefaultFlags, &code) != errSecSuccess) {
        return 0;
    }
    int satisfied = ravenpass_code_satisfies(code, requirement);
    CFRelease(code);
    return satisfied;
}

int ravenpass_self_team(char *team, size_t capacity) {
    SecCodeRef code = NULL;
    if (SecCodeCopySelf(kSecCSDefaultFlags, &code) != errSecSuccess) {
        return 0;
    }
    SecStaticCodeRef staticCode = NULL;
    OSStatus status = SecCodeCopyStaticCode(code, kSecCSDefaultFlags, &staticCode);
    CFRelease(code);
    if (status != errSecSuccess) {
        return 0;
    }
    CFDictionaryRef information = NULL;
    status = SecCodeCopySigningInformation(staticCode, kSecCSSigningInformation, &information);
    CFRelease(staticCode);
    if (status != errSecSuccess) {
        return 0;
    }
    CFTypeRef identifier = CFDictionaryGetValue(information, kSecCodeInfoTeamIdentifier);
    int found = identifier != NULL && CFGetTypeID(identifier) == CFStringGetTypeID() &&
        CFStringGetCString((CFStringRef)identifier, team, (CFIndex)capacity, kCFStringEncodingUTF8);
    CFRelease(information);
    return found;
}

// The audit token names the peer's process for its lifetime; another process can reuse a process identifier.
int ravenpass_peer_satisfies(int socket, const char *requirement) {
    audit_token_t token;
    socklen_t length = sizeof(token);
    if (getsockopt(socket, SOL_LOCAL, LOCAL_PEERTOKEN, &token, &length) != 0 || length != sizeof(token)) {
        return 0;
    }
    CFDataRef tokenData = CFDataCreate(NULL, (const UInt8 *)&token, sizeof(token));
    if (tokenData == NULL) {
        return 0;
    }
    const void *keys[] = {kSecGuestAttributeAudit};
    const void *values[] = {tokenData};
    CFDictionaryRef attributes = CFDictionaryCreate(NULL, keys, values, 1, &kCFTypeDictionaryKeyCallBacks,
        &kCFTypeDictionaryValueCallBacks);
    CFRelease(tokenData);
    if (attributes == NULL) {
        return 0;
    }
    SecCodeRef code = NULL;
    OSStatus status = SecCodeCopyGuestWithAttributes(NULL, attributes, kSecCSDefaultFlags, &code);
    CFRelease(attributes);
    if (status != errSecSuccess) {
        return 0;
    }
    int satisfied = ravenpass_code_satisfies(code, requirement);
    CFRelease(code);
    return satisfied;
}

// Creating the group container directly fails with EPERM on macOS 27; the file manager has the system create it.
int ravenpass_group_container(const char *group, char *path, size_t capacity) {
    @autoreleasepool {
        NSString *identifier = [NSString stringWithUTF8String:group];
        NSURL *container = [[NSFileManager defaultManager] containerURLForSecurityApplicationGroupIdentifier:identifier];
        if (container == nil) {
            return 0;
        }
        return [container getFileSystemRepresentation:path maxLength:capacity];
    }
}

void ravenpass_post(const char *name) {
    notify_post(name);
}
