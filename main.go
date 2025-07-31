package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
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
	err := checkRcloneRemote(remoteBase)
	if err != nil {
		log.Error("Failed to execute rclone: ", "error", err)
		return
	}

	// get old backup list
	old_backups, err := listRemoteBackups()
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
				if err := purgeRemoteDir(old_backups[0]); err != nil {
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
				err := copyToRemote(path, fmt.Sprintf("%s/%s", remotePath, path))
				if err != nil {
					log.Error("Backup failed", "path", path, "error", err)
					return
				}
				log.Info("✅ Backup done", "path", path)

			}(path)
		}
		waitgroup.Wait()

		// Add file to verify backup as completed
		_, err := runCommand("rclone", "touch", fmt.Sprintf("%s/%s", remotePath, "BACKUP_COMPLETED"))
		if err != nil {
			log.Error("failed to create BACKUP_COMPLETED file")
		}

	} else {
		daysSince := time.Since(latest_backup_time).Hours() / 24
		log.Info("✅ Last backup is recent.", "days since last backup", math.Round(daysSince))

	}

	log.Info("========= Backup done =========")
}

func checkRcloneRemote(remoteBase string) error {
	// check if rclone is installed
	if _, err := exec.LookPath("rclone"); err != nil {
		return fmt.Errorf("rclone not found in path: %w", err)
	}
	log.Debug("rclone installed")

	// Check if the remoteBase is accessible
	cmd := exec.Command("rclone", "lsd", remoteBase)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to access remote %q: %v\n%s", remoteBase, err, stderr.String())
	}
	log.Debug("remote access ok", "remote", remoteBase)

	return nil
}

func listRemoteBackups() ([]string, error) {
	backup_list, err := runCommand("rclone", "lsf", remoteBase, "--dirs-only")
	if err != nil {
		return nil, fmt.Errorf("error running rclone lsf %s", remoteBase)
	}

	old_backups := []string{}
	if strings.TrimSpace(backup_list) == "" {
		log.Warn("No old backups found")
	} else {
		old_backups = strings.Split(strings.TrimSpace(backup_list), "\n")
		log.Debug("Backups found on remote", "number", len(old_backups))
	}

	return old_backups, nil
}

func zipDirectory(sourceDir, zipPath string) error {
	log.Debug("Zipping directory", "source", sourceDir, "output", zipPath)
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("error creating zipFile %s", zipPath)
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	count := 0
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn("Error reading path. Skipping", "path", path, "error", err)
			return nil
		}

		// Skip symlinks and dirs
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("error creating relative path: %s", relPath)
		}

		if count%100 == 0 {
			log.Debug("Zipping progress", "files", count, "dir", sourceDir, "current", relPath)
		}
		count++

		file, err := os.Open(path)
		if err != nil {
			log.Warn("Skipping unreadable file", "path", path, "error", err)
			return nil
		}
		defer file.Close()

		writerEntry, err := writer.Create(relPath)
		if err != nil {
			return fmt.Errorf("error creating file %s", relPath)
		}
		_, err = io.Copy(writerEntry, file)
		return err
	})

	log.Debug("Zipping done", "dir", sourceDir, "total files", count)
	return fmt.Errorf("error filepath.Walk %s", err)
}

func purgeRemoteDir(dir string) error {
	fullPath := fmt.Sprintf("%s/%s", remoteBase, dir)
	log.Info("Purging remote dir", "dir", fullPath)
	_, err := runCommand("rclone", "purge", fullPath)
	return fmt.Errorf("error purging remote directory %s", err)
}

func copyToRemote(localPath, remotePath string) error {
	baseName := strings.ReplaceAll(strings.TrimPrefix(localPath, "/"), "/", "_")
	sourceName := ""
	if compress {
		sourceName = fmt.Sprintf("/tmp/%s.zip", baseName)
		if !dryRun {
			err := zipDirectory(localPath, sourceName)
			if err != nil {
				return fmt.Errorf("failed to zip %s: %w", localPath, err)
			}
			defer os.Remove(sourceName)
		} else {
			log.Warn("dryRun: directory not zipped", "localPath", localPath)
		}
	} else {
		sourceName = localPath
	}

	if !dryRun {
		_, err := runCommand("rclone", "copy", sourceName, remotePath, "--progress", "--transfers=1", "--checkers=4", "--fast-list")
		if err != nil {
			return fmt.Errorf("failed to copy to remote %s: %w", remotePath, err)
		}

		// Add file to verify backup as completed
		_, err = runCommand("rclone", "touch", fmt.Sprintf("%s/%s", remotePath, "BACKUP_COMPLETED"))
		if err != nil {
			return fmt.Errorf("failed to create BACKUP_COMPLETED file")
		}

	} else {
		log.Warn("dryRun: rclone copy not done", "remotePath", remotePath)
	}
	return nil
}

func runCommand(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Debug("Execute:", "cmd", name+" "+strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error: %v\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
