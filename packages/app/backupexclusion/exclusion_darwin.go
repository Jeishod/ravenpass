//go:build darwin && cgo

package backupexclusion

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>

static int ravenpass_exclude_from_backup(const char *path) {
    @autoreleasepool {
        NSString *string = [NSString stringWithUTF8String:path];
        if (string == nil) return 0;
        NSURL *url = [NSURL fileURLWithPath:string];
        return [url setResourceValue:@YES forKey:NSURLIsExcludedFromBackupKey error:NULL] ? 1 : 0;
    }
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

var errNotExcluded = errors.New("the file could not be excluded from backups")

func exclude(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	if C.ravenpass_exclude_from_backup(cPath) == 0 {
		return errNotExcluded
	}
	return nil
}
