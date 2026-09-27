//go:build darwin && cgo

#import <AppKit/AppKit.h>

#include "dock_darwin.h"

static void ravenpass_on_main(dispatch_block_t block) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

void ravenpass_dock_show(void) {
    ravenpass_on_main(^{
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSApp activateIgnoringOtherApps:YES];
    });
}

void ravenpass_dock_hide(void) {
    ravenpass_on_main(^{
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    });
}
