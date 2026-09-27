//go:build darwin && cgo

#import <Foundation/Foundation.h>

#include "trash_darwin.h"

int ravenpass_trash_move(const char *path) {
    @autoreleasepool {
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        NSError *error = nil;
        BOOL moved = [[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:nil error:&error];
        return moved ? 1 : 0;
    }
}
