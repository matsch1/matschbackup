package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

const (
	defaultRemoteBase    = "fritznas:/fritz.nas/NAS/Matthias/backups"
	defaultBackupMax     = 14
	defaultDaysThreshold = 2
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
	}

	// Clean and sort
	for i := range dirs {
		dirs[i] = strings.TrimSuffix(dirs[i], "/")
	}
	sort.Strings(dirs)

	if len(dirs) > maxBackups && !dryRun {
		log.Info("Max number of backups reached ", "n", len(dirs))
		oldest := dirs[0]
		log.Info("Deleting oldest backup: ", "path", oldest)
		if err := purgeRemoteDir(oldest); err != nil {
			log.Error("Failed to delete old backup: ", "error", err)
		}
	}

	// Check if last backup is recent
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
		for _, path := range pathsToBackup {
			log.Info("Backing up:", "path", path)
			if !dryRun {
				if err := copyToRemote(path, fmt.Sprintf("%s/%s", remotePath, path)); err != nil {
					log.Error("Backup failed: ", "error", err)
				}
			}
			log.Info("✅ Backup done successfully: ", "path", remotePath)
		}
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
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}

func zipDirectory(sourceDir, zipFile string) error {
	_, err := runCommand("zip", "-r", zipFile, ".", "-x", "*.cache", "*.tmp") // Add excludes if needed
	if err != nil {
		return err
	}

	// Change directory so that archive doesn’t contain full paths
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd %s && zip -r %s .", sourceDir, zipFile))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func purgeRemoteDir(dir string) error {
	fullPath := fmt.Sprintf("%s/%s", remoteBase, dir)
	cmd := "rclone"
	args := []string{"purge", fullPath}

	log.Debug("Execute: ", "cmd", "args", args)
	return exec.Command(cmd, args...).Run()
}

func copyToRemote(localPath, remotePath string) error {
	// Create ZIP file in /tmp
	baseName := strings.ReplaceAll(strings.TrimPrefix(localPath, "/"), "/", "_")
	zipName := fmt.Sprintf("/tmp/%s.zip", baseName)

	// Zip the localPath into zipName
	err := zipDirectory(localPath, zipName)
	if err != nil {
		return fmt.Errorf("failed to zip %s: %w", localPath, err)
	}
	defer os.Remove(zipName) // Clean up after upload

	// Define remote destination (add .zip extension)
	remoteZipPath := remotePath + ".zip"

	// rclone copy the zip file
	cmdName := "rclone"
	args := []string{
		"copy", zipName, remoteZipPath,
		"--progress", "--transfers=1", "--checkers=4", "--fast-list",
	}

	log.Debug("Execute: ", "cmd", cmdName, "args", args)

	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
