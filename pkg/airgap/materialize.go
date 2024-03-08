package airgap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const K0S_IMAGE_PATH = "/var/lib/k0s/images/install.tar.gz"

// MaterializeAirgapImages places the the airgap image bundle for k0s
func MaterializeAirgapImages(airgapFile string) error {
	// setup destination
	err := os.MkdirAll(filepath.Dir(K0S_IMAGE_PATH), 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// read file from airgapFile
	rawfile, err := os.Open(airgapFile)
	if err != nil {
		return fmt.Errorf("failed to open airgap file: %w", err)
	}
	defer rawfile.Close()

	// decompress tarball
	ungzip, err := gzip.NewReader(rawfile)
	if err != nil {
		return fmt.Errorf("failed to decompress airgap file: %w", err)
	}

	// iterate through tarball
	tarreader := tar.NewReader(ungzip)
	var nextFile *tar.Header
	for {
		nextFile, err = tarreader.Next()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("application images not found in %s", airgapFile)
			}
			return fmt.Errorf("failed to read airgap file: %w", err)
		}

		if nextFile.Name == "images.tar.gz" {
			break
		}
	}

	// stream to destination file
	var destFile *os.File
	destFile, err = os.Create(K0S_IMAGE_PATH)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()
	err = destFile.Chmod(0755)
	if err != nil {
		return fmt.Errorf("failed to set destination file permissions: %w", err)
	}

	_, err = io.Copy(destFile, tarreader)
	if err != nil {
		return fmt.Errorf("failed to copy images file: %w", err)
	}

	return nil
}
