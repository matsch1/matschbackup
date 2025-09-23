package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"matschbackup/internal/remote"
	"matschbackup/pkg"
)

const (
	defaultBackupMax     = 14
	defaultDaysThreshold = 7
	defaultConcurrency   = 2 // max number of parallel backups
)

var (
	pathsToBackup []string
	maxBackups    int
	remoteBase    string
	daysThreshold int
	concurrency   int
	dryRun        bool
	compress      bool
	debug         bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup specific files to Fritz!NAS via rclone",
		Run:   runBackup,
	}

	rootCmd.Flags().StringSliceVar(&pathsToBackup, "path", nil, "Paths to back up")
	rootCmd.MarkFlagRequired("pathsToBackup")
	rootCmd.Flags().StringVar(&remoteBase, "remote", "", "Remote rclone target path")
	rootCmd.MarkFlagRequired("remote")
	rootCmd.Flags().IntVar(&maxBackups, "max-backups", defaultBackupMax, "Maximum number of backups to keep")
	rootCmd.Flags().IntVar(&daysThreshold, "max-days", defaultDaysThreshold, "Minimum age in days before making new backup")
	rootCmd.Flags().IntVar(&concurrency, "concurrency", defaultConcurrency, "Max. number of parralel backup directories")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only show what would be done")
	rootCmd.Flags().BoolVarP(&compress, "zip", "z", false, "Compress directories before copying to remote")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug output")

	if err := rootCmd.Execute(); err != nil {
		log.Error("Error rootCmd: ", "error", err)
	}
}

func runBackup(cmd *cobra.Command, args []string) {
	log.Info("========= Start backup =========")
	if debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug Mode")
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	remotePath := fmt.Sprintf("%s/bak_%s", remoteBase, timestamp)

	// check rclone config
	err := remote.CheckRcloneRemote(remoteBase)
	if err != nil {
		log.Error("Failed to execute rclone: ", "error", err)
		return
	}

	// get old backup list
	old_backups, err := remote.ListRemoteBackups(remoteBase)
	if err != nil {
		log.Error("Failed to list remote backups: ", "error", err)
		return
	}

	for i := range old_backups {
		old_backups[i] = strings.TrimSuffix(old_backups[i], "/")
	}
	sort.Strings(old_backups)

	// find days since last backup
	latest_backup := old_backups[len(old_backups)-1]
	tsStr := strings.TrimPrefix(latest_backup, "bak_")
	latest_backup_time, err := time.Parse("2006-01-02_15-04-05", tsStr)
	if err != nil {
		log.Error("Failed to parse timestamp of latest backup: ", "error", err)
	}

	// delete oldest backup if necessary and do backup
	if time.Since(latest_backup_time) > time.Duration(daysThreshold)*24*time.Hour {
		if len(old_backups) >= maxBackups {
			log.Info("Deleting oldest backup to maintain limit", "dir", old_backups[0])
			if !dryRun {
				backup_to_delete := 0 // oldest backup
				number_of_valid_backups, _ := remote.GetNumberOfValidBackups(remotePath)
				if number_of_valid_backups <= 1 {
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
		}
		// do backup
		var waitgroup sync.WaitGroup
		sem := make(chan struct{}, concurrency)

		for _, path := range pathsToBackup {
			waitgroup.Add(1)
			go func(path string) {
				defer waitgroup.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				log.Info("Backing up:", "path", path)
				err := remote.CopyToRemote(path, fmt.Sprintf("%s/%s", remotePath, path), compress, dryRun)
				if err != nil {
					log.Error("Backup failed", "path", path, "error", err)
					return
				}
				log.Info("✅ Backup done", "path", path)

			}(path)
		}
		waitgroup.Wait()

		// Add file to verify backup as completed
		_, err := pkg.RunCommand("rclone", "touch", fmt.Sprintf("%s/%s", remotePath, "BACKUP_COMPLETED"))
		if err != nil {
			log.Error("failed to create BACKUP_COMPLETED file")
		}

	} else {
		daysSince := time.Since(latest_backup_time).Hours() / 24
		log.Info("✅ Last backup is recent.", "days since last backup", math.Round(daysSince))

	}

	log.Info("========= Backup done =========")
}
