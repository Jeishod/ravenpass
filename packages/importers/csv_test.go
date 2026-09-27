package importers

import (
	"bytes"
	"errors"
	"testing"
)

func TestRowCellsAreFoundByHeaderName(t *testing.T) {
	text := CSV(" ServiceName ,Notes,ServiceName,CurrentPassword\nExample,Own notes,Repeated\n")
	var rows []Row
	if err := text.Rows(func(row Row) { rows = append(rows, row) }); err != nil || len(rows) != 1 {
		t.Fatalf("rows = %d, error = %v", len(rows), err)
	}
	tests := map[string]string{
		"ServiceName":     "Example",
		"Notes":           "Own notes",
		"CurrentPassword": "",
		"CardNumber":      "",
	}
	for column, want := range tests {
		if got := rows[0].Cell(column); got != want {
			t.Errorf("Cell(%q) = %q, want %q", column, got, want)
		}
	}
}

func TestCSVIsRecognisedByTheColumnsItsHeaderNames(t *testing.T) {
	tests := []struct {
		name string
		text CSV
		want bool
	}{
		{"every required column", CSV("name,type,notes\n"), true},
		{"a missing column", CSV("name,notes\n"), false},
		{"text that is not UTF-8", CSV("name,type\n\xff\n"), false},
		{"no header", CSV(""), false},
	}
	for _, test := range tests {
		if got := test.text.Recognised("type", "name"); got != test.want {
			t.Errorf("%s: Recognised = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestARecordThatDoesNotParseIsUnrecognized(t *testing.T) {
	text := CSV("name,type\n\"open quote,login\n")
	if err := text.Rows(func(Row) {}); !errors.Is(err, ErrUnrecognized) {
		t.Fatalf("Rows = %v, want ErrUnrecognized", err)
	}
	if err := CSV("").Rows(func(Row) {}); !errors.Is(err, ErrUnrecognized) {
		t.Fatalf("Rows without a header = %v, want ErrUnrecognized", err)
	}
}

func TestWipeClearsTheText(t *testing.T) {
	text := CSV("name,type\nExample,login\n")
	text.Wipe()
	if !bytes.Equal(text, make([]byte, len(text))) {
		t.Fatal("the text survives Wipe")
	}
}
