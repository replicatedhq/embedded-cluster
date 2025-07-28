package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"gopkg.in/yaml.v2"
)

// findChartArchive finds the chart archive that corresponds to the given HelmChart CR
func findChartArchive(helmChartArchives [][]byte, templatedCR *kotsv1beta2.HelmChart) ([]byte, error) {
	if len(helmChartArchives) == 0 {
		return nil, fmt.Errorf("no helm chart archives found")
	}

	// Get chart name and version from the templated CR
	expectedName := templatedCR.GetChartName()
	expectedVersion := templatedCR.GetChartVersion()

	if expectedName == "" {
		return nil, fmt.Errorf("chart name is empty in HelmChart CR %s", templatedCR.Name)
	}
	if expectedVersion == "" {
		return nil, fmt.Errorf("chart version is empty in HelmChart CR %s", templatedCR.Name)
	}

	// Search through all chart archives to find matching name and version
	for _, archive := range helmChartArchives {
		chartName, chartVersion, err := extractChartMetadata(archive)
		if err != nil {
			return nil, fmt.Errorf("extract chart metadata: %w", err)
		}

		if chartName == expectedName && chartVersion == expectedVersion {
			return archive, nil
		}
	}

	return nil, fmt.Errorf("no chart archive found for chart name %s and version %s", expectedName, expectedVersion)
}

// extractChartMetadata extracts chart name and version from a .tgz archive
func extractChartMetadata(archiveBytes []byte) (name, version string, err error) {
	gzr, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return "", "", fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return "", "", fmt.Errorf("Chart.yaml not found")
		}
		if err != nil {
			return "", "", fmt.Errorf("read tar entry: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Look for Chart.yaml - accept any path ending with it since archives can have a base directory
		if !strings.HasSuffix(header.Name, "Chart.yaml") {
			continue
		}

		// Skip subcharts (with or without a base directory)
		if strings.Contains(header.Name, "/charts/") || strings.HasPrefix(header.Name, "charts/") {
			continue
		}

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			return "", "", fmt.Errorf("read Chart.yaml: %w", err)
		}

		var metadata struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		}
		if err := yaml.Unmarshal(buf.Bytes(), &metadata); err != nil {
			return "", "", fmt.Errorf("unmarshal Chart.yaml: %w", err)
		}

		return metadata.Name, metadata.Version, nil
	}
}

// writeChartArchiveToTemp writes the chart archive to a temporary file and returns the path
func writeChartArchiveToTemp(chartArchive []byte) (string, error) {
	// Create a temporary file for the chart archive
	tmpFile, err := os.CreateTemp("", "helm-chart-*.tgz")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Write the chart archive to the temporary file
	if _, err := tmpFile.Write(chartArchive); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write chart archive: %w", err)
	}

	return tmpFile.Name(), nil
}
