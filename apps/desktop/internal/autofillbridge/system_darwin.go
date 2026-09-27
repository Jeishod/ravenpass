//go:build darwin && cgo

package autofillbridge

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Foundation -framework Security
#include <stdlib.h>
#include "system_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"sync"
	"unsafe"
)

const (
	appIdentifier       = "com.dortanes.ravenpass"
	extensionIdentifier = "com.dortanes.ravenpass.autofill"
	group               = "group.com.dortanes.ravenpass"
	// listeningNotice is the Darwin notification the extension waits for after launching the app.
	listeningNotice = group + ".listening"
	// maxPathBytes is PATH_MAX on macOS.
	maxPathBytes = 1024
)

// teamIdentifier is the form of an Apple Developer Team ID; nothing else enters a requirement.
var teamIdentifier = regexp.MustCompile(`^[A-Z0-9]{10}$`)

// ownTeam is the team that signed this process, empty for an ad-hoc signature.
var ownTeam = sync.OnceValue(func() string {
	team := make([]byte, 64)
	if C.ravenpass_self_team((*C.char)(unsafe.Pointer(&team[0])), C.size_t(len(team))) == 0 {
		return ""
	}
	return teamOf(C.GoString((*C.char)(unsafe.Pointer(&team[0]))))
})

// Mac admits the app and the extension signed by the team that signed this process.
type Mac struct{}

// Signed reports false for a build without a team: its bridge refuses every peer.
func (Mac) Signed() bool {
	team := ownTeam()
	return team != "" && selfSatisfies(requirement(team, appIdentifier))
}

// Admits fails closed for a build without a team.
func (Mac) Admits(conn net.Conn) bool {
	team := ownTeam()
	return team != "" && peerSatisfies(conn, requirement(team, extensionIdentifier))
}

func (Mac) Container() (string, error) {
	identifier := C.CString(group)
	defer C.free(unsafe.Pointer(identifier))
	path := make([]byte, maxPathBytes)
	if C.ravenpass_group_container(identifier, (*C.char)(unsafe.Pointer(&path[0])), C.size_t(len(path))) == 0 {
		return "", errors.New("the system has no container for the App Group")
	}
	return C.GoString((*C.char)(unsafe.Pointer(&path[0]))), nil
}

func (Mac) AnnounceListening() {
	name := C.CString(listeningNotice)
	defer C.free(unsafe.Pointer(name))
	C.ravenpass_post(name)
}

// teamOf is raw when it has the form of a Team ID, else empty.
func teamOf(raw string) string {
	if !teamIdentifier.MatchString(raw) {
		return ""
	}
	return raw
}

// requirement admits identifier signed by team with an Apple Development or Developer ID certificate.
func requirement(team, identifier string) string {
	return fmt.Sprintf(`anchor apple generic and certificate leaf[subject.OU] = "%s" and identifier "%s"`, team, identifier)
}

func selfSatisfies(requirement string) bool {
	text := C.CString(requirement)
	defer C.free(unsafe.Pointer(text))
	return C.ravenpass_self_satisfies(text) != 0
}

// peerSatisfies reports whether the process at the other end of the Unix socket conn is validly signed to requirement.
func peerSatisfies(conn net.Conn, requirement string) bool {
	socket, ok := conn.(*net.UnixConn)
	if !ok {
		return false
	}
	raw, err := socket.SyscallConn()
	if err != nil {
		return false
	}
	text := C.CString(requirement)
	defer C.free(unsafe.Pointer(text))
	satisfied := false
	if err := raw.Control(func(descriptor uintptr) {
		satisfied = C.ravenpass_peer_satisfies(C.int(descriptor), text) != 0
	}); err != nil {
		return false
	}
	return satisfied
}
