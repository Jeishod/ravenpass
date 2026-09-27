package backups

import (
	"encoding/json"
	"slices"

	"github.com/dortanes/ravenpass/packages/app/privatefile"
)

const (
	journalVersion  = 1
	maxJournalBytes = 1 << 20
)

// entry is one backup Ravenpass wrote; Hash is the hex vault head hash and At is Unix seconds.
type entry struct {
	Folder   string `json:"folder"`
	Address  string `json:"address"`
	Name     string `json:"name"`
	Revision uint64 `json:"revision"`
	Hash     string `json:"hash"`
	At       int64  `json:"at"`
}

type journalFile struct {
	Version int                `json:"version"`
	Vaults  map[string][]entry `json:"vaults"`
}

// journal lists the backups Ravenpass wrote, oldest first, by vault; only listed files are ever removed.
type journal struct {
	path   string
	vaults map[string][]entry
}

func openJournal(path string) *journal {
	opened := &journal{path: path, vaults: map[string][]entry{}}
	data, err := privatefile.Read(path, maxJournalBytes)
	if err != nil {
		return opened
	}
	var stored journalFile
	if err := json.Unmarshal(data, &stored); err != nil || stored.Version != journalVersion || stored.Vaults == nil {
		return opened
	}
	opened.vaults = stored.Vaults
	return opened
}

// backups lists the backups of vaultID, oldest first.
func (j *journal) backups(vaultID string) []entry {
	return slices.Clone(j.vaults[vaultID])
}

// latest is the newest backup of vaultID in folder.
func (j *journal) latest(vaultID, folder string) (entry, bool) {
	listed := j.vaults[vaultID]
	for i := len(listed) - 1; i >= 0; i-- {
		if listed[i].Folder == folder {
			return listed[i], true
		}
	}
	return entry{}, false
}

// record lists entries as the backups of vaultID and writes the journal; memory keeps them if the write fails.
func (j *journal) record(vaultID string, entries []entry) error {
	if len(entries) == 0 {
		delete(j.vaults, vaultID)
	} else {
		j.vaults[vaultID] = entries
	}
	data, err := json.Marshal(journalFile{Version: journalVersion, Vaults: j.vaults})
	if err != nil {
		return err
	}
	return privatefile.Write(j.path, data)
}
