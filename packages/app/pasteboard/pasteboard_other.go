//go:build !darwin || !cgo

package pasteboard

func write([]byte, string) (int64, error) { return 0, ErrUnavailable }

func writeText(string) (int64, error) { return 0, ErrUnavailable }

func changeCount() int64 { return 0 }

func clearContents() {}
