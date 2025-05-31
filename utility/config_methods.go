package utility

import (
	"log/slog"
	// "github.com/c-m3-codin/gsched/utility" // Self-import not needed if types are in this package
)

// scheduleUpdateCheck checks if the schedule file has been updated.
// This is a method of utility.Config.
func (uc *Config) ScheduleUpdateCheck() { // Renamed to be public if called from another package's struct method
	hash, err := Filemd5sum(uc.ScheduleFile) // Assuming Filemd5sum is also in utility package
	if err != nil {
		slog.Error("Failed to calculate file hash", "error", err, "path", uc.ScheduleFile)
		return
	}
	slog.Debug("Schedule file hash check", "previous_hash", uc.ScheduleFileHash, "current_hash", hash)
	if uc.ScheduleFileHash != hash {
		slog.Info("Schedule file has changed, reloading.", "old_hash", uc.ScheduleFileHash, "new_hash", hash)
		uc.ScheduleFileHash = hash
		uc.ScheduleChangeChannel <- true
	}
}
