//go:build darwin && cgo

#import <Foundation/Foundation.h>

#include <stdlib.h>
#include <string.h>

#include "systemlanguages_darwin.h"

char *ravenpass_preferred_languages(void) {
    @autoreleasepool {
        NSString *tags = [[NSLocale preferredLanguages] componentsJoinedByString:@"\n"];
        return tags.length > 0 ? strdup(tags.UTF8String) : NULL;
    }
}
