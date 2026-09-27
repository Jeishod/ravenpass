// Package clouddrive names the macOS cloud drive that syncs a folder.
package clouddrive

import (
	"path/filepath"
	"strings"
)

// rootDepth is the folder count of /Users/<account>/Library/<drives folder>/<drive>.
const rootDepth = 5

// renamed maps CloudStorage folder prefixes, the part before "-<account>", to differing drive names.
var renamed = map[string]string{"GoogleDrive": "Google Drive"}

// Places names a folder by its cloud drive, adding the folder's own name below the drive's root.
type Places struct{}

// Place names folder by the cloud drive that syncs it, false for a folder outside every drive.
func (Places) Place(folder string) (string, bool) {
	folders := strings.FieldsFunc(folder, func(r rune) bool { return r == filepath.Separator })
	drive, synced := driveAt(folders)
	if !synced {
		return "", false
	}
	if len(folders) == rootDepth {
		return drive, true
	}
	return drive + " › " + folders[len(folders)-1], true
}

// driveAt names the cloud drive whose root folders starts with, false for folders outside one.
func driveAt(folders []string) (string, bool) {
	if len(folders) < rootDepth || folders[0] != "Users" || folders[2] != "Library" {
		return "", false
	}
	root := folders[4]
	switch folders[3] {
	case "Mobile Documents":
		if root == "com~apple~CloudDocs" {
			return "iCloud Drive", true
		}
	case "CloudStorage":
		provider, _, _ := strings.Cut(root, "-")
		if name, known := renamed[provider]; known {
			return name, true
		}
		if provider != "" {
			return provider, true
		}
	}
	return "", false
}
