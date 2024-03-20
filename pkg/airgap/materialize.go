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

// MaterializeAirgap places the airgap image bundle for k0s
// this should be located at 'images-amd64.tar.gz' within embedded-cluster.tar.gz within the airgap bundle
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
			err = writeOneFile(tarreader, K0sImagePath)
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

func writeOneFile(reader io.Reader, path string) error {
	// setup destination
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// stream to destination file
	destFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()
	err = destFile.Chmod(0755)
	if err != nil {
		return fmt.Errorf("failed to set destination file permissions: %w", err)
	}

	_, err = io.Copy(destFile, reader)
	if err != nil {
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

		if !nextFile.FileInfo().IsDir() {
			err = writeOneFile(tarreader, filepath.Join(defaults.EmbeddedClusterChartsSubDir(), nextFile.Name))
			if err != nil {
				return fmt.Errorf("failed to write chart file: %w", err)
			}
		}
	}
}
