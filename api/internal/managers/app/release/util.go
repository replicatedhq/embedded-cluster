package release

import (
	"bytes"
	"fmt"
	"os"

	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"helm.sh/helm/v3/pkg/chart/loader"
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
func extractChartMetadata(chartArchive []byte) (name, version string, err error) {
	ch, err := loader.LoadArchive(bytes.NewReader(chartArchive))
	if err != nil {
		return "", "", fmt.Errorf("load archive: %w", err)
	}
	return ch.Metadata.Name, ch.Metadata.Version, nil
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

// mergePreflightSpecs merges multiple preflight specs into a single spec
func mergePreflightSpecs(specs []troubleshootv1beta2.PreflightSpec) *troubleshootv1beta2.PreflightSpec {
	if len(specs) == 0 {
		return nil
	}

	merged := &troubleshootv1beta2.PreflightSpec{
		Analyzers:  []*troubleshootv1beta2.Analyze{},
		Collectors: []*troubleshootv1beta2.Collect{},
	}

	// Merge all analyzers and collectors from all specs
	for _, spec := range specs {
		if spec.Analyzers != nil {
			merged.Analyzers = append(merged.Analyzers, spec.Analyzers...)
		}
		if spec.Collectors != nil {
			merged.Collectors = append(merged.Collectors, spec.Collectors...)
		}
	}

	return merged
}
