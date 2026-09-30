//go:build darwin && cgo

#import <AppKit/AppKit.h>

#include "appearance_darwin.h"

// Windows without an appearance of their own inherit the application's; WKWebView reports it as prefers-color-scheme.
void ravenpass_appearance_set(ravenpass_appearance appearance) {
    dispatch_block_t set = ^{
        NSAppearance *chosen = nil;
        if (appearance == ravenpass_appearance_light) {
            chosen = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
        } else if (appearance == ravenpass_appearance_dark) {
            chosen = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        }
        [NSApplication sharedApplication].appearance = chosen;
    };
    if ([NSThread isMainThread]) {
        set();
    } else {
        dispatch_async(dispatch_get_main_queue(), set);
    }
}
