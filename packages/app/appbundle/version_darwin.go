//go:build darwin

package appbundle

import "golang.org/x/sys/unix"

func productVersion() string {
	version, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return ""
	}
	return version
}
