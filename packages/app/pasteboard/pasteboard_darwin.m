//go:build darwin && cgo

#import <AppKit/AppKit.h>

#include "pasteboard_darwin.h"

// org.nspasteboard.ConcealedType asks clipboard history tools not to record the item.
static NSString *const RavenpassConcealedType = @"org.nspasteboard.ConcealedType";
// org.nspasteboard.TransientType asks clipboard history tools not to keep or restore the item.
static NSString *const RavenpassTransientType = @"org.nspasteboard.TransientType";

static long ravenpass_pasteboard_replace(NSPasteboardItem *item) {
    NSPasteboard *board = [NSPasteboard generalPasteboard];
    [board clearContents];
    if (![board writeObjects:@[item]]) {
        return -1;
    }
    return (long)[board changeCount];
}

long ravenpass_pasteboard_write(const void *bytes, long length, const char *type) {
    @autoreleasepool {
        NSPasteboardItem *item = [[NSPasteboardItem alloc] init];
        NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
        long count = -1;
        if ([item setData:data forType:[NSString stringWithUTF8String:type]] &&
            [item setData:[NSData data] forType:RavenpassConcealedType]) {
            count = ravenpass_pasteboard_replace(item);
        }
        [item release];
        return count;
    }
}

long ravenpass_pasteboard_write_text(const void *bytes, long length) {
    @autoreleasepool {
        NSString *text = [[NSString alloc] initWithBytes:bytes length:(NSUInteger)length encoding:NSUTF8StringEncoding];
        if (text == nil) {
            return -1;
        }
        NSPasteboardItem *item = [[NSPasteboardItem alloc] init];
        long count = -1;
        if ([item setString:text forType:NSPasteboardTypeString] &&
            [item setData:[NSData data] forType:RavenpassConcealedType] &&
            [item setData:[NSData data] forType:RavenpassTransientType]) {
            count = ravenpass_pasteboard_replace(item);
        }
        [item release];
        [text release];
        return count;
    }
}

long ravenpass_pasteboard_change_count(void) {
    @autoreleasepool {
        return (long)[[NSPasteboard generalPasteboard] changeCount];
    }
}

void ravenpass_pasteboard_clear(void) {
    @autoreleasepool {
        [[NSPasteboard generalPasteboard] clearContents];
    }
}
