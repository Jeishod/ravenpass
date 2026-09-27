package preferences

import (
	"errors"
	"slices"
	"time"
)

// BackupInterval is how often an open vault that changed is backed up.
type BackupInterval string

// Backup intervals this build offers.
const (
	BackupDaily   BackupInterval = "daily"
	BackupWeekly  BackupInterval = "weekly"
	BackupMonthly BackupInterval = "monthly"
)

const defaultBackupKeep = 5

var backupKeeps = []int{1, 3, 5, 10, 30}

// Errors a backup choice is refused with.
var (
	ErrUnsupportedBackupChoice = errors.New("unsupported backup interval or count")
	ErrBackupFolderMissing     = errors.New("no backup folder is chosen")
)

// BackupIntervals lists the intervals this build offers, in the order the interface shows them.
func BackupIntervals() []BackupInterval {
	return []BackupInterval{BackupDaily, BackupWeekly, BackupMonthly}
}

// Every is how long a backup stays the latest before the next one is due. A month is 30 days.
func (i BackupInterval) Every() time.Duration {
	switch i {
	case BackupWeekly:
		return 7 * 24 * time.Hour
	case BackupMonthly:
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// BackupKeeps lists how many backups of a vault the owner may keep in the folder, fewest first.
func BackupKeeps() []int { return slices.Clone(backupKeeps) }

func offeredBackupInterval(interval BackupInterval) bool {
	return slices.Contains(BackupIntervals(), interval)
}

func offeredBackupKeep(keep int) bool { return slices.Contains(backupKeeps, keep) }

// BackupFolder is where backups go: a path on macOS, a document tree URI on Android.
type BackupFolder struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Place   string `json:"place"`
}

// AutoBackup is how an open vault is backed up on its own; a zero Folder means backups are off.
type AutoBackup struct {
	Enabled  bool
	Interval BackupInterval
	Keep     int
	Folder   BackupFolder
}

// AutoBackup reports the recorded choices, or off, daily and keeping 5 where none was recorded.
func (s *Store) AutoBackup() AutoBackup {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	backup := AutoBackup{Enabled: s.current.AutoBackupOn, Interval: BackupDaily, Keep: defaultBackupKeep}
	if s.current.AutoBackupInterval != "" {
		backup.Interval = s.current.AutoBackupInterval
	}
	if s.current.AutoBackupKeep != 0 {
		backup.Keep = s.current.AutoBackupKeep
	}
	if s.current.AutoBackupFolder != nil {
		backup.Folder = *s.current.AutoBackupFolder
	}
	return backup
}

// SetAutoBackup records whether backups run, how often and how many to keep; Enabled needs a folder.
func (s *Store) SetAutoBackup(enabled bool, interval BackupInterval, keep int) error {
	if !offeredBackupInterval(interval) || !offeredBackupKeep(keep) {
		return ErrUnsupportedBackupChoice
	}
	return s.apply(func(next *record) error {
		if enabled && next.AutoBackupFolder == nil {
			return ErrBackupFolderMissing
		}
		next.AutoBackupOn = enabled
		next.AutoBackupInterval = interval
		next.AutoBackupKeep = keep
		return nil
	})
}

// SetBackupFolder records the folder backups go to, leaving Enabled as it was.
func (s *Store) SetBackupFolder(folder BackupFolder) error {
	if folder.Address == "" {
		return ErrBackupFolderMissing
	}
	return s.update(func(next *record) { next.AutoBackupFolder = &folder })
}

// keepOfferedAutoBackup forgets unoffered or incomplete backup choices.
func keepOfferedAutoBackup(stored *record) {
	if !offeredBackupInterval(stored.AutoBackupInterval) {
		stored.AutoBackupInterval = ""
	}
	if !offeredBackupKeep(stored.AutoBackupKeep) {
		stored.AutoBackupKeep = 0
	}
	if stored.AutoBackupFolder != nil && stored.AutoBackupFolder.Address == "" {
		stored.AutoBackupFolder = nil
	}
	if stored.AutoBackupFolder == nil {
		stored.AutoBackupOn = false
	}
}
