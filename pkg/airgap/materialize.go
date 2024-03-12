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

// MaterializeAirgapImages places the airgap image bundle for k0s
// this should be located at 'images-amd64.tar.gz' within embedded-cluster.tar.gz within the airgap bundle
func MaterializeAirgapImages(airgapReader io.Reader) error {
	// setup destination
	err := os.MkdirAll(filepath.Dir(K0S_IMAGE_PATH), 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// decompress tarball
	ungzip, err := gzip.NewReader(airgapReader)
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
				return fmt.Errorf("application images not found in airgap file")
			}
			return fmt.Errorf("failed to read airgap file: %w", err)
		}

		if nextFile.Name == "embedded-cluster.tar.gz" {
			break
		}
	}

	internalUngzip, err := gzip.NewReader(tarreader)
	if err != nil {
		return fmt.Errorf("failed to decompress embedded-cluster.tar.gz within airgap file: %w", err)
	}
	internalTarReader := tar.NewReader(internalUngzip)
	var internalNextFile *tar.Header
	for {
		internalNextFile, err = internalTarReader.Next()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("k0s images not found in embedded-cluster.tar.gz within airgap file")
			}
			return fmt.Errorf("failed to read embedded-cluster.tar.gz within airgap file: %w", err)
		}

		if internalNextFile.Name == "images-amd64.tar.gz" {
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

	_, err = io.Copy(destFile, internalTarReader)
	if err != nil {
		return fmt.Errorf("failed to copy images file: %w", err)
	}

	return nil
}
