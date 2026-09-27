package api

import (
	"bytes"
	"image/jpeg"
	"testing"
	"time"

	"github.com/dortanes/ravenpass/packages/app/photos"
	"github.com/dortanes/ravenpass/packages/vault"
)

func scanDocuments(first, second []string) []IdentityDocument {
	return []IdentityDocument{
		{Type: "passport", Number: "X1234567", Scans: first},
		{Type: "id-card", Number: "ID-99", Scans: second},
	}
}

func stageFixture(t *testing.T, service *Service, name string) ScanDraft {
	t.Helper()
	draft, err := service.stageScan(name, photoFixture(t, name))
	if err != nil || !draft.Chosen || draft.Token == "" {
		t.Fatalf("staging %s = %+v, error = %v", name, draft, err)
	}
	return draft
}

func revision(t *testing.T, service *Service) uint64 {
	t.Helper()
	state, err := service.vault.State()
	if err != nil {
		t.Fatal(err)
	}
	return state.Head.Revision
}

func readInput(t *testing.T, service *Service, id string) IdentityInput {
	t.Helper()
	identity, err := service.ReadIdentity(id)
	if err != nil {
		t.Fatal(err)
	}
	return identity.IdentityInput
}

// storedScanIdentity creates an identity whose passport holds one stored PDF scan.
func storedScanIdentity(t *testing.T) (*Service, string, string) {
	t.Helper()
	service := newReadyService(t)
	draft := stageFixture(t, service, "scan.pdf")
	owner, err := service.CreateIdentity(IdentityInput{Label: "Alex", Documents: scanDocuments([]string{"new:" + draft.Token}, []string{})}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return service, owner, readInput(t, service, owner).Documents[0].Scans[0]
}

func TestNewDocumentTakesTwoScansInOneSave(t *testing.T) {
	service := newReadyService(t)
	picture := stageFixture(t, service, "orientation-6.jpg")
	pdf := stageFixture(t, service, "scan.pdf")
	if picture.MediaType != vault.MediaJPEG || picture.Name != "orientation-6.jpg" || picture.Thumbnail == "" {
		t.Fatalf("picture draft = %+v", picture)
	}
	if pdf.MediaType != vault.MediaPDF || pdf.Thumbnail != "" {
		t.Fatalf("PDF draft = %+v", pdf)
	}
	before := revision(t, service)
	owner, err := service.CreateIdentity(IdentityInput{Label: "Alex", Documents: scanDocuments([]string{"new:" + picture.Token, "new:" + pdf.Token}, []string{})}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if after := revision(t, service); after != before+1 {
		t.Fatalf("the save took %d revisions", after-before)
	}
	identity, err := service.ReadIdentity(owner)
	if err != nil {
		t.Fatal(err)
	}
	scans := identity.Documents[0].Scans
	if len(scans) != 2 || len(identity.Attachments) != 2 || len(identity.Documents[1].Scans) != 0 {
		t.Fatalf("scans = %v, attachments = %+v", scans, identity.Attachments)
	}
	stored, err := service.readScan(scans[0])
	if err != nil {
		t.Fatal(err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(stored.Content))
	if err != nil || config.Width != 48 || config.Height != 32 {
		t.Fatalf("stored picture is %dx%d, error = %v", config.Width, config.Height, err)
	}
	stored, err = service.readScan(scans[1])
	if err != nil || !bytes.Equal(stored.Content, photoFixture(t, "scan.pdf")) {
		t.Fatalf("stored PDF = %s, error = %v", stored.Name, err)
	}
	if identities, err := service.ListIdentities(); err != nil || len(identities) != 1 {
		t.Fatalf("identities = %+v, error = %v", identities, err)
	}
}

func TestChoosingAScanRunsTheVaultRules(t *testing.T) {
	service := newReadyService(t)
	tests := []struct {
		name string
		data []byte
		want failure
	}{
		{"file named like a PDF that is not one", photoFixture(t, "fake.pdf"), failureScanUnsupported},
		{"PDF without its end marker", []byte("%PDF-1.4\ncut short"), failureScanUnsupported},
		{"file over the size bound", make([]byte, photos.MaxFileBytes+1), failureScanTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.stageScan("scan.pdf", test.data)
			assertFailure(t, err, test.want)
		})
	}
	for range maxStagedScans {
		stageFixture(t, service, "scan.pdf")
	}
	_, err := service.stageScan("scan.pdf", photoFixture(t, "scan.pdf"))
	assertFailure(t, err, failureScanLimit)
}

func TestDiscardKeepsTheStoredScans(t *testing.T) {
	service, owner, stored := storedScanIdentity(t)
	draft := stageFixture(t, service, "upright.png")
	if err := service.DiscardScans(); err != nil {
		t.Fatal(err)
	}
	before := revision(t, service)
	input := readInput(t, service, owner)
	input.Documents[0].Scans = append(input.Documents[0].Scans, "new:"+draft.Token)
	assertFailure(t, service.UpdateIdentity(owner, input, nil), failureFileNotSelected)
	if revision(t, service) != before {
		t.Fatal("a save naming a discarded scan changed the vault")
	}
	if scans := readInput(t, service, owner).Documents[0].Scans; len(scans) != 1 || scans[0] != stored {
		t.Fatalf("stored scans after a cancel = %v", scans)
	}
}

func TestStagedTokensAreUsedOnce(t *testing.T) {
	service, owner, _ := storedScanIdentity(t)
	draft := stageFixture(t, service, "scan.pdf")
	input := readInput(t, service, owner)
	twice := input
	twice.Documents = scanDocuments(append(input.Documents[0].Scans, "new:"+draft.Token), []string{"new:" + draft.Token})
	before := revision(t, service)
	assertFailure(t, service.UpdateIdentity(owner, twice, nil), failureFileNotSelected)
	unknown := input
	unknown.Documents = scanDocuments(append(input.Documents[0].Scans, "new:0123"), []string{})
	assertFailure(t, service.UpdateIdentity(owner, unknown, nil), failureFileNotSelected)
	if revision(t, service) != before {
		t.Fatal("a refused token changed the vault")
	}

	once := input
	once.Documents = scanDocuments(append(input.Documents[0].Scans, "new:"+draft.Token), []string{})
	if err := service.UpdateIdentity(owner, once, nil); err != nil {
		t.Fatal(err)
	}
	after := revision(t, service)
	assertFailure(t, service.UpdateIdentity(owner, once, nil), failureFileNotSelected)
	if revision(t, service) != after {
		t.Fatal("a reused token changed the vault")
	}
	if scans := readInput(t, service, owner).Documents[0].Scans; len(scans) != 2 {
		t.Fatalf("scans after one save = %v", scans)
	}
}

func TestRefusedSaveKeepsTheStagedScans(t *testing.T) {
	service := newReadyService(t)
	draft := stageFixture(t, service, "scan.pdf")
	input := IdentityInput{Documents: scanDocuments([]string{"new:" + draft.Token}, []string{})}
	_, err := service.CreateIdentity(input, nil)
	assertFailure(t, err, failureInvalidItem)
	input.Label = "Alex"
	if _, err := service.CreateIdentity(input, nil); err != nil {
		t.Fatalf("saving again after a refusal: %v", err)
	}
}

func TestLockClearsTheStagedScans(t *testing.T) {
	service := newReadyService(t)
	draft := stageFixture(t, service, "scan.pdf")
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, staged := service.scans.find(draft.Token); staged {
		t.Fatal("a staged scan survived the lock")
	}
}

func TestScanLimitCountsStoredAndNewScans(t *testing.T) {
	service, owner, _ := storedScanIdentity(t)
	input := readInput(t, service, owner)
	for range 2 {
		input.Documents[0].Scans = append(input.Documents[0].Scans, "new:"+stageFixture(t, service, "scan.pdf").Token)
	}
	if err := service.UpdateIdentity(owner, input, nil); err != nil {
		t.Fatal(err)
	}
	input = readInput(t, service, owner)
	for range 2 {
		input.Documents[0].Scans = append(input.Documents[0].Scans, "new:"+stageFixture(t, service, "scan.pdf").Token)
	}
	before := revision(t, service)
	assertFailure(t, service.UpdateIdentity(owner, input, nil), failureScanLimit)
	if revision(t, service) != before {
		t.Fatal("a fifth scan changed the vault")
	}
}

func TestCopiedScanIsClearedOnlyWhileItIsStillThere(t *testing.T) {
	service, _, pdf := storedScanIdentity(t)
	board := attachPasteboard(service)
	var discard func()
	schedule := func(_ time.Duration, clear func()) { discard = clear }

	if err := service.copyScan(pdf, schedule); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(board.content, photoFixture(t, "scan.pdf")) || board.mediaType != vault.MediaPDF {
		t.Fatalf("pasteboard holds %q as %s", board.content, board.mediaType)
	}
	discard()
	if board.content != nil {
		t.Fatal("the copied scan stayed on the pasteboard after its lifetime")
	}

	if err := service.copyScan(pdf, schedule); err != nil {
		t.Fatal(err)
	}
	board.replace("something else")
	discard()
	if board.text != "something else" {
		t.Fatal("clearing removed what another app put on the pasteboard")
	}

	if err := service.copyScan(pdf, schedule); err != nil {
		t.Fatal(err)
	}
	stale := discard
	if !service.copyToClipboard("text", schedule) {
		t.Fatal("text copy failed")
	}
	stale()
	if board.text != "text" {
		t.Fatal("a later text copy let the earlier scan timer clear the pasteboard")
	}

	if err := service.copyScan(pdf, schedule); err != nil {
		t.Fatal(err)
	}
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if board.content != nil {
		t.Fatal("locking left the copied scan on the pasteboard")
	}
}

func TestCopyScanReportsARefusedPasteboard(t *testing.T) {
	service, owner, pdf := storedScanIdentity(t)
	service.pasteboard = &memoryPasteboard{failing: true}
	assertFailure(t, service.copyScan(pdf, func(time.Duration, func()) {}), failureCopyFailed)
	assertFailure(t, service.copyScan(owner, func(time.Duration, func()) {}), failureItemUnreadable)
}

func TestScansAreReachedOnlyThroughTheirDocument(t *testing.T) {
	service, owner, pdf := storedScanIdentity(t)
	assertFailure(t, service.DeleteItem(pdf), failureItemUnreadable)
	assertFailure(t, service.SetPinned(pdf, true), failureItemUnreadable)
	_, err := service.readScan(owner)
	assertFailure(t, err, failureItemUnreadable)
	_, err = service.readScan("not a scan")
	assertFailure(t, err, failureItemUnreadable)

	kept := readInput(t, service, owner)
	if err := service.UpdateIdentity(owner, kept, nil); err != nil {
		t.Fatalf("an update passing the scans back: %v", err)
	}
	if _, err := service.readScan(pdf); err != nil {
		t.Fatalf("a scan passed back was lost: %v", err)
	}
	kept.Documents = kept.Documents[1:]
	if err := service.UpdateIdentity(owner, kept, nil); err != nil {
		t.Fatal(err)
	}
	_, err = service.readScan(pdf)
	assertFailure(t, err, failureItemUnreadable)
	identity, err := service.ReadIdentity(owner)
	if err != nil || len(identity.Attachments) != 0 || identity.Attachments == nil {
		t.Fatalf("attachments after dropping the document = %+v, error = %v", identity.Attachments, err)
	}
}
