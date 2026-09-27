package documents

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
)

const (
	address  = "content://com.example.drive/document/1"
	folder   = "content://com.example.drive/tree/backups"
	maxBytes = 16
	// untouched is the kept count of a failing write that leaves the document as it was.
	untouched = -1
)

var (
	errGone     = errors.New("the document is gone")
	errOffline  = errors.New("the drive is out of reach")
	errUpload   = errors.New("the upload failed")
	errFinalize = errors.New("the witness was not saved")
)

// drive is a document provider in memory: a document exists, empty, once a picker created it.
type drive struct {
	mu        sync.Mutex
	documents map[string][]byte
	writes    int
	deleted   []string
	// failRead fails every read, as a provider that cannot reach a document it still has does.
	failRead error
	// failWrite fails every write, after it stores the first kept bytes of the data.
	failWrite error
	kept      int
	// altered stores other content than a write was given.
	altered bool
	// inside counts the calls in progress; most is the highest count seen.
	inside, most int
	// label answers Pick and PickFolder unless canceled or pickErr is set.
	label    storage.Label
	canceled bool
	pickErr  error
	picks    []pick
	// creations records CreateIn calls, each given an address of its own unless createErr is set.
	creations []creation
	createErr error
}

type pick struct {
	create    bool
	folder    bool
	name      string
	mediaType string
}

type creation struct {
	folder    string
	name      string
	mediaType string
}

func newDrive(documents map[string][]byte) *drive {
	return &drive{documents: documents}
}

// enter counts a call as in progress until the returned function runs.
func (d *drive) enter() func() {
	d.mu.Lock()
	d.inside++
	d.most = max(d.most, d.inside)
	d.mu.Unlock()
	runtime.Gosched()
	return func() {
		d.mu.Lock()
		d.inside--
		d.mu.Unlock()
	}
}

func (d *drive) Pick(create bool, name string) (string, storage.Label, bool, error) {
	defer d.enter()()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.picks = append(d.picks, pick{create: create, name: name})
	if d.canceled || d.pickErr != nil {
		return "", storage.Label{}, false, d.pickErr
	}
	if create {
		d.documents[address] = []byte{}
	}
	return address, d.label, true, nil
}

func (d *drive) Create(name, mediaType string) (string, bool, error) {
	defer d.enter()()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.picks = append(d.picks, pick{create: true, name: name, mediaType: mediaType})
	if d.canceled || d.pickErr != nil {
		return "", false, d.pickErr
	}
	d.documents[address] = []byte{}
	return address, true, nil
}

func (d *drive) PickFolder() (string, storage.Label, bool, error) {
	defer d.enter()()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.picks = append(d.picks, pick{folder: true})
	if d.canceled || d.pickErr != nil {
		return "", storage.Label{}, false, d.pickErr
	}
	return folder, d.label, true, nil
}

func (d *drive) CreateIn(parent, name, mediaType string) (string, error) {
	defer d.enter()()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.creations = append(d.creations, creation{folder: parent, name: name, mediaType: mediaType})
	if d.createErr != nil {
		return "", d.createErr
	}
	created := fmt.Sprintf("%s/document/%d", parent, len(d.creations))
	d.documents[created] = []byte{}
	return created, nil
}

func (d *drive) Read(address string, limit int64) ([]byte, error) {
	defer d.enter()()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failRead != nil {
		return nil, d.failRead
	}
	content, found := d.documents[address]
	if !found {
		return nil, storage.ErrNotFound
	}
	if int64(len(content)) > limit {
		return nil, storage.ErrTooLarge
	}
	return bytes.Clone(content), nil
}

func (d *drive) Write(address string, data []byte) error {
	defer d.enter()()
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, found := d.documents[address]; !found {
		return errGone
	}
	d.writes++
	switch {
	case d.failWrite != nil:
		if d.kept != untouched {
			d.documents[address] = bytes.Clone(data[:d.kept])
		}
		return d.failWrite
	case d.altered:
		d.documents[address] = append(bytes.Clone(data), 0)
	default:
		d.documents[address] = bytes.Clone(data)
	}
	return nil
}

func (d *drive) Delete(address string) error {
	defer d.enter()()
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, found := d.documents[address]; !found {
		return errGone
	}
	delete(d.documents, address)
	d.deleted = append(d.deleted, address)
	return nil
}

func open(t *testing.T, backend *Backend) storage.Store {
	t.Helper()
	opened, err := backend.Open(storage.Target{Kind: storage.Document, Path: address}, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	return opened
}

type finalizer struct {
	runs int
	err  error
}

func (f *finalizer) run() error {
	f.runs++
	return f.err
}

func head(content []byte) *[sha256.Size]byte {
	sum := sha256.Sum256(content)
	return &sum
}

func TestAnEmptyDocumentHoldsNoVault(t *testing.T) {
	provider := newDrive(map[string][]byte{address: {}})
	if _, err := open(t, NewBackend(provider)).LoadCiphertext(); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("an empty document: got %v, want ErrNotFound", err)
	}
	provider.documents[address] = []byte("vault")
	loaded, err := open(t, NewBackend(provider)).LoadCiphertext()
	if err != nil || string(loaded) != "vault" {
		t.Fatalf("a document holding a vault: %q, %v", loaded, err)
	}
}

func TestADocumentOutOfReachIsNotAMissingVault(t *testing.T) {
	provider := newDrive(map[string][]byte{address: []byte("vault")})
	provider.failRead = errOffline
	opened := open(t, NewBackend(provider))
	if _, err := opened.LoadCiphertext(); err == nil || errors.Is(err, storage.ErrNotFound) || !errors.Is(err, errOffline) {
		t.Fatalf("a document out of reach: got %v, want its provider's error", err)
	}
	finish := &finalizer{}
	err := opened.CommitCiphertext(nil, []byte("new vault"), finish.run)
	if err == nil || errors.Is(err, storage.ErrStaleHead) || !errors.Is(err, errOffline) {
		t.Fatalf("a new vault over a document out of reach: got %v, want its provider's error", err)
	}
	if provider.writes != 0 || finish.runs != 0 || string(provider.documents[address]) != "vault" {
		t.Fatalf("%d writes and %d finalizers left %q", provider.writes, finish.runs, provider.documents[address])
	}
}

func TestADocumentItsProviderNoLongerHasIsAMissingVault(t *testing.T) {
	provider := newDrive(map[string][]byte{})
	opened := open(t, NewBackend(provider))
	if _, err := opened.LoadCiphertext(); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("a document that is gone: got %v, want ErrNotFound", err)
	}
	finish := &finalizer{}
	err := opened.CommitCiphertext(head([]byte("vault")), []byte("next vault"), finish.run)
	if !errors.Is(err, storage.ErrStaleHead) || !errors.Is(err, storage.ErrVaultMissing) {
		t.Fatalf("saving over a document that is gone: got %v, want ErrVaultMissing", err)
	}
	if provider.writes != 0 || finish.runs != 0 {
		t.Fatalf("%d writes and %d finalizers after the document went", provider.writes, finish.runs)
	}
}

func TestADocumentOverTheLimitIsTooLarge(t *testing.T) {
	provider := newDrive(map[string][]byte{address: bytes.Repeat([]byte{1}, maxBytes+1)})
	if _, err := open(t, NewBackend(provider)).LoadCiphertext(); !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("a document over the limit: got %v, want ErrTooLarge", err)
	}
}

func TestACommitWritesTheVaultOverItsHead(t *testing.T) {
	provider := newDrive(map[string][]byte{address: {}})
	opened := open(t, NewBackend(provider))
	finish := &finalizer{}
	if err := opened.CommitCiphertext(nil, []byte("first"), finish.run); err != nil {
		t.Fatal(err)
	}
	if err := opened.CommitCiphertext(head([]byte("first")), []byte("second"), finish.run); err != nil {
		t.Fatal(err)
	}
	if string(provider.documents[address]) != "second" || finish.runs != 2 {
		t.Fatalf("document %q after %d finalized commits", provider.documents[address], finish.runs)
	}
	if err := opened.ReconcileCiphertext(*head([]byte("second")), finish.run); err != nil || finish.runs != 3 {
		t.Fatalf("reconciling the current head: %v after %d finalizers", err, finish.runs)
	}
}

func TestACommitRefusesAStaleHeadBeforeWriting(t *testing.T) {
	for name, test := range map[string]struct {
		content  []byte
		expected *[sha256.Size]byte
	}{
		"another vault":             {[]byte("other"), head([]byte("vault"))},
		"a vault where none is due": {[]byte("vault"), nil},
		"no vault where one is due": {[]byte{}, head([]byte("vault"))},
	} {
		provider := newDrive(map[string][]byte{address: test.content})
		finish := &finalizer{}
		opened := open(t, NewBackend(provider))
		if err := opened.CommitCiphertext(test.expected, []byte("next"), finish.run); !errors.Is(err, storage.ErrStaleHead) {
			t.Fatalf("%s: got %v, want ErrStaleHead", name, err)
		}
		if test.expected != nil {
			if err := opened.ReconcileCiphertext(*test.expected, finish.run); !errors.Is(err, storage.ErrStaleHead) {
				t.Fatalf("%s: reconciling got %v, want ErrStaleHead", name, err)
			}
		}
		if provider.writes != 0 || finish.runs != 0 {
			t.Fatalf("%s: %d writes and %d finalizers after a stale head", name, provider.writes, finish.runs)
		}
	}
}

func TestACommitRefusesWhatNoStoreKeeps(t *testing.T) {
	provider := newDrive(map[string][]byte{address: {}})
	opened := open(t, NewBackend(provider))
	finish := &finalizer{}
	if err := opened.CommitCiphertext(nil, nil, finish.run); !errors.Is(err, storage.ErrEmptyCiphertext) {
		t.Fatalf("an empty vault: got %v", err)
	}
	if err := opened.CommitCiphertext(nil, bytes.Repeat([]byte{1}, maxBytes+1), finish.run); !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("a vault over the limit: got %v", err)
	}
	if err := opened.CommitCiphertext(nil, []byte("vault"), nil); !errors.Is(err, storage.ErrFinalizerRequired) {
		t.Fatalf("a commit without a finalizer: got %v", err)
	}
	if err := opened.ReconcileCiphertext(*head([]byte("vault")), nil); !errors.Is(err, storage.ErrFinalizerRequired) {
		t.Fatalf("a reconcile without a finalizer: got %v", err)
	}
	if provider.writes != 0 {
		t.Fatalf("%d writes of refused commits", provider.writes)
	}
}

func TestAWriteThatReadsBackOtherContentIsUncertain(t *testing.T) {
	provider := newDrive(map[string][]byte{address: {}})
	provider.altered = true
	finish := &finalizer{}
	err := open(t, NewBackend(provider)).CommitCiphertext(nil, []byte("vault"), finish.run)
	if !errors.Is(err, storage.ErrDurabilityUncertain) || finish.runs != 0 {
		t.Fatalf("a write read back altered: got %v after %d finalizers", err, finish.runs)
	}
}

func TestAFailedWriteIsUncertainOnceItMayHaveTouchedTheVault(t *testing.T) {
	for name, test := range map[string]struct {
		kept      int
		uncertain bool
	}{
		"untouched": {untouched, false},
		"truncated": {0, true},
		"partial":   {3, true},
	} {
		provider := newDrive(map[string][]byte{address: []byte("vault")})
		provider.failWrite, provider.kept = errUpload, test.kept
		finish := &finalizer{}
		err := open(t, NewBackend(provider)).CommitCiphertext(head([]byte("vault")), []byte("next vault"), finish.run)
		if err == nil || errors.Is(err, storage.ErrDurabilityUncertain) != test.uncertain || finish.runs != 0 {
			t.Fatalf("%s: got %v after %d finalizers, uncertain %t", name, err, finish.runs, test.uncertain)
		}
		if !test.uncertain && !errors.Is(err, errUpload) {
			t.Fatalf("%s: got %v, want the provider's error", name, err)
		}
	}
}

func TestAFailedFinalizerFailsTheCommit(t *testing.T) {
	provider := newDrive(map[string][]byte{address: {}})
	finish := &finalizer{err: errFinalize}
	err := open(t, NewBackend(provider)).CommitCiphertext(nil, []byte("vault"), finish.run)
	if !errors.Is(err, storage.ErrFinalizerFailed) || !errors.Is(err, errFinalize) {
		t.Fatalf("a failed finalizer: got %v", err)
	}
	if string(provider.documents[address]) != "vault" {
		t.Fatalf("the document holds %q, want the written vault", provider.documents[address])
	}
}

func TestDiscardEmptyKeepsADocumentHoldingAVault(t *testing.T) {
	provider := newDrive(map[string][]byte{address: []byte("vault")})
	placeholder := open(t, NewBackend(provider)).(storage.Placeholder)
	if err := placeholder.DiscardEmpty(); err != nil || len(provider.deleted) != 0 {
		t.Fatalf("discarding a vault: %v, deleted %v", err, provider.deleted)
	}
	provider.documents[address] = []byte{}
	if err := placeholder.DiscardEmpty(); err != nil || len(provider.deleted) != 1 {
		t.Fatalf("discarding an empty document: %v, deleted %v", err, provider.deleted)
	}
}

func TestDiscardEmptyKeepsADocumentAVaultWasOpenedAt(t *testing.T) {
	provider := newDrive(map[string][]byte{address: {}})
	opened, err := NewBackend(provider).Open(storage.Target{Kind: storage.Document, Path: address, Vault: "known-vault"}, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.(storage.Placeholder).DiscardEmpty(); !errors.Is(err, errKnownVault) || len(provider.deleted) != 0 {
		t.Fatalf("discarding a known vault's empty document: %v, deleted %v", err, provider.deleted)
	}
	if _, kept := provider.documents[address]; !kept {
		t.Fatal("a known vault's document was deleted")
	}
}

func TestRemoveDeletesTheDocument(t *testing.T) {
	provider := newDrive(map[string][]byte{address: []byte("vault")})
	opened := open(t, NewBackend(provider))
	if opened.Restricted() {
		t.Fatal("a document reports its provider's sharing as restricted")
	}
	if err := opened.Remove(); err != nil || len(provider.deleted) != 1 {
		t.Fatalf("removing a vault: %v, deleted %v", err, provider.deleted)
	}
	if err := opened.Remove(); !errors.Is(err, errGone) {
		t.Fatalf("removing a document that is gone: got %v", err)
	}
}

func TestStoresOfOneBackendTakeTurns(t *testing.T) {
	provider := newDrive(map[string][]byte{address: {}})
	backend := NewBackend(provider)
	const writers = 8
	results := make(chan error, writers)
	for writer := range writers {
		opened := open(t, backend)
		go func() {
			results <- opened.CommitCiphertext(nil, []byte{byte(writer + 1)}, func() error { return nil })
		}()
	}
	committed := 0
	for range writers {
		switch err := <-results; {
		case err == nil:
			committed++
		case !errors.Is(err, storage.ErrStaleHead):
			t.Fatalf("a concurrent commit: %v", err)
		}
	}
	if committed != 1 || provider.most != 1 {
		t.Fatalf("%d commits landed over an empty document, %d calls at once", committed, provider.most)
	}
}

func TestCheckAcceptsOnlyDocumentAddresses(t *testing.T) {
	backend := NewBackend(newDrive(map[string][]byte{}))
	if backend.Kind() != storage.Document {
		t.Fatalf("kind %q", backend.Kind())
	}
	for _, path := range []string{
		address,
		"content://com.android.externalstorage.documents/document/primary%3ARavenpass%2Fvault.rpv",
	} {
		if err := backend.Check(path); err != nil {
			t.Fatalf("%q: %v", path, err)
		}
	}
	for _, path := range []string{
		"",
		"vault.rpv",
		"/sdcard/Ravenpass/vault.rpv",
		"file:///sdcard/Ravenpass/vault.rpv",
		"content:///document/1",
		"content:document",
		"https://drive.example.com/vault.rpv",
	} {
		if err := backend.Check(path); !errors.Is(err, storage.ErrInvalidPath) {
			t.Fatalf("%q: got %v, want ErrInvalidPath", path, err)
		}
		if _, err := backend.Open(storage.Target{Kind: storage.Document, Path: path}, maxBytes); !errors.Is(err, storage.ErrInvalidPath) {
			t.Fatalf("opening %q: got %v, want ErrInvalidPath", path, err)
		}
	}
}
