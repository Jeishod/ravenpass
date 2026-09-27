package localfile

import "golang.org/x/sys/unix"

// flushDevice issues F_FULLFSYNC, which macOS fsync omits; exFAT and network volumes reject it, keeping only fsync.
func flushDevice(fd uintptr) error {
	if _, err := unix.FcntlInt(fd, unix.F_FULLFSYNC, 0); err != nil && !unsupportedOperation(err) {
		return err
	}
	return nil
}
