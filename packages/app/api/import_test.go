package api

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/storage"
	"github.com/dortanes/ravenpass/packages/importers"
	"github.com/dortanes/ravenpass/packages/importers/bitwarden"
	"github.com/dortanes/ravenpass/packages/vault"
)

const bitwardenExport = `{
  "encrypted": false,
  "folders": [{"id": "f1", "name": "Work"}],
  "items": [
    {"id": "1", "type": 1, "name": "Mail", "folderId": "f1", "favorite": true,
     "login": {"uris": [{"uri": "https://mail.example.test"}], "username": "alex", "password": "imported-secret", "totp": "JBSWY3DPEHPK3PXP"}},
    {"id": "2", "type": 2, "name": "Router", "notes": "admin panel", "secureNote": {"type": 0}}
  ]
}`

func writeExport(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bitwarden_export.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func stageExport(t *testing.T, service *Service, content string) ImportChoice {
	t.Helper()
	choice, err := service.stageImport(pickedFile{path: writeExport(t, content)}, bitwarden.Open)
	if err != nil {
		t.Fatal(err)
	}
	return choice
}

func previewKind(preview ImportPreview, kind string) ImportKind {
	for _, listed := range preview.Kinds {
		if listed.Kind == kind {
			return listed
		}
	}
	return ImportKind{}
}

// lockedExport is an export that opens with the password "right".
type lockedExport struct {
	locked bool
	closed bool
	export importers.Export
}

func (f *lockedExport) Format() importers.Format { return importers.FormatEncryptedJSON }
func (f *lockedExport) Locked() bool             { return f.locked }
func (f *lockedExport) Close()                   { f.closed = true }

func (f *lockedExport) Unlock(password string) error {
	if password != "right" {
		return importers.ErrWrongPassword
	}
	f.locked = false
	return nil
}

func (f *lockedExport) Read(importers.Labels) (importers.Export, error) {
	if f.locked {
		return importers.Export{}, importers.ErrLocked
	}
	return f.export, nil
}

func TestImportAddsTheStagedExportInOneSave(t *testing.T) {
	service := newReadyService(t)
	choice := stageExport(t, service, bitwardenExport)
	if !choice.Chosen || choice.Locked || choice.Name != "bitwarden_export.json" {
		t.Fatalf("choice = %+v", choice)
	}
	preview := choice.Preview
	if preview.Format != "json" || preview.Items != 2 || previewKind(preview, "credential").Count != 1 || previewKind(preview, "credential").OneTimeCodes != 1 || previewKind(preview, "note").Count != 1 || preview.Groups.New != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	before := revision(t, service)
	result, err := service.ImportItems(ImportOptions{Groups: true, SkipDuplicates: true})
	if err != nil {
		t.Fatal(err)
	}
	if revision(t, service) != before+1 {
		t.Fatal("the import did not write exactly one save")
	}
	want := ImportResult{Added: 2, Kinds: []ImportKindTotal{{"credential", 1}, {"note", 1}}, Groups: 1, Format: "json", Located: true}
	if result.Added != want.Added || result.Groups != want.Groups || result.Format != want.Format || result.Located != want.Located || len(result.Kinds) != 2 || result.Kinds[0] != want.Kinds[0] || result.Kinds[1] != want.Kinds[1] {
		t.Fatalf("result = %+v", result)
	}
	credentials, err := service.ListCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 1 || credentials[0].Label != "Mail" || !credentials[0].Pinned || len(credentials[0].Groups) != 1 {
		t.Fatalf("credentials = %+v", credentials)
	}
	if _, err := service.ImportItems(ImportOptions{}); err == nil {
		t.Fatal("the same export imported twice")
	} else {
		assertFailure(t, err, failureImportNotActive)
	}
}

func TestImportPlansAgainstTheVaultAsItIsNow(t *testing.T) {
	service := newReadyService(t)
	choice := stageExport(t, service, bitwardenExport)
	if previewKind(choice.Preview, "credential").Duplicates != 0 {
		t.Fatalf("preview = %+v", choice.Preview)
	}
	if _, err := service.CreateCredential(CredentialInput{Label: "mail ", Websites: []string{"https://mail.example.test"}, Login: "alex"}, nil); err != nil {
		t.Fatal(err)
	}
	result, err := service.ImportItems(ImportOptions{SkipDuplicates: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Groups != 0 || result.Kinds[0].Kind != "note" {
		t.Fatalf("result = %+v", result)
	}
}

func TestImportAddsASeedPhraseKeptInANoteAsASeed(t *testing.T) {
	service := newReadyService(t)
	choice := stageExport(t, service, `{"encrypted": false, "items": [
		{"type": 2, "name": "Anytype", "notes": "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"}
	]}`)
	if seed := previewKind(choice.Preview, "seed"); seed.Count != 1 || len(seed.Converted) != 1 || seed.Converted[0].From != "note" {
		t.Fatalf("preview = %+v", choice.Preview)
	}
	result, err := service.ImportItems(ImportOptions{SkipDuplicates: true})
	if err != nil || len(result.Kinds) != 1 || result.Kinds[0] != (ImportKindTotal{Kind: "seed", Count: 1}) {
		t.Fatalf("result = %+v, %v", result, err)
	}
	seeds, err := service.ListSeeds()
	if err != nil || len(seeds) != 1 || seeds[0].Label != "Anytype" {
		t.Fatalf("seeds = %+v, %v", seeds, err)
	}
}

// aliasVaultExport is an AliasVault CSV export with a login, an alias, a card and a note.
const aliasVaultExport = "ServiceName,FolderPath,ServiceUrl,Username,CurrentPassword,AliasEmail,TwoFactorSecret,AliasGender,AliasFirstName,AliasLastName,AliasNickName,AliasBirthDate,CardholderName,CardNumber,CardExpiryMonth,CardExpiryYear,CardCvv,CardPin,Notes,CreatedAt,UpdatedAt\r\n" +
	"Mail,Work,https://mail.example.test,alex,imported-secret,,JBSWY3DPEHPK3PXP,,,,,,,,,,,,,09/12/2025 17:28:39,09/12/2025 17:28:39\r\n" +
	"Shop,,https://shop.example.test,,alias-secret,shopper@example.test,,Female,Ann,Lee,,01/02/1990 00:00:00,,,,,,,,09/12/2025 17:28:39,09/12/2025 17:28:39\r\n" +
	"Visa,,,,,,,,,,,,Ann Lee,4111111111111111,12,2030,123,1234,,09/12/2025 17:28:39,09/12/2025 17:28:39\r\n" +
	"Router,,,,,,,,,,,,,,,,,,admin panel,09/12/2025 17:28:39,09/12/2025 17:28:39\r\n"

func TestImportReadsAnAliasVaultExport(t *testing.T) {
	service := newReadyService(t)
	path := filepath.Join(t.TempDir(), "aliasvault-export.csv")
	if err := os.WriteFile(path, []byte(aliasVaultExport), 0o600); err != nil {
		t.Fatal(err)
	}
	choice, err := service.stageImport(pickedFile{path: path}, importSources["aliasvault"].open)
	if err != nil {
		t.Fatal(err)
	}
	preview := choice.Preview
	credential := previewKind(preview, "credential")
	if preview.Format != "csv" || preview.Items != 4 || credential.Count != 2 || credential.OneTimeCodes != 1 || len(credential.Converted) != 1 || credential.Converted[0] != (ImportConversion{From: "alias", Count: 1}) {
		t.Fatalf("preview = %+v", preview)
	}
	if previewKind(preview, "card").Count != 1 || previewKind(preview, "note").Count != 1 || preview.Groups.New != 1 || !slices.Equal(preview.CardIssuers, []string{"41111111"}) {
		t.Fatalf("preview = %+v", preview)
	}
	_, err = service.ImportItems(ImportOptions{CardNetworks: map[string]string{"41111111": "visa-electron"}})
	assertFailure(t, err, failureInvalidItem)
	result, err := service.ImportItems(ImportOptions{Groups: true, SkipDuplicates: true, CardNetworks: map[string]string{"41111111": "visa"}})
	if err != nil || result.Added != 4 || result.Groups != 1 || result.Format != "csv" {
		t.Fatalf("result = %+v, %v", result, err)
	}
	cards, err := service.ListCards()
	if err != nil || len(cards) != 1 || cards[0].Network != "visa" {
		t.Fatalf("cards = %+v, %v", cards, err)
	}
}

func TestImportWithOnlyDuplicatesLeftIsRefused(t *testing.T) {
	service := newReadyService(t)
	stageExport(t, service, `{"encrypted": false, "items": [{"type": 2, "name": "Router", "notes": "admin"}]}`)
	if _, err := service.CreateNote(NoteInput{Label: "Router", Body: "admin"}, nil); err != nil {
		t.Fatal(err)
	}
	_, err := service.ImportItems(ImportOptions{SkipDuplicates: true})
	assertFailure(t, err, failureImportEmpty)
	if result, err := service.ImportItems(ImportOptions{}); err != nil || result.Added != 1 {
		t.Fatalf("keeping duplicates = %+v, %v", result, err)
	}
}

func TestLockedExportNeedsItsPassword(t *testing.T) {
	service := newReadyService(t)
	file := &lockedExport{locked: true, export: importers.Export{
		Format: importers.FormatEncryptedJSON,
		Items:  []importers.Item{{Content: vault.NewItem{Note: &vault.NoteInput{Label: "Router"}}, Origin: importers.OriginNote}},
	}}
	choice, err := service.stageImport(pickedFile{path: writeExport(t, "{}")}, func(io.ReaderAt, int64) (importers.File, error) { return file, nil })
	if err != nil {
		t.Fatal(err)
	}
	if !choice.Locked || choice.Preview.Kinds == nil || choice.Preview.Skipped == nil {
		t.Fatalf("choice = %+v", choice)
	}
	_, err = service.ImportItems(ImportOptions{})
	assertFailure(t, err, failureImportNotActive)
	_, err = service.UnlockImportFile("wrong")
	assertFailure(t, err, failureImportPasswordWrong)
	preview, err := service.UnlockImportFile("right")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Format != "encrypted-json" || previewKind(preview, "note").Count != 1 || !file.closed {
		t.Fatalf("preview = %+v, closed = %t", preview, file.closed)
	}
	result, err := service.ImportItems(ImportOptions{})
	if err != nil || result.Format != "encrypted-json" {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestLockAndCancelDropTheStagedExport(t *testing.T) {
	for name, drop := range map[string]func(*Service) error{
		"lock":   (*Service).Lock,
		"cancel": (*Service).CancelImport,
	} {
		t.Run(name, func(t *testing.T) {
			service := newReadyService(t)
			stageExport(t, service, bitwardenExport)
			if err := drop(service); err != nil {
				t.Fatal(err)
			}
			_, err := service.ImportItems(ImportOptions{})
			assertFailure(t, err, failureImportNotActive)
			assertFailure(t, service.RevealImportFile(), failureImportNotActive)
		})
	}
}

func TestUnreadableExportsNameTheirCause(t *testing.T) {
	service := newReadyService(t)
	tests := []struct {
		name string
		path string
		want failure
	}{
		{"not an export", writeExport(t, "hello"), failureImportUnrecognized},
		{"account bound", writeExport(t, `{"encrypted": true, "encKeyValidation_DO_NOT_EDIT": "2.a|b|c", "folders": [], "items": []}`), failureImportAccountBound},
		{"a folder", t.TempDir(), failureImportUnrecognized},
		{"a file gone since it was chosen", filepath.Join(t.TempDir(), "gone.json"), failureImportUnrecognized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.stageImport(pickedFile{path: test.path}, bitwarden.Open)
			assertFailure(t, err, test.want)
			_, err = service.ImportItems(ImportOptions{})
			assertFailure(t, err, failureImportNotActive)
		})
	}
}

func TestImportCallsNeedASourceAndAWindow(t *testing.T) {
	service := newReadyService(t)
	_, err := service.ChooseImportFile("other")
	assertFailure(t, err, failureImportSourceUnknown)
	for source := range importSources {
		_, err = service.ChooseImportFile(source)
		assertFailure(t, err, failureWindowUnavailable)
	}
	assertFailure(t, service.TrashImportFile(), failureImportNotActive)
	stageExport(t, service, bitwardenExport)
	assertFailure(t, service.TrashImportFile(), failureImportNotActive)
	if _, err := service.ImportItems(ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	assertFailure(t, service.RevealImportFile(), failureWindowUnavailable)
}

func TestImportFailuresNameTheExport(t *testing.T) {
	causes := map[error]failure{
		importers.ErrUnrecognized:          failureImportUnrecognized,
		importers.ErrLocked:                failureImportUnrecognized,
		importers.ErrTooLarge:              failureImportTooLarge,
		importers.ErrAccountBound:          failureImportAccountBound,
		importers.ErrWrongPassword:         failureImportPasswordWrong,
		importers.ErrUnsupportedEncryption: failureImportEncryption,
	}
	for cause, code := range causes {
		assertFailure(t, presentImport(errors.Join(errors.New("reading"), cause)), code)
	}
	assertFailure(t, presentImport(vault.ErrResourceLimit), failureResourceLimit)
	assertFailure(t, presentImportSave(vault.ErrResourceLimit), failureImportLimit)
	assertFailure(t, presentImportSave(storage.ErrTooLarge), failureImportLimit)
	assertFailure(t, presentImportSave(storage.ErrStaleHead), failureVaultChanged)
}

func TestImportPreviewCountsImportedAndLeftBehindPasskeysApart(t *testing.T) {
	preview := importPreview(importers.Preview{Passkeys: 1, ImportedPasskeys: 3})
	if preview.Passkeys != 1 || preview.ImportedPasskeys != 3 {
		t.Fatalf("passkeys left behind %d, imported %d", preview.Passkeys, preview.ImportedPasskeys)
	}
}

func TestImportSlicesAreWrittenAsLists(t *testing.T) {
	encoded, err := json.Marshal(ImportChoice{Preview: emptyPreview()})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"kinds":[]`, `"skipped":[]`} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("%s is missing from %s", field, encoded)
		}
	}
	preview := importPreview(importers.Preview{Kinds: []importers.KindPreview{{Kind: vault.KindNote, Count: 1}}})
	encoded, err = json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"converted":[]`) {
		t.Fatalf("conversions are not a list in %s", encoded)
	}
	if encoded, _ := json.Marshal(kindTotals(nil)); string(encoded) != "[]" {
		t.Fatalf("no totals are written as %s", encoded)
	}
}
