// Package backupexclusion keeps a file out of the system's backups.
package backupexclusion

// System marks files through the operating system.
type System struct{}

// Exclude marks the file at path to be left out of backups; the mark survives a rename, not a replacement.
func (System) Exclude(path string) error { return exclude(path) }
