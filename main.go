package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
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
	defaultRemoteBase    = "fritznas:/fritz.nas/NAS/Matthias/backups"
	defaultConcurrency   = 2
	defaultBackupMax     = 14
	defaultDaysThreshold = 7
)

var (
	pathsToBackup        []string
	defaultPathsToBackup = []string{
		"/home/" + os.Getenv("USER") + "/Files",
		"/home/" + os.Getenv("USER") + "/src",
		"/home/" + os.Getenv("USER") + "/obsidian",
		"/home/" + os.Getenv("USER") + "/_Ablage",
		"/home/" + os.Getenv("USER") + "/.config",
	}
	maxBackups    int
	remoteBase    string
	daysThreshold int
	dryRun        bool
	compress      bool
	verbose       bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup specific files to Fritz!NAS via rclone",
		Run:   runBackup,
	}

	rootCmd.Flags().StringArrayVarP(&pathsToBackup, "path", "p", defaultPathsToBackup, "Paths to back up")
	rootCmd.Flags().IntVar(&maxBackups, "max-backups", defaultBackupMax, "Maximum number of backups to keep")
	rootCmd.Flags().StringVar(&remoteBase, "remote", defaultRemoteBase, "Remote rclone target path")
	rootCmd.Flags().IntVar(&daysThreshold, "days", defaultDaysThreshold, "Minimum age in days before making new backup")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only show what would be done")
	rootCmd.Flags().BoolVarP(&compress, "zip", "c", false, "Compress directories before copying to remote")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	if err := rootCmd.Execute(); err != nil {
		log.Error("Error rootCmd: ", "error", err)
	}
}

func runCommand(name string, args ...string) (string, error) {
	if verbose {
		log.SetLevel(log.DebugLevel)
		log.Debug("Execute:", "cmd", name+" "+strings.Join(args, " "))
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error: %v\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func runBackup(cmd *cobra.Command, args []string) {
	log.Info("========= Start backup =========")

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	remotePath := fmt.Sprintf("%s/bak_%s", remoteBase, timestamp)

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
		sem := make(chan struct{}, defaultConcurrency)

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
	} else {
		log.Info("✅ Last backup is recent.")
	}

	log.Info("========= Backup done =========")
}

func listRemoteBackups() ([]string, error) {
	out, err := runCommand("rclone", "lsf", remoteBase, "--dirs-only")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}

func zipDirectory(sourceDir, zipPath string) error {
	log.Debug("Zipping directory", "source", sourceDir, "output", zipPath)
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	count := 0
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn("Error reading path", "path", path, "error", err)
			return nil
		}

		// Skip symlinks and dirs
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
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
			return err
		}
		_, err = io.Copy(writerEntry, file)
		return err
	})

	log.Debug("Zipping done", "dir", sourceDir, "total files", count)
	return err
}

func purgeRemoteDir(dir string) error {
	fullPath := fmt.Sprintf("%s/%s", remoteBase, dir)
	log.Info("Purging remote dir", "dir", fullPath)
	_, err := runCommand("rclone", "purge", fullPath)
	return err
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

	} else {
		log.Warn("dryRun: rclone copy not done", "remotePath", remotePath)
	}
	return nil
}
