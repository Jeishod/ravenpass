package api

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/linkserver"
	"github.com/dortanes/ravenpass/packages/app/linkstore"
)

// fakeLinks stands for the link server and records what the service asks of it.
type fakeLinks struct {
	extensions []linkstore.Extension
	listErr    error
	reachable  bool
	offer      linkserver.Offer
	beginErr   error
	linked     linkstore.Extension
	awaitErr   error
	awaitedCtx context.Context
	canceled   int
	waiting    linkserver.Offer
	unlinked   []string
	unlinkErr  error
	renamed    []string
	renameErr  error
}

func (f *fakeLinks) Extensions() ([]linkstore.Extension, error) { return f.extensions, f.listErr }
func (f *fakeLinks) Reachable() bool                            { return f.reachable }
func (f *fakeLinks) Begin() (linkserver.Offer, error)           { return f.offer, f.beginErr }
func (f *fakeLinks) Cancel()                                    { f.canceled++ }
func (f *fakeLinks) Waiting() (linkserver.Offer, bool)          { return f.waiting, f.waiting.Key != "" }

func (f *fakeLinks) Await(ctx context.Context) (linkstore.Extension, error) {
	f.awaitedCtx = ctx
	return f.linked, f.awaitErr
}

func (f *fakeLinks) Unlink(id string) error {
	f.unlinked = append(f.unlinked, id)
	return f.unlinkErr
}

func (f *fakeLinks) Rename(id, name string) error {
	f.renamed = append(f.renamed, id+" "+name)
	return f.renameErr
}

var linkedAt = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

func TestExtensionLinksListsTheLinkedExtensions(t *testing.T) {
	links := &fakeLinks{
		extensions: []linkstore.Extension{
			{ID: "first", Name: "Chrome · macOS", LinkedAt: linkedAt},
			{ID: "second", Name: "Chrome · Windows", LinkedAt: linkedAt.Add(time.Hour)},
		},
		reachable: true,
	}
	service := &Service{links: links}
	listed, err := service.ExtensionLinks()
	if err != nil {
		t.Fatal(err)
	}
	want := ExtensionLinks{
		Extensions: []LinkedExtension{
			{ID: "first", Name: "Chrome · macOS", LinkedAt: linkedAt.UnixMilli()},
			{ID: "second", Name: "Chrome · Windows", LinkedAt: linkedAt.Add(time.Hour).UnixMilli()},
		},
		Reachable: true,
	}
	if !slices.Equal(listed.Extensions, want.Extensions) || listed.Reachable != want.Reachable {
		t.Fatalf("listed = %+v", listed)
	}

	links.extensions, links.reachable = nil, false
	listed, err = service.ExtensionLinks()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"extensions":[],"reachable":false}` {
		t.Fatalf("nothing linked encodes as %s", encoded)
	}

	links.listErr = linkstore.ErrMalformed
	_, err = service.ExtensionLinks()
	assertFailure(t, err, failureGeneral)
}

func TestBeginExtensionLinkOffersTheKey(t *testing.T) {
	links := &fakeLinks{offer: linkserver.Offer{Key: "key", ExpiresAt: linkedAt}}
	service := &Service{links: links}
	offer, err := service.BeginExtensionLink()
	if err != nil || offer != (ExtensionLinkOffer{Key: "key", ExpiresAt: linkedAt.UnixMilli()}) {
		t.Fatalf("offer = %+v, error = %v", offer, err)
	}
	links.beginErr = linkserver.ErrUnavailable
	_, err = service.BeginExtensionLink()
	assertFailure(t, err, failureLinkUnavailable)
}

func TestAwaitExtensionLinkReportsTheLinkedExtension(t *testing.T) {
	links := &fakeLinks{linked: linkstore.Extension{ID: "linked", Name: "Chrome · macOS", LinkedAt: linkedAt}}
	service := &Service{links: links}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	linked, err := service.AwaitExtensionLink(ctx)
	if err != nil || linked != (LinkedExtension{ID: "linked", Name: "Chrome · macOS", LinkedAt: linkedAt.UnixMilli()}) {
		t.Fatalf("linked = %+v, error = %v", linked, err)
	}
	if links.awaitedCtx != ctx {
		t.Fatal("the call's context did not reach the link server")
	}
	for cause, code := range map[error]failure{
		linkserver.ErrExpired:  failureLinkExpired,
		linkserver.ErrCanceled: failureLinkCanceled,
	} {
		links.awaitErr = cause
		_, err := service.AwaitExtensionLink(ctx)
		assertFailure(t, err, code)
	}
}

func TestCancelExtensionLinkCancelsTheWaitingKey(t *testing.T) {
	links := &fakeLinks{}
	service := &Service{links: links}
	if err := service.CancelExtensionLink(); err != nil {
		t.Fatal(err)
	}
	if links.canceled != 1 {
		t.Fatalf("canceled %d times", links.canceled)
	}
}

// watchedKey records what a copied key waits for and the discard that removes it.
type watchedKey struct {
	ended   <-chan struct{}
	discard func()
}

func (w *watchedKey) watch(ended <-chan struct{}, discard func()) {
	w.ended, w.discard = ended, discard
}

// end checks that the key's wait is over and runs the discard its end triggers.
func (w *watchedKey) end(t *testing.T, clipboard *memoryPasteboard, cause string) {
	t.Helper()
	select {
	case <-w.ended:
	default:
		t.Fatalf("the copied key still waits after %s", cause)
	}
	w.discard()
	if clipboard.text != "" {
		t.Fatalf("the key stayed on the clipboard after %s", cause)
	}
}

func TestCopyExtensionLinkKeyKeepsTheKeyUntilItEnds(t *testing.T) {
	ended := make(chan struct{})
	links := &fakeLinks{waiting: linkserver.Offer{Key: "waiting key", Done: ended}}
	service := &Service{links: links, preferences: newTestPreferences(t)}
	if err := service.SetClipboardClearing(true, 15); err != nil {
		t.Fatal(err)
	}
	clipboard := attachPasteboard(service)
	var clearEarlier func()
	if !service.copyToClipboard("earlier copy", func(_ time.Duration, clear func()) { clearEarlier = clear }) {
		t.Fatal("copy failed")
	}
	var watched watchedKey
	if err := service.copyExtensionLinkKey(watched.watch); err != nil {
		t.Fatal(err)
	}
	if clipboard.text != "waiting key" {
		t.Fatalf("clipboard = %q", clipboard.text)
	}
	if watched.ended != ended {
		t.Fatal("the copy does not wait for the key to end")
	}
	clearEarlier()
	if clipboard.text != "waiting key" {
		t.Fatal("the clearing delay removed a key that can still link")
	}
	close(ended)
	watched.end(t, clipboard, "the key ended")

	clipboard.failing = true
	err := service.copyExtensionLinkKey(func(<-chan struct{}, func()) { t.Fatal("a failed copy waits to be removed") })
	assertFailure(t, err, failureCopyFailed)
	clipboard.failing = false

	links.waiting = linkserver.Offer{}
	err = service.copyExtensionLinkKey(func(<-chan struct{}, func()) { t.Fatal("no key, yet a removal waits") })
	assertFailure(t, err, failureLinkExpired)
}

func TestAnEndedLinkKeyLeavesLaterCopiesAlone(t *testing.T) {
	first := make(chan struct{})
	links := &fakeLinks{waiting: linkserver.Offer{Key: "first key", Done: first}}
	service := &Service{links: links, preferences: newTestPreferences(t)}
	if err := service.SetClipboardClearing(true, 30); err != nil {
		t.Fatal(err)
	}
	clipboard := attachPasteboard(service)
	var watched watchedKey
	if err := service.copyExtensionLinkKey(watched.watch); err != nil {
		t.Fatal(err)
	}
	clipboard.replace("copied elsewhere")
	close(first)
	watched.discard()
	if clipboard.text != "copied elsewhere" {
		t.Fatal("the ended key removed what the user copied since")
	}

	second := make(chan struct{})
	links.waiting = linkserver.Offer{Key: "second key", Done: second}
	if err := service.copyExtensionLinkKey(watched.watch); err != nil {
		t.Fatal(err)
	}
	var delay time.Duration
	var clearPassword func()
	if !service.copyToClipboard("password", func(after time.Duration, clear func()) { delay, clearPassword = after, clear }) {
		t.Fatal("copy failed")
	}
	close(second)
	watched.discard()
	if clipboard.text != "password" {
		t.Fatal("the ended key removed a newer copy")
	}
	if delay != 30*time.Second {
		t.Fatalf("clear delay = %v, want 30s", delay)
	}
	clearPassword()
	if clipboard.text != "" {
		t.Fatal("a copy made after the key stayed past its clearing delay")
	}
}

func TestUnlinkExtensionForgetsTheExtension(t *testing.T) {
	links := &fakeLinks{}
	service := &Service{links: links}
	if err := service.UnlinkExtension("linked"); err != nil {
		t.Fatal(err)
	}
	links.unlinkErr = linkstore.ErrNotFound
	assertFailure(t, service.UnlinkExtension("missing"), failureExtensionNotFound)
	if len(links.unlinked) != 2 || links.unlinked[0] != "linked" || links.unlinked[1] != "missing" {
		t.Fatalf("unlinked = %v", links.unlinked)
	}
}

func TestRenameExtensionNamesTheExtensionAnew(t *testing.T) {
	links := &fakeLinks{}
	service := &Service{links: links}
	if err := service.RenameExtension("linked", "Work laptop"); err != nil {
		t.Fatal(err)
	}
	for cause, code := range map[error]failure{
		linkstore.ErrNotFound:    failureExtensionNotFound,
		linkstore.ErrInvalidName: failureExtensionNameInvalid,
	} {
		links.renameErr = cause
		assertFailure(t, service.RenameExtension("linked", ""), code)
	}
	if len(links.renamed) != 3 || links.renamed[0] != "linked Work laptop" {
		t.Fatalf("renamed = %q", links.renamed)
	}
}

func TestLockingCancelsTheWaitingKey(t *testing.T) {
	service := newReadyService(t)
	links := &fakeLinks{}
	service.links = links
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if links.canceled != 1 {
		t.Fatalf("locking canceled the key %d times", links.canceled)
	}
}

func TestExtensionLinkMethodsDriveTheLinkServer(t *testing.T) {
	service := newReadyService(t)
	listed, err := service.ExtensionLinks()
	if err != nil || len(listed.Extensions) != 0 || !listed.Reachable {
		t.Fatalf("listed = %+v, error = %v", listed, err)
	}
	offer, err := service.BeginExtensionLink()
	if err != nil {
		t.Fatal(err)
	}
	if waiting, ok := service.links.Waiting(); !ok || offer.Key == "" || offer.Key != waiting.Key {
		t.Fatalf("offered key %q is not the key the link server waits on", offer.Key)
	}
	remaining := time.Until(time.UnixMilli(offer.ExpiresAt))
	if remaining <= 4*time.Minute || remaining > 5*time.Minute {
		t.Fatalf("the key expires in %v", remaining)
	}
	clipboard := attachPasteboard(service)
	var watched watchedKey
	if err := service.copyExtensionLinkKey(watched.watch); err != nil || clipboard.text != offer.Key {
		t.Fatalf("copied %q, error = %v", clipboard.text, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.AwaitExtensionLink(ctx)
	assertFailure(t, err, failureLinkCanceled)
	watched.end(t, clipboard, "the wait was canceled")
	assertFailure(t, service.copyExtensionLinkKey(watched.watch), failureLinkExpired)

	begin := func() {
		t.Helper()
		if _, err := service.BeginExtensionLink(); err != nil {
			t.Fatal(err)
		}
	}
	copyKey := func() {
		t.Helper()
		if err := service.copyExtensionLinkKey(watched.watch); err != nil {
			t.Fatal(err)
		}
	}
	begin()
	copyKey()
	if err := service.CancelExtensionLink(); err != nil {
		t.Fatal(err)
	}
	watched.end(t, clipboard, "the key was canceled")

	begin()
	copyKey()
	begin()
	watched.end(t, clipboard, "the key was replaced")

	copyKey()
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	watched.end(t, clipboard, "the vault was locked")
	assertFailure(t, service.copyExtensionLinkKey(watched.watch), failureLinkExpired)
	assertFailure(t, service.UnlinkExtension("missing"), failureExtensionNotFound)
	assertFailure(t, service.RenameExtension("missing", "Work laptop"), failureExtensionNotFound)
	assertFailure(t, service.RenameExtension("missing", " "), failureExtensionNameInvalid)
}
