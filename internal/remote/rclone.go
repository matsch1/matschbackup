package remote

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"
	"matschbackup/pkg"
)

func CheckRcloneRemote(remoteBase string) error {
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

func ListRemoteBackups(remoteBase string) ([]string, error) {
	backup_list, err := pkg.RunCommand("rclone", "lsf", remoteBase, "--dirs-only")
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

func PurgeRemoteDir(remoteBase string, dir string) error {
	fullPath := fmt.Sprintf("%s/%s", remoteBase, dir)
	log.Info("Purging remote dir", "dir", fullPath)
	_, err := pkg.RunCommand("rclone", "purge", fullPath)
	return fmt.Errorf("error purging remote directory %s", err)
}

func CopyToRemote(localPath string, remotePath string, compress bool, dryRun bool) error {
	baseName := strings.ReplaceAll(strings.TrimPrefix(localPath, "/"), "/", "_")
	sourceName := ""
	if compress {
		sourceName = fmt.Sprintf("/tmp/%s.zip", baseName)
		if !dryRun {
			err := pkg.ZipDirectory(localPath, sourceName)
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
		_, err := pkg.RunCommand("rclone", "copy", sourceName, remotePath, "--progress", "--transfers=1", "--checkers=4", "--fast-list")
		if err != nil {
			return fmt.Errorf("failed to copy to remote %s: %w", remotePath, err)
		}

		// Add file to verify backup as completed
		_, err = pkg.RunCommand("rclone", "touch", fmt.Sprintf("%s/%s", remotePath, "BACKUP_COMPLETED"))
		if err != nil {
			return fmt.Errorf("failed to create BACKUP_COMPLETED file")
		}

	} else {
		log.Warn("dryRun: rclone copy not done", "remotePath", remotePath)
	}
	return nil
}

func BackupIsValid(backup string) (bool, error) {
	log.Debug("evaluating backup", "path", backup)

	return true, nil
}

func GetNumberOfValidBackups(backup_remote string) (int, error) {
	log.Debug("get number of valid backups in", "path", backup_remote)

	return 0, nil
}
