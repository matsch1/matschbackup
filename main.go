package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"matschbackup/internal/remote"
	"matschbackup/internal/utils"
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

	// check rclone config
	err := remote.RcloneRemoteAccessible(remoteBase)
	if err != nil {
		log.Error("Failed to execute rclone: ", "error", err)
		return
	}

	// delete oldest backup if necessary and do backup
	backup_to_old, err := utils.LastBackupToOld(remoteBase, daysThreshold)
	if backup_to_old {
		log.Debug("Recent backup to old")

		to_many_backups, err := utils.ToManyBackups(remoteBase, maxBackups)
		if to_many_backups {
			log.Debug("To many backups on remote")
			utils.DeleteOldBackup(remoteBase, dryRun)
		}
		// do backup
		var waitgroup sync.WaitGroup
		sem := make(chan struct{}, concurrency)

		// Create path for new backup
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		remotePath := fmt.Sprintf("%s/bak_%s", remoteBase, timestamp)

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
		_, err = pkg.RunCommand("rclone", "touch", fmt.Sprintf("%s/%s", remotePath, "BACKUP_COMPLETED"))
		if err != nil {
			log.Error("failed to create BACKUP_COMPLETED file")
		}

	} else {
		latest_backup_time, _ := utils.GetLastBackup(remoteBase)
		daysSince := time.Since(latest_backup_time).Hours() / 24
		log.Info("✅ Last backup is recent.", "days since last backup", math.Round(daysSince))

	}

	log.Info("========= Backup done =========")
}
