package authenticator

import (
	"encoding/json"
	"testing"
)

func TestClientDataIsWrittenInChromesKeyOrder(t *testing.T) {
	got := ClientData(Get, []byte{0xfb, 0xff, 0xbf}, "https://login.example.com")
	want := `{"type":"webauthn.get","challenge":"-_-_","origin":"https://login.example.com","crossOrigin":false}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	created := ClientData(Create, []byte("challenge"), "https://example.com")
	if want := `{"type":"webauthn.create","challenge":"Y2hhbGxlbmdl","origin":"https://example.com","crossOrigin":false}`; string(created) != want {
		t.Fatalf("got %s, want %s", created, want)
	}
}

func TestClientDataEscapesTheOrigin(t *testing.T) {
	origin := "https://a\"b\\c<d\n"
	got := ClientData(Create, nil, origin)
	want := `{"type":"webauthn.create","challenge":"","origin":"https://a\"b\\c<d\n","crossOrigin":false}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	var decoded struct{ Origin string }
	if err := json.Unmarshal(got, &decoded); err != nil || decoded.Origin != origin {
		t.Fatalf("the origin did not survive a round trip: %q, %v", decoded.Origin, err)
	}
}
