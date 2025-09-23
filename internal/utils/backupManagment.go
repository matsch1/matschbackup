package utils

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"matschbackup/internal/remote"
)

func GetListOfBackupNames(remoteBase string) ([]string, error) {
	old_backups, err := remote.ListRemoteBackups(remoteBase)
	if err != nil {
		return []string{}, fmt.Errorf("Failed to list remote backups: %s", err)
	}

	for i := range old_backups {
		old_backups[i] = strings.TrimSuffix(old_backups[i], "/")
	}
	sort.Strings(old_backups)
	return old_backups, nil
}

func DeleteOldBackup(remoteBase string, dryRun bool) error {
	old_backups, _ := GetListOfBackupNames(remoteBase)
	if !dryRun {
		backup_to_delete := 0 // oldest backup
		number_of_valid_backups, _ := remote.GetNumberOfValidBackups(remoteBase)
		if number_of_valid_backups <= 1 {
			log.Warn("Numver of completed backups <= 1")
			backup_to_delete_is_valid, _ := remote.BackupIsValid(old_backups[backup_to_delete])
			if backup_to_delete_is_valid {
				backup_to_delete = backup_to_delete + 1
			}
		}
		if err := remote.PurgeRemoteDir(remoteBase, old_backups[backup_to_delete]); err != nil {
			log.Error("Failed to delete old backup: ", "error", err)
		}
	} else {
		log.Warn("dryRun! Backup not deleted", "dir", old_backups[0])
	}

	return nil
}

func GetLastBackup(remoteBase string) (time.Time, error) {
	// get list of backup names
	old_backups, err := GetListOfBackupNames(remoteBase)
	if err != nil {
		return time.Time{}, fmt.Errorf("Failed to list remote backups: %s ", err)
	}

	// find days since last backup
	latest_backup := old_backups[len(old_backups)-1]
	timeString := strings.TrimPrefix(latest_backup, "bak_")
	latest_backup_time, err := ConvertTimeStringToTime(timeString)
	if err != nil {
		return time.Time{}, fmt.Errorf("Failed to convert backup time: %s ", err)
	}

	return latest_backup_time, nil
}
func LastBackupToOld(remoteBase string, daysThreshold int) (bool, error) {
	latest_backup_time, _ := GetLastBackup(remoteBase)

	if time.Since(latest_backup_time) > time.Duration(daysThreshold)*24*time.Hour {
		return true, nil
	} else {
		return false, nil
	}
}

func ToManyBackups(remoteBase string, maxBackups int) (bool, error) {
	old_backups, err := GetListOfBackupNames(remoteBase)
	if err != nil {
		return false, fmt.Errorf("error getting list of old backups: %s", err)
	}
	if len(old_backups) >= maxBackups {
		return true, nil
	}
	return false, nil
}
