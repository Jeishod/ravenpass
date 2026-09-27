package importers

import "strings"

// Extras collects the values an item carries that Ravenpass has no field for, as labelled lines for the item's notes.
type Extras struct {
	lines []string
}

// Add writes value as "Label: value", or under "Label:" when it spans lines; an unlabelled value is written alone.
func (e *Extras) Add(label, value string) {
	switch {
	case strings.TrimSpace(value) == "":
	case strings.TrimSpace(label) == "":
		e.lines = append(e.lines, value)
	case strings.ContainsAny(value, "\r\n"):
		e.lines = append(e.lines, label+":\n"+value)
	default:
		e.lines = append(e.lines, label+": "+value)
	}
}

// Notes returns the item's own notes, then one blank line, then the collected lines.
func (e *Extras) Notes(notes string) string {
	if len(e.lines) == 0 {
		return notes
	}
	lines := strings.Join(e.lines, "\n")
	if strings.TrimSpace(notes) == "" {
		return lines
	}
	return strings.TrimRight(notes, "\r\n") + "\n\n" + lines
}
