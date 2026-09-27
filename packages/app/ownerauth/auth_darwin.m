//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <dispatch/dispatch.h>
#include <stdlib.h>

#include "auth_darwin.h"

static int ravenpass_authentication_result(NSError *error) {
    if (error == nil || ![error.domain isEqualToString:LAErrorDomain]) {
        return RAVENPASS_AUTH_FAILED;
    }
    switch (error.code) {
        case LAErrorUserCancel:
        case LAErrorSystemCancel:
        case LAErrorAppCancel:
        case LAErrorUserFallback:
            return RAVENPASS_AUTH_CANCELED;
        case LAErrorAuthenticationFailed:
            return RAVENPASS_AUTH_FAILED;
        default:
            return RAVENPASS_AUTH_UNAVAILABLE;
    }
}

// Shows no prompt; returns 0 on a Mac without a password or where the policy cannot be evaluated.
int ravenpass_device_owner_available(void) {
    @autoreleasepool {
        LAContext *context = [[LAContext alloc] init];
        BOOL available = [context canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:NULL];
        [context release];
        return available ? 1 : 0;
    }
}

struct ravenpass_owner_prompt {
    LAContext *context;
    dispatch_semaphore_t completion;
    int result;
};

// result is written before the semaphore is signalled and read only after it is waited for.
static void ravenpass_owner_prompt_finish(ravenpass_owner_prompt *prompt, int result) {
    prompt->result = result;
    dispatch_semaphore_signal(prompt->completion);
}

ravenpass_owner_prompt *ravenpass_owner_prompt_start(const char *reason) {
    @autoreleasepool {
        ravenpass_owner_prompt *prompt = calloc(1, sizeof(ravenpass_owner_prompt));
        if (prompt == NULL) {
            return NULL;
        }
        prompt->completion = dispatch_semaphore_create(0);
        if (prompt->completion == NULL) {
            free(prompt);
            return NULL;
        }
        prompt->context = [[LAContext alloc] init];

        NSError *availabilityError = nil;
        if (![prompt->context canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:&availabilityError]) {
            ravenpass_owner_prompt_finish(prompt, availabilityError == nil
                ? RAVENPASS_AUTH_UNAVAILABLE : ravenpass_authentication_result(availabilityError));
            return prompt;
        }
        // LocalAuthentication raises an exception for a nil or empty reason.
        NSString *localizedReason = [NSString stringWithUTF8String:reason];
        if (localizedReason == nil || localizedReason.length == 0) {
            ravenpass_owner_prompt_finish(prompt, RAVENPASS_AUTH_UNAVAILABLE);
            return prompt;
        }
        [prompt->context evaluatePolicy:LAPolicyDeviceOwnerAuthentication
                        localizedReason:localizedReason
                                  reply:^(BOOL success, NSError *error) {
            ravenpass_owner_prompt_finish(prompt, success ? RAVENPASS_AUTH_SUCCESS : ravenpass_authentication_result(error));
        }];
        return prompt;
    }
}

int ravenpass_owner_prompt_wait(ravenpass_owner_prompt *prompt) {
    dispatch_semaphore_wait(prompt->completion, DISPATCH_TIME_FOREVER);
    return prompt->result;
}

// Dismisses a prompt still shown, whose reply then reports LAErrorAppCancel; an ended evaluation is unaffected.
void ravenpass_owner_prompt_cancel(ravenpass_owner_prompt *prompt) {
    [prompt->context invalidate];
}

void ravenpass_owner_prompt_free(ravenpass_owner_prompt *prompt) {
    [prompt->context release];
    dispatch_release(prompt->completion);
    free(prompt);
}
