package pkg

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

func ZipDirectory(sourceDir, zipPath string) error {
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
	return err
}
