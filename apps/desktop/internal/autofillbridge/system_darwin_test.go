//go:build darwin && cgo

package autofillbridge

import (
	"net"
	"path/filepath"
	"testing"
)

// connected returns the accepted end of a Unix socket whose peer is this test binary, which the linker signs ad hoc.
func connected(t *testing.T) net.Conn {
	t.Helper()
	listener, err := net.Listen("unix", filepath.Join(container(t), socketName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	dialed, err := net.Dial("unix", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dialed.Close() })
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = accepted.Close() })
	return accepted
}

func TestABuildSignedAdHocHasNoTeam(t *testing.T) {
	if !selfSatisfies("always") {
		t.Fatal("this process's code could not be checked at all")
	}
	if team := ownTeam(); team != "" {
		t.Fatalf("an ad-hoc signature names team %q", team)
	}
}

func TestABuildWithoutATeamAdmitsNoPeer(t *testing.T) {
	conn := connected(t)
	if !peerSatisfies(conn, "always") {
		t.Fatal("the peer's code could not be checked at all")
	}
	if (Mac{}).Admits(conn) {
		t.Fatal("a build without a team admitted a peer")
	}
	if (Mac{}).Signed() {
		t.Fatal("a build without a team counts as signed for the extension")
	}
}

func TestOnlyATeamIDEntersARequirement(t *testing.T) {
	for raw, want := range map[string]string{
		"AB12CD34EF":             "AB12CD34EF",
		"":                       "",
		"ab12cd34ef":             "",
		"AB12CD34E":              "",
		`AB12CD34EF" or "always`: "",
	} {
		if got := teamOf(raw); got != want {
			t.Errorf("teamOf(%q) = %q, want %q", raw, got, want)
		}
	}
	want := `anchor apple generic and certificate leaf[subject.OU] = "AB12CD34EF" and identifier "com.dortanes.ravenpass.autofill"`
	if got := requirement("AB12CD34EF", extensionIdentifier); got != want {
		t.Fatalf("requirement = %s", got)
	}
}
