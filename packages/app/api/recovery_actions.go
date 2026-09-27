package api

import (
	"bytes"
	"errors"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
)

// recoverySheet is the printed recovery key page. It loads nothing and runs no script; its words wrap into two
// columns on paper narrower than 120 mm.
var recoverySheet = template.Must(template.New("recovery-sheet").Parse(`<!DOCTYPE html>
<html lang="{{.Language}}">
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<style>
@page { margin: 16mm; }
html { background: #fff; color: #000; }
body { margin: 0; font: 11pt -apple-system, "Helvetica Neue", Roboto, Arial, sans-serif; }
h1 { margin: 0 0 4pt; font-size: 20pt; }
p { margin: 0 0 4pt; }
.printed { margin-bottom: 18pt; }
ol { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8pt 12pt; margin: 0 0 18pt; padding: 0; list-style: none; }
li { padding: 6pt 8pt; border: 0.75pt solid #000; border-radius: 4pt; font: 15pt ui-monospace, Menlo, "Roboto Mono", monospace; break-inside: avoid; }
li span { display: inline-block; min-width: 2ch; margin-right: 1ch; text-align: right; color: #444; }
@media (max-width: 120mm) { ol { grid-template-columns: repeat(2, 1fr); } }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
<p class="printed">{{.Printed}}</p>
<ol>{{range .Words}}<li><span>{{.Number}}</span>{{.Word}}</li>{{end}}</ol>
<p>{{.Keep}}</p>
<p>{{.Warning}}</p>
</body>
</html>
`))

// sheetCapacity holds a rendered page in one allocation, so the buffer cleared after printing is the only one it grew.
const sheetCapacity = 8 << 10

type numberedWord struct {
	Number int
	Word   string
}

type sheetPage struct {
	preferences.RecoverySheetText
	Words []numberedWord
}

// SaveRecoveryKey saves the staged recovery key, of a vault being created or a new phrase, as a text file, only while
// phrase still matches it.
func (s *Service) SaveRecoveryKey(phrase string) error {
	if err := s.vault.VerifyStagedPhrase(phrase); err != nil {
		return presentStagedPhraseError(err)
	}
	dialogs := s.preferences.Dialogs()
	file := SavedFile{
		Prompt: dialogs.SaveRecoveryKey, Name: dialogs.RecoveryFileName, MediaType: mediaText,
		Filter: dialogs.TextFilter, Pattern: "*.txt",
	}
	text := s.preferences.RecoveryFile()
	saved, err := s.saveFile(file, func(target SaveTarget) error {
		var writeErr error
		err := s.vault.WithVerifiedStagedPhrase(phrase, func() error {
			writeErr = writeRecoveryKey(target, phrase, text)
			return writeErr
		})
		if writeErr != nil {
			return presentRecoveryFileError(writeErr)
		}
		if err != nil {
			return presentStagedPhraseError(err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !saved {
		return fail(failureRecoveryKeyNotSaved)
	}
	return nil
}

// CopyRecoveryKey copies the staged recovery key while phrase still matches it, as SaveRecoveryKey does.
func (s *Service) CopyRecoveryKey(phrase string) error {
	var copyErr error
	err := s.vault.WithVerifiedStagedPhrase(phrase, func() error {
		copyErr = s.copyRecoveryKey(phrase, clearAfter)
		return copyErr
	})
	if copyErr != nil {
		return copyErr
	}
	if err != nil {
		return presentStagedPhraseError(err)
	}
	return nil
}

// PrintRecoveryKey shows the system print dialog for the staged recovery key once phrase matches it, as
// SaveRecoveryKey does. The page is built in memory and handed only to the host's printer; its buffer is cleared once
// Print returns, while the host's print view and the system spooler keep their own copies until the job ends.
func (s *Service) PrintRecoveryKey(phrase string) error {
	if err := s.vault.VerifyStagedPhrase(phrase); err != nil {
		return presentStagedPhraseError(err)
	}
	text := s.preferences.RecoverySheet()
	page, err := renderRecoverySheet(phrase, text, time.Now())
	if err != nil {
		return fail(failureRecoveryKeyPrintFailed)
	}
	defer clear(page)
	defer s.hold()()
	if err := s.printer.Print(text.Title, page); err != nil {
		return fail(failureRecoveryKeyPrintFailed)
	}
	return nil
}

func renderRecoverySheet(phrase string, text preferences.RecoverySheetText, day time.Time) ([]byte, error) {
	words := strings.Fields(phrase)
	numbered := make([]numberedWord, len(words))
	for i, word := range words {
		numbered[i] = numberedWord{Number: i + 1, Word: word}
	}
	text.Printed = strings.ReplaceAll(text.Printed, "{date}", day.Format(time.DateOnly))
	page := bytes.NewBuffer(make([]byte, 0, sheetCapacity))
	if err := recoverySheet.Execute(page, sheetPage{RecoverySheetText: text, Words: numbered}); err != nil {
		clear(page.Bytes())
		return nil, err
	}
	return page.Bytes(), nil
}

func (s *Service) copyRecoveryKey(phrase string, schedule func(time.Duration, func())) error {
	if !s.copyToClipboard(phrase, schedule) {
		return fail(failureRecoveryKeyCopyFailed)
	}
	return nil
}

func writeRecoveryKey(target SaveTarget, phrase string, file preferences.RecoveryFileText) error {
	contents := make([]byte, 0, len(file.Intro)+len(phrase)+len(file.Notice))
	contents = append(contents, file.Intro...)
	contents = append(contents, phrase...)
	contents = append(contents, file.Notice...)
	defer clear(contents)
	return target.Write(contents)
}

func presentStagedPhraseError(err error) error {
	if errors.Is(err, vaultservice.ErrNoPendingSetup) {
		return fail(failureSetupNotActive)
	}
	return fail(failureRecoveryKeyMismatch)
}

func presentRecoveryFileError(err error) error {
	switch {
	case errors.Is(err, os.ErrPermission):
		return fail(failurePermissionDenied)
	case errors.Is(err, ErrFileUnverified):
		return fail(failureFileUnverified)
	default:
		return fail(failureRecoveryKeySaveFailed)
	}
}
