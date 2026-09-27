package linkserver

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/dortanes/ravenpass/packages/app/linkproto"
)

func TestFillsAndCodesReportProgressWhileThePersonConfirms(t *testing.T) {
	server, vault := newServer(t, recordsPath(t), func(s *Server) {
		s.progressInterval = 40 * time.Millisecond
		s.idleTimeout = 100 * time.Millisecond
	})
	key := begin(t, server)
	extension := newExtension(t)
	extension.linkWith(t, key)
	conn, transport := extension.openSession(t, int(key.Port))
	for payload, answer := range map[string]string{
		`{"id":7,"type":"fill","credential":"` + exampleID + `","origin":"https://example.com"}`: `{"login":"alex","email":"alex@example.com","password":"secret"}`,
		`{"id":7,"type":"code","credential":"` + exampleID + `","origin":"https://example.com"}`: `{"code":"287082","digits":6,"period":30,"expiresAt":1790000010000}`,
	} {
		vault.verifyWith(verifiedAfter(linkproto.ProgressConfirmOnDevice, 250*time.Millisecond, nil))
		sendRequest(t, conn, transport, payload)
		progress, final := progressThen(t, conn, transport)
		if len(progress) < 2 || slices.ContainsFunc(progress, func(p string) bool { return p != "confirm-on-device" }) {
			t.Fatalf("progress = %q", progress)
		}
		if final.ID != 7 || final.Error != "" || string(final.Result) != answer {
			t.Fatalf("after the progress = %+v, want %s", final, answer)
		}

		vault.verifyWith(verifiedAfter(linkproto.ProgressConfirmInRavenpass, 10*time.Millisecond, ErrDeclined))
		sendRequest(t, conn, transport, payload)
		progress, refusal := progressThen(t, conn, transport)
		if !slices.Equal(progress, []string{"confirm-in-ravenpass"}) || refusal.ID != 7 || refusal.Error != "declined" || refusal.Result != nil {
			t.Fatalf("progress %q, then %+v", progress, refusal)
		}
	}
	vault.verifyWith(nil)
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if id := linkedID(t, server, extension); len(vault.fillers) != 2 || vault.fillers[0] != id || vault.fillers[1] != id {
		t.Fatalf("fills were asked for %q, want the linked extension %q", vault.fillers, id)
	}
}

func TestFillsWaitingForThePersonLeaveConnectionsToStatusRequests(t *testing.T) {
	server, vault := newServer(t, recordsPath(t))
	key := begin(t, server)
	extension := newExtension(t)
	extension.linkWith(t, key)
	confirmed := make(chan struct{})
	vault.verifyWith(func(ctx context.Context, asked func(linkproto.Progress)) error {
		asked(linkproto.ProgressConfirmInRavenpass)
		select {
		case <-confirmed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	fill := `{"id":7,"type":"fill","credential":"` + exampleID + `","origin":"https://example.com"}`
	waiting := make([]*websocket.Conn, 0, maxConnections)
	for range maxConnections {
		conn, transport := extension.openSession(t, int(key.Port))
		sendRequest(t, conn, transport, fill)
		if first := nextFrame(t, conn, transport); first.Progress != "confirm-in-ravenpass" {
			t.Fatalf("first message = %+v", first)
		}
		waiting = append(waiting, conn)
	}

	conn, transport := extension.openSession(t, int(key.Port))
	if reply := request(t, conn, transport, `{"id":1,"type":"status"}`); reply != `{"id":1,"result":{"vault":"locked"}}` {
		t.Fatalf("status while fills wait = %s", reply)
	}

	close(confirmed)
	for _, waiter := range append(waiting, conn) {
		if err := waiter.Close(websocket.StatusNormalClosure, ""); err != nil {
			t.Fatal(err)
		}
	}
}
