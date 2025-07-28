package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"testing"

	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindChartArchive(t *testing.T) {
	tests := []struct {
		name              string
		helmChartArchives [][]byte
		templatedCR       *kotsv1beta2.HelmChart
		expectedArchive   []byte
		expectError       bool
		errorContains     string
	}{
		{
			name:              "empty archives",
			helmChartArchives: [][]byte{},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "nginx",
						ChartVersion: "1.0.0",
					},
				},
			},
			expectError:   true,
			errorContains: "no helm chart archives found",
		},
		{
			name:              "nil archives",
			helmChartArchives: nil,
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "nginx",
						ChartVersion: "1.0.0",
					},
				},
			},
			expectError:   true,
			errorContains: "no helm chart archives found",
		},
		{
			name:              "empty chart name",
			helmChartArchives: [][]byte{createTestChartArchive(t, "nginx", "1.0.0")},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "",
						ChartVersion: "1.0.0",
					},
				},
			},
			expectError:   true,
			errorContains: "chart name is empty",
		},
		{
			name:              "empty chart version",
			helmChartArchives: [][]byte{createTestChartArchive(t, "nginx", "1.0.0")},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "nginx",
						ChartVersion: "",
					},
				},
			},
			expectError:   true,
			errorContains: "chart version is empty",
		},
		{
			name: "successful match",
			helmChartArchives: [][]byte{
				createTestChartArchive(t, "redis", "2.0.0"),
				createTestChartArchive(t, "nginx", "1.0.0"),
				createTestChartArchive(t, "postgres", "3.0.0"),
			},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "nginx",
						ChartVersion: "1.0.0",
					},
				},
			},
			expectedArchive: createTestChartArchive(t, "nginx", "1.0.0"),
			expectError:     false,
		},
		{
			name: "successful match with base directory chart",
			helmChartArchives: [][]byte{
				createTestChartArchive(t, "redis", "2.0.0"),
				createTestChartArchive(t, "myapp", "1.5.0"),
			},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "myapp",
						ChartVersion: "1.5.0",
					},
				},
			},
			expectedArchive: createTestChartArchive(t, "myapp", "1.5.0"),
			expectError:     false,
		},
		{
			name: "no matching chart",
			helmChartArchives: [][]byte{
				createTestChartArchive(t, "redis", "2.0.0"),
				createTestChartArchive(t, "postgres", "3.0.0"),
			},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "nginx",
						ChartVersion: "1.0.0",
					},
				},
			},
			expectError:   true,
			errorContains: "no chart archive found for chart name nginx and version 1.0.0",
		},
		{
			name: "subchart matching name/version but no top-level chart",
			helmChartArchives: [][]byte{
				createArchiveWithSubchart(t, "nginx", "1.0.0"),
			},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "nginx",
						ChartVersion: "1.0.0",
					},
				},
			},
			expectError:   true,
			errorContains: "Chart.yaml file is missing",
		},
		{
			name: "invalid archive in collection",
			helmChartArchives: [][]byte{
				[]byte("invalid-archive-data"),
			},
			templatedCR: &kotsv1beta2.HelmChart{
				Spec: kotsv1beta2.HelmChartSpec{
					Chart: kotsv1beta2.ChartIdentifier{
						Name:         "nginx",
						ChartVersion: "1.0.0",
					},
				},
			},
			expectError:   true,
			errorContains: "extract chart metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := findChartArchive(tt.helmChartArchives, tt.templatedCR)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result)

				// Validate that the returned archive content matches expected
				assert.Equal(t, tt.expectedArchive, result, "returned archive content should match expected archive")
			}
		})
	}
}

func TestExtractChartMetadata(t *testing.T) {
	tests := []struct {
		name            string
		archiveBytes    []byte
		expectedName    string
		expectedVersion string
		expectError     bool
	}{
		{
			name:            "valid chart archive",
			archiveBytes:    createTestChartArchive(t, "nginx", "1.2.3"),
			expectedName:    "nginx",
			expectedVersion: "1.2.3",
			expectError:     false,
		},
		{
			name:         "invalid gzip data",
			archiveBytes: []byte("not-a-gzip-file"),
			expectError:  true,
		},
		{
			name:         "empty archive",
			archiveBytes: createEmptyArchive(t),
			expectError:  true,
		},
		{
			name:         "archive without Chart.yaml",
			archiveBytes: createArchiveWithoutChart(t),
			expectError:  true,
		},
		{
			name:         "invalid Chart.yaml content",
			archiveBytes: createArchiveWithInvalidChart(t),
			expectError:  true,
		},
		{
			name:         "only subchart Chart.yaml",
			archiveBytes: createArchiveWithSubchart(t, "nginx", "1.0.0"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version, err := extractChartMetadata(tt.archiveBytes)

			if tt.expectError {
				require.Error(t, err)
				assert.Empty(t, name)
				assert.Empty(t, version)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedName, name)
				assert.Equal(t, tt.expectedVersion, version)
			}
		})
	}
}

func TestWriteChartArchiveToTemp(t *testing.T) {
	tests := []struct {
		name         string
		chartArchive []byte
		expectError  bool
	}{
		{
			name:         "valid chart archive",
			chartArchive: createTestChartArchive(t, "nginx", "1.0.0"),
			expectError:  false,
		},
		{
			name:         "empty archive",
			chartArchive: []byte{},
			expectError:  false,
		},
		{
			name:         "large archive",
			chartArchive: bytes.Repeat([]byte("test-data"), 1000),
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, err := writeChartArchiveToTemp(tt.chartArchive)

			if tt.expectError {
				require.Error(t, err)
				assert.Empty(t, filePath)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, filePath)

				// Verify file exists and has correct content
				fileContent, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, tt.chartArchive, fileContent)

				// Clean up
				err = os.Remove(filePath)
				assert.NoError(t, err)
			}
		})
	}
}

// Helper functions for creating test archives

func createTestChartArchive(t *testing.T, name, version string) []byte {
	chartYaml := fmt.Sprintf(`apiVersion: v2
name: %s
version: %s
description: A test Helm chart
type: application
`, name, version)

	return createTarGzArchive(t, map[string]string{
		fmt.Sprintf("%s/Chart.yaml", name): chartYaml,
	})
}

func createEmptyArchive(t *testing.T) []byte {
	return createTarGzArchive(t, map[string]string{})
}

func createArchiveWithoutChart(t *testing.T) []byte {
	return createTarGzArchive(t, map[string]string{
		"mychart/README.md": "some random content",
	})
}

func createArchiveWithInvalidChart(t *testing.T) []byte {
	return createTarGzArchive(t, map[string]string{
		"mychart/Chart.yaml": "invalid: yaml: content: [",
	})
}

func createArchiveWithSubchart(t *testing.T, name, version string) []byte {
	chartYaml := fmt.Sprintf(`apiVersion: v2
name: %s
version: %s
description: A subchart that should be ignored
type: application
`, name, version)

	// Create a subchart within a base chart directory
	chartYamlPath := fmt.Sprintf("%s/charts/subchart/Chart.yaml", name)
	return createTarGzArchive(t, map[string]string{
		chartYamlPath: chartYaml,
	})
}

// createTarGzArchive creates a tar.gz archive with the given files
func createTarGzArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for filename, content := range files {
		header := &tar.Header{
			Name: filename,
			Mode: 0600,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(header))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	return buf.Bytes()
}
