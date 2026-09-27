package importers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// CSV is export text of comma-separated records led by a header row naming their columns.
type CSV []byte

// Recognised reports whether the text is UTF-8 with a header naming every required column.
func (t CSV) Recognised(required ...string) bool {
	if !utf8.Valid(t) {
		return false
	}
	header, err := t.records().Read()
	if err != nil {
		return false
	}
	named := columnsOf(header)
	for _, column := range required {
		if _, found := named[column]; !found {
			return false
		}
	}
	return true
}

// Rows calls visit with each record after the header; a record that does not parse is ErrUnrecognized.
func (t CSV) Rows(visit func(Row)) error {
	records := t.records()
	header, err := records.Read()
	if err != nil {
		return Unrecognized(err)
	}
	named := columnsOf(header)
	for {
		record, err := records.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return Unrecognized(err)
		}
		visit(Row{columns: named, record: record})
	}
}

// Wipe zeroes the text's bytes in place, including any the caller still shares.
func (t CSV) Wipe() {
	clear(t)
}

func (t CSV) records() *csv.Reader {
	reader := csv.NewReader(bytes.NewReader(t))
	reader.FieldsPerRecord = -1
	return reader
}

// Row is one CSV record read by the column names of its header.
type Row struct {
	columns columns
	record  []string
}

// Cell is the value of column in the row, empty when the header does not name it or the row ends before it.
func (r Row) Cell(column string) string {
	position, found := r.columns[column]
	if !found || position >= len(r.record) {
		return ""
	}
	return r.record[position]
}

// columns is the position of each trimmed header name; the first of a repeated name counts.
type columns map[string]int

func columnsOf(header []string) columns {
	positions := make(columns, len(header))
	for i, name := range header {
		name = strings.TrimSpace(name)
		if _, repeated := positions[name]; !repeated {
			positions[name] = i
		}
	}
	return positions
}
