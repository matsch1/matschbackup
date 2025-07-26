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
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	if err := rootCmd.Execute(); err != nil {
		log.Error("Error rootCmd: ", "error", err)
	}
}

func runBackup(cmd *cobra.Command, args []string) {
	log.Info("========= Start backup =========")

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	remotePath := fmt.Sprintf("%s/bak_%s", remoteBase, timestamp)

	dirs, err := listRemoteDirs()
	if err != nil {
		log.Error("Failed to list remote dirs: ", "error", err)
		return
	}

	for i := range dirs {
		dirs[i] = strings.TrimSuffix(dirs[i], "/")
	}
	sort.Strings(dirs)

	if len(dirs) >= maxBackups && !dryRun {
		toDelete := dirs[0]
		log.Info("Deleting oldest backup to maintain limit", "dir", toDelete)
		if err := purgeRemoteDir(toDelete); err != nil {
			log.Error("Failed to delete old backup: ", "error", err)
		}
	}

	doBackup := false

	if len(dirs) == 0 {
		log.Info("No old backup found.")
		doBackup = true
	} else {
		latest := dirs[len(dirs)-1]
		tsStr := strings.TrimPrefix(latest, "bak_")
		latestTime, err := time.Parse("2006-01-02_15-04-05", tsStr)
		if err != nil {
			log.Error("Failed to parse timestamp: ", "error", err)
		}

		age := time.Since(latestTime)
		if age > time.Duration(daysThreshold)*24*time.Hour {
			doBackup = true
		} else {
			log.Info("✅ Last backup is recent.")
		}
	}

	if doBackup {
		var wg sync.WaitGroup
		sem := make(chan struct{}, defaultConcurrency) // limit concurrency

		for _, path := range pathsToBackup {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				log.Info("Backing up:", "path", path)
				if !dryRun {
					err := copyToRemote(path, fmt.Sprintf("%s/%s", remotePath, path))
					if err != nil {
						log.Error("Backup failed", "path", path, "error", err)
						return
					}
					log.Info("✅ Backup done", "path", path)
				}
			}(path)
		}
		wg.Wait()
	}

	log.Info("========= Backup done =========")
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

func listRemoteDirs() ([]string, error) {
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
	return exec.Command("rclone", "purge", fullPath).Run()
}

func copyToRemote(localPath, remotePath string) error {
	baseName := strings.ReplaceAll(strings.TrimPrefix(localPath, "/"), "/", "_")
	zipName := fmt.Sprintf("/tmp/%s.zip", baseName)

	err := zipDirectory(localPath, zipName)
	if err != nil {
		return fmt.Errorf("failed to zip %s: %w", localPath, err)
	}
	defer os.Remove(zipName)

	remoteZipPath := remotePath + ".zip"
	cmd := exec.Command("rclone", "copy", zipName, remoteZipPath, "--progress", "--transfers=1", "--checkers=4", "--fast-list")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
