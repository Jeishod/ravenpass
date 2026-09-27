package localfile

// flushDevice does nothing: fsync on Linux already flushes the drive's cache.
func flushDevice(uintptr) error { return nil }
