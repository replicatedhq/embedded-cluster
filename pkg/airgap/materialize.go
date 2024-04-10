package airgap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

const K0sImagePath = "/var/lib/k0s/images/install.tar"

// MaterializeAirgap places the airgap image bundle for k0s and the embedded cluster charts on disk.
// - image bundle should be located at 'images-amd64.tar' within the embedded-cluster directory within the airgap bundle.
// - charts should be located at 'charts.tar.gz' within the embedded-cluster directory within the airgap bundle.
func MaterializeAirgap(airgapReader io.Reader) error {
	// decompress tarball
	ungzip, err := gzip.NewReader(airgapReader)
	if err != nil {
		return fmt.Errorf("failed to decompress airgap file: %w", err)
	}

	// iterate through tarball
	tarreader := tar.NewReader(ungzip)
	foundCharts, foundImages := false, false
	var nextFile *tar.Header
	for {
		nextFile, err = tarreader.Next()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("embedded-cluster.tar.gz not found in airgap file")
			}
			return fmt.Errorf("failed to read airgap file: %w", err)
		}

		if nextFile.Name == "embedded-cluster/images-amd64.tar" {
			err = writeOneFile(tarreader, K0sImagePath, nextFile.Mode)
			if err != nil {
				return fmt.Errorf("failed to write k0s images file: %w", err)
			}
			foundImages = true
		}

		if nextFile.Name == "embedded-cluster/charts.tar.gz" {
			err = writeChartFiles(tarreader)
			if err != nil {
				return fmt.Errorf("failed to write chart files: %w", err)
			}
			foundCharts = true
		}

		if foundCharts && foundImages {
			return nil
		}
	}
}

func writeOneFile(reader io.Reader, path string, mode int64) error {
	// setup destination
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// stream to destination file
	destFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if err := destFile.Chmod(os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to set destination file permissions: %w", err)
	}

	if _, err := io.Copy(destFile, reader); err != nil {
		return fmt.Errorf("failed to copy images file: %w", err)
	}
	return nil
}

// take in a stream of a tarball and write the charts contained within to disk
func writeChartFiles(reader io.Reader) error {
	// decompress tarball
	ungzip, err := gzip.NewReader(reader)
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
				return nil
			}
			return fmt.Errorf("failed to read airgap file: %w", err)
		}

		if nextFile.FileInfo().IsDir() {
			continue
		}

		subdir := defaults.EmbeddedClusterChartsSubDir()
		dst := filepath.Join(subdir, nextFile.Name)
		if err := writeOneFile(tarreader, dst, nextFile.Mode); err != nil {
			return fmt.Errorf("failed to write chart file: %w", err)
		}
	}
}
