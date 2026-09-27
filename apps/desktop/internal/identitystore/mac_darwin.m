//go:build darwin && cgo

#import <AuthenticationServices/AuthenticationServices.h>
#import <os/log.h>

#include "mac_darwin.h"

// The store answers from another process.
static const int64_t ravenpass_state_timeout = 5;
static const int64_t ravenpass_change_timeout = 10;

typedef void (^ravenpass_store_completion)(BOOL success, NSError *error);

static os_log_t ravenpass_store_log(void) {
    static os_log_t log;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        log = os_log_create("com.dortanes.ravenpass", "identity-store");
    });
    return log;
}

// ravenpass_await starts a call that runs done once, and reports whether done ran within seconds.
static BOOL ravenpass_await(int64_t seconds, void (^start)(dispatch_block_t done)) {
    dispatch_semaphore_t answered = dispatch_semaphore_create(0);
    start(^{
        dispatch_semaphore_signal(answered);
    });
    return dispatch_semaphore_wait(answered, dispatch_time(DISPATCH_TIME_NOW, seconds * NSEC_PER_SEC)) == 0;
}

int ravenpass_identities_enabled(void) {
    __block BOOL enabled = NO;
    BOOL answered = ravenpass_await(ravenpass_state_timeout, ^(dispatch_block_t done) {
        [[ASCredentialIdentityStore sharedStore] getCredentialIdentityStoreStateWithCompletion:^(ASCredentialIdentityStoreState *state) {
            enabled = state.isEnabled;
            done();
        }];
    });
    if (!answered) {
        os_log_error(ravenpass_store_log(), "state: no answer within %lld seconds", ravenpass_state_timeout);
        return 0;
    }
    return enabled;
}

// ravenpass_store_change reports whether the store took change, named by what in the log.
static int ravenpass_store_change(const char *what, void (^change)(ravenpass_store_completion completion)) {
    __block BOOL taken = NO;
    __block NSError *refusal = nil;
    BOOL answered = ravenpass_await(ravenpass_change_timeout, ^(dispatch_block_t done) {
        change(^(BOOL success, NSError *error) {
            taken = success;
            refusal = error;
            done();
        });
    });
    if (!answered) {
        os_log_error(ravenpass_store_log(), "%{public}s: no answer within %lld seconds", what, ravenpass_change_timeout);
        return 0;
    }
    if (!taken) {
        os_log_error(ravenpass_store_log(), "%{public}s refused: %{public}@", what, refusal);
    }
    return taken;
}

static NSString *ravenpass_text(NSDictionary *listed, NSString *key) {
    id value = listed[key];
    return [value isKindOfClass:[NSString class]] ? value : nil;
}

static NSData *ravenpass_data(NSDictionary *listed, NSString *key) {
    NSString *encoded = ravenpass_text(listed, key);
    return encoded == nil ? nil : [[NSData alloc] initWithBase64EncodedString:encoded options:0];
}

// ravenpass_identity is nil for an entry that describes no identity.
API_AVAILABLE(macos(15.0))
static id<ASCredentialIdentity> ravenpass_identity(NSDictionary *listed) {
    NSString *kind = ravenpass_text(listed, @"kind");
    NSString *site = ravenpass_text(listed, @"site");
    NSString *user = ravenpass_text(listed, @"user");
    NSString *record = ravenpass_text(listed, @"record");
    if (kind == nil || site.length == 0 || user == nil || record.length == 0) {
        return nil;
    }
    ASCredentialServiceIdentifier *service = [[ASCredentialServiceIdentifier alloc]
        initWithIdentifier:site type:ASCredentialServiceIdentifierTypeDomain];
    if ([kind isEqualToString:@"password"]) {
        return [[ASPasswordCredentialIdentity alloc] initWithServiceIdentifier:service user:user recordIdentifier:record];
    }
    if ([kind isEqualToString:@"code"]) {
        return [[ASOneTimeCodeCredentialIdentity alloc] initWithServiceIdentifier:service label:user recordIdentifier:record];
    }
    if ([kind isEqualToString:@"passkey"]) {
        NSData *credentialID = ravenpass_data(listed, @"credentialID");
        NSData *userHandle = ravenpass_data(listed, @"userHandle");
        if (credentialID.length == 0 || userHandle.length == 0) {
            return nil;
        }
        return [[ASPasskeyCredentialIdentity alloc] initWithRelyingPartyIdentifier:site userName:user
            credentialID:credentialID userHandle:userHandle recordIdentifier:record];
    }
    return nil;
}

int ravenpass_identities_replace(const char *listed, size_t length) {
    // The store belongs to the AutoFill extension, which needs macOS 15.
    if (@available(macOS 15.0, *)) {
        @autoreleasepool {
            NSData *text = [NSData dataWithBytesNoCopy:(void *)listed length:length freeWhenDone:NO];
            id parsed = [NSJSONSerialization JSONObjectWithData:text options:0 error:NULL];
            if (![parsed isKindOfClass:[NSArray class]]) {
                os_log_error(ravenpass_store_log(), "replace: the list is not an array");
                return 0;
            }
            NSMutableArray<id<ASCredentialIdentity>> *identities = [NSMutableArray arrayWithCapacity:[parsed count]];
            for (id entry in parsed) {
                id<ASCredentialIdentity> identity = [entry isKindOfClass:[NSDictionary class]] ? ravenpass_identity(entry) : nil;
                if (identity == nil) {
                    os_log_error(ravenpass_store_log(), "replace: entry %lu describes no identity", (unsigned long)identities.count);
                    return 0;
                }
                [identities addObject:identity];
            }
            return ravenpass_store_change("replace", ^(ravenpass_store_completion completion) {
                [[ASCredentialIdentityStore sharedStore] replaceCredentialIdentityEntries:identities completion:completion];
            });
        }
    }
    return 0;
}

int ravenpass_identities_remove_all(void) {
    @autoreleasepool {
        return ravenpass_store_change("remove all", ^(ravenpass_store_completion completion) {
            [[ASCredentialIdentityStore sharedStore] removeAllCredentialIdentitiesWithCompletion:completion];
        });
    }
}
