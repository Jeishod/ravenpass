package clouddrive

import "testing"

func TestAFolderACloudDriveSyncsIsPlacedByTheDriveAndItsFolderThere(t *testing.T) {
	const library = "/Users/me/Library"
	for folder, want := range map[string]string{
		library + "/CloudStorage/GoogleDrive-me@example.com":                    "Google Drive",
		library + "/CloudStorage/GoogleDrive-me@example.com/My Drive":           "Google Drive › My Drive",
		library + "/CloudStorage/GoogleDrive-me@example.com/My Drive/Ravenpass": "Google Drive › Ravenpass",
		library + "/CloudStorage/Dropbox":                                       "Dropbox",
		library + "/CloudStorage/OneDrive-Personal/Vaults":                      "OneDrive › Vaults",
		library + "/CloudStorage/Box-Box":                                       "Box",
		library + "/CloudStorage/pCloud-me/Vaults":                              "pCloud › Vaults",
		library + "/Mobile Documents/com~apple~CloudDocs":                       "iCloud Drive",
		library + "/Mobile Documents/com~apple~CloudDocs/Vaults":                "iCloud Drive › Vaults",
	} {
		if got, named := (Places{}).Place(folder); !named || got != want {
			t.Errorf("Place(%q) = %q, %v, want %q", folder, got, named, want)
		}
	}
}

func TestAFolderOutsideEveryCloudDriveIsNotNamed(t *testing.T) {
	for _, folder := range []string{
		"/",
		"/Users/me/Documents",
		"/Users/me/Library",
		"/Users/me/Library/CloudStorage",
		"/Users/me/Library/CloudStorage/-me",
		"/Users/me/Library/Mobile Documents/iCloud~com~example~app/Documents",
		"/Volumes/Backup/Users/me/Library/CloudStorage/Dropbox",
	} {
		if got, named := (Places{}).Place(folder); named {
			t.Errorf("Place(%q) = %q, want no name", folder, got)
		}
	}
}
