package importers

import "testing"

func TestExtrasWriteLabelledLinesAfterTheNotes(t *testing.T) {
	tests := []struct {
		name  string
		notes string
		lines [][2]string
		want  string
	}{
		{"one line", "", [][2]string{{"Website", "https://example.test"}}, "Website: https://example.test"},
		{"a value over lines", "", [][2]string{{"Private key", "line one\nline two"}}, "Private key:\nline one\nline two"},
		{"a value with no label", "", [][2]string{{" ", "loose value"}}, "loose value"},
		{"a blank value", "Own", [][2]string{{"PIN", " \n "}}, "Own"},
		{"after the notes", "Own notes", [][2]string{{"A", "1"}, {"B", "2"}}, "Own notes\n\nA: 1\nB: 2"},
		{"after notes ending in line breaks", "Own notes\r\n\n", [][2]string{{"A", "1"}}, "Own notes\n\nA: 1"},
		{"after blank notes", "  \n", [][2]string{{"A", "1"}}, "A: 1"},
		{"nothing to add", "Own notes\n", nil, "Own notes\n"},
		{"a value kept as written", "", [][2]string{{"Field", "  spaced  "}}, "Field:   spaced  "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var extras Extras
			for _, line := range test.lines {
				extras.Add(line[0], line[1])
			}
			if got := extras.Notes(test.notes); got != test.want {
				t.Fatalf("Notes = %q, want %q", got, test.want)
			}
		})
	}
}
