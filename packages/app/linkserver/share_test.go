package linkserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/dortanes/ravenpass/packages/app/linkproto"
)

var (
	exampleFile   = []byte("%PDF-1.4\n%%EOF\n")
	exampleShared = linkproto.SharedFile{Name: "passport.pdf", MediaType: "application/pdf", Size: len(exampleFile)}
	shareRequest  = `{"id":7,"type":"share","identity":"` + exampleID + `","file":"photo","origin":"https://example.com"}`
	shareCall     = "share " + exampleID + " photo https://example.com"
)

// frame is any message the desktop app answers a request with.
type frame struct {
	ID       int64           `json:"id"`
	Result   json.RawMessage `json:"result"`
	Error    string          `json:"error"`
	Parts    int             `json:"parts"`
	Progress string          `json:"progress"`
}

func nextFrame(t *testing.T, conn *websocket.Conn, transport *peerTransport) frame {
	t.Helper()
	var next frame
	if err := json.Unmarshal(nextMessage(t, conn, transport), &next); err != nil {
		t.Fatal(err)
	}
	return next
}

// body reads the parts a head announced and joins them.
func body(t *testing.T, conn *websocket.Conn, transport *peerTransport, head frame) []byte {
	t.Helper()
	var joined []byte
	for range head.Parts {
		part := nextMessage(t, conn, transport)
		if len(part) > linkproto.MaxPlaintextBytes {
			t.Fatalf("a part carries %d bytes", len(part))
		}
		joined = append(joined, part...)
	}
	return joined
}

// sharedFile reads a share's head and parts and checks that they describe and carry content.
func sharedFile(t *testing.T, conn *websocket.Conn, transport *peerTransport, head frame, file linkproto.SharedFile, content []byte, parts int) {
	t.Helper()
	var described linkproto.SharedFile
	if err := json.Unmarshal(head.Result, &described); err != nil {
		t.Fatal(err)
	}
	if head.ID != 7 || described != file || head.Parts != parts {
		t.Fatalf("share head = %+v describing %+v, want %d parts of %+v", head, described, parts, file)
	}
	if !bytes.Equal(body(t, conn, transport, head), content) {
		t.Fatal("the parts do not carry the file")
	}
}

func TestIdentitiesListsEachIdentitysFiles(t *testing.T) {
	extension, vault, port := linked(t)
	conn, transport := extension.openSession(t, port)
	if reply := request(t, conn, transport, `{"id":1,"type":"identities","origin":"https://example.com"}`); reply != `{"id":1,"result":{"identities":[]}}` {
		t.Fatalf("no identities = %s", reply)
	}
	vault.identities = []linkproto.Identity{{ID: exampleID, Label: "Alex", Thumbnail: "/9j/", Files: []linkproto.IdentityFile{
		{ID: "photo", Kind: linkproto.FilePhoto, Name: "photo.jpg", MediaType: "image/jpeg", Thumbnail: "/9j/"},
		{ID: "0a", Kind: linkproto.FileScan, Name: "passport.pdf", MediaType: "application/pdf", Document: &linkproto.Document{Type: "passport"}},
	}}}
	want := `{"id":2,"result":{"identities":[{"id":"` + exampleID + `","label":"Alex","thumbnail":"/9j/","files":[` +
		`{"id":"photo","kind":"photo","name":"photo.jpg","mediaType":"image/jpeg","thumbnail":"/9j/"},` +
		`{"id":"0a","kind":"scan","name":"passport.pdf","mediaType":"application/pdf","document":{"type":"passport","label":""},"thumbnail":""}]}]}}`
	if reply := request(t, conn, transport, `{"id":2,"type":"identities","origin":"https://example.com"}`); reply != want {
		t.Fatalf("identities = %s", reply)
	}
	if calls := vault.takeCalls(); !slices.Equal(calls, []string{"identities", "identities"}) {
		t.Fatalf("the vault was asked %q", calls)
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

func TestALargeIdentityListFollowsAHeadInParts(t *testing.T) {
	extension, vault, port := linked(t)
	thumbnail := strings.Repeat("A", 16_000)
	for i := range 12 {
		vault.identities = append(vault.identities, linkproto.Identity{ID: exampleID, Label: "Identity " + string(rune('A'+i)), Thumbnail: thumbnail, Files: []linkproto.IdentityFile{}})
	}
	conn, transport := extension.openSession(t, port)
	sendRequest(t, conn, transport, `{"id":3,"type":"identities","origin":"https://example.com"}`)
	head := nextFrame(t, conn, transport)
	if head.ID != 3 || head.Parts != 3 || head.Result != nil || head.Error != "" {
		t.Fatalf("head = %+v", head)
	}
	want, err := json.Marshal(linkproto.Identities{Identities: vault.identities})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body(t, conn, transport, head), want) {
		t.Fatal("the parts do not join into the result")
	}
	if reply := request(t, conn, transport, `{"id":4,"type":"status"}`); reply != `{"id":4,"result":{"vault":"locked"}}` {
		t.Fatalf("a request after the parts = %s", reply)
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

func TestAShareSendsTheFileAfterAHead(t *testing.T) {
	extension, vault, port := linked(t)
	conn, transport := extension.openSession(t, port)
	sendRequest(t, conn, transport, shareRequest)
	sharedFile(t, conn, transport, nextFrame(t, conn, transport), exampleShared, exampleFile, 1)

	large := bytes.Repeat([]byte{0xd8}, 3*linkproto.MaxPlaintextBytes+10)
	largeShared := linkproto.SharedFile{Name: "front.jpg", MediaType: "image/jpeg", Size: len(large)}
	vault.shareWith(func(context.Context, func(linkproto.Progress)) (linkproto.SharedFile, []byte, error) {
		return largeShared, bytes.Clone(large), nil
	})
	sendRequest(t, conn, transport, shareRequest)
	sharedFile(t, conn, transport, nextFrame(t, conn, transport), largeShared, large, 4)
	if calls := vault.takeCalls(); !slices.Equal(calls, []string{shareCall, shareCall}) {
		t.Fatalf("the vault was asked %q", calls)
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

// verified is a share that reports progress, waits for wait, then ends with a file or err.
func verified(progress linkproto.Progress, wait time.Duration, err error) func(context.Context, func(linkproto.Progress)) (linkproto.SharedFile, []byte, error) {
	return func(ctx context.Context, asked func(linkproto.Progress)) (linkproto.SharedFile, []byte, error) {
		asked(progress)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return linkproto.SharedFile{}, nil, ctx.Err()
		}
		if err != nil {
			return linkproto.SharedFile{}, nil, err
		}
		return exampleShared, bytes.Clone(exampleFile), nil
	}
}

// progressThen reads a share's progress messages and returns them with the frame that ends them.
func progressThen(t *testing.T, conn *websocket.Conn, transport *peerTransport) ([]string, frame) {
	t.Helper()
	var progress []string
	for {
		next := nextFrame(t, conn, transport)
		if next.Progress == "" {
			return progress, next
		}
		if next.ID != 7 || next.Result != nil || next.Error != "" || next.Parts != 0 {
			t.Fatalf("progress message = %+v", next)
		}
		progress = append(progress, next.Progress)
	}
}

func TestAShareReportsProgressWhileThePersonVerifies(t *testing.T) {
	server, vault := newServer(t, recordsPath(t), func(s *Server) { s.progressInterval = 40 * time.Millisecond })
	key := begin(t, server)
	extension := newExtension(t)
	extension.linkWith(t, key)
	conn, transport := extension.openSession(t, int(key.Port))

	vault.shareWith(verified(linkproto.ProgressConfirmOnDevice, 150*time.Millisecond, nil))
	sendRequest(t, conn, transport, shareRequest)
	progress, head := progressThen(t, conn, transport)
	if len(progress) < 2 || slices.ContainsFunc(progress, func(p string) bool { return p != "confirm-on-device" }) {
		t.Fatalf("progress = %q", progress)
	}
	sharedFile(t, conn, transport, head, exampleShared, exampleFile, 1)

	vault.shareWith(verified(linkproto.ProgressConfirmInRavenpass, 10*time.Millisecond, ErrDeclined))
	sendRequest(t, conn, transport, shareRequest)
	progress, refusal := progressThen(t, conn, transport)
	if !slices.Equal(progress, []string{"confirm-in-ravenpass"}) || refusal.ID != 7 || refusal.Error != "declined" || refusal.Result != nil || refusal.Parts != 0 {
		t.Fatalf("progress %q, then %+v", progress, refusal)
	}
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

func TestTheIdleLimitDoesNotRunWhileAShareWaits(t *testing.T) {
	server, vault := newServer(t, recordsPath(t), func(s *Server) {
		s.idleTimeout = 200 * time.Millisecond
		s.progressInterval = time.Minute
	})
	key := begin(t, server)
	extension := newExtension(t)
	extension.linkWith(t, key)
	conn, transport := extension.openSession(t, int(key.Port))
	vault.shareWith(verified(linkproto.ProgressConfirmInRavenpass, 600*time.Millisecond, nil))
	sendRequest(t, conn, transport, shareRequest)
	progress, head := progressThen(t, conn, transport)
	if !slices.Equal(progress, []string{"confirm-in-ravenpass"}) {
		t.Fatalf("progress = %q", progress)
	}
	sharedFile(t, conn, transport, head, exampleShared, exampleFile, 1)
	started := time.Now()
	wantStatus(t, closeStatus(t, conn), linkproto.CloseIdle)
	if silence := time.Since(started); silence > 2*time.Second {
		t.Fatalf("the idle limit resumed only after %v", silence)
	}
}

func TestAnEndedSessionEndsItsShare(t *testing.T) {
	server, vault := newServer(t, recordsPath(t), func(s *Server) { s.progressInterval = 20 * time.Millisecond })
	key := begin(t, server)
	extension := newExtension(t)
	extension.linkWith(t, key)
	ended := make(chan error, 1)
	vault.shareWith(func(ctx context.Context, asked func(linkproto.Progress)) (linkproto.SharedFile, []byte, error) {
		asked(linkproto.ProgressConfirmOnDevice)
		<-ctx.Done()
		ended <- ctx.Err()
		return linkproto.SharedFile{}, nil, ctx.Err()
	})
	conn, transport := extension.openSession(t, int(key.Port))
	sendRequest(t, conn, transport, shareRequest)
	if first := nextFrame(t, conn, transport); first.Progress != "confirm-on-device" {
		t.Fatalf("first message = %+v", first)
	}
	if err := conn.CloseNow(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-ended:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the share ended with %v", err)
		}
	case <-time.After(testWait):
		t.Fatal("the share kept waiting after the session ended")
	}
}
