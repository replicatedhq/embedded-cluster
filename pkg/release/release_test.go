package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

var (
	//go:embed testdata/release.yaml
	releaseData []byte

	//go:embed testdata/velero-multi.yaml
	veleroMultiData []byte

	//go:embed testdata/helmcharts-multi.yaml
	helmchartsMultiData []byte

	//go:embed testdata/mixed-multi.yaml
	mixedMultiData []byte
)

func Test_newReleaseDataFrom(t *testing.T) {
	release, err := newReleaseDataFrom([]byte{})
	require.NoError(t, err)
	assert.NotNil(t, release)
	cfg := release.EmbeddedClusterConfig
	assert.Nil(t, cfg)
}

func TestGetApplication(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)
	app := release.Application
	assert.NotNil(t, app)
}

func TestGetEmbeddedClusterConfig(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)
	cfg := release.EmbeddedClusterConfig
	assert.NotNil(t, cfg)
}

func TestGetHostPreflights(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)
	preflights := release.HostPreflights
	assert.NotNil(t, preflights)
}

func TestGetAppTitle(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)
	title := release.Application.Spec.Title
	assert.Equal(t, "Embedded Cluster Smoke Test App", title)
}

func TestGetHelmChartCRs(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)
	helmCharts := release.HelmChartCRs
	assert.NotNil(t, helmCharts)
	assert.Len(t, helmCharts, 1) // One HelmChart CR in test data
}

func TestGetHelmChartArchives(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)
	archives := release.HelmChartArchives
	assert.NotNil(t, archives)
	assert.Len(t, archives, 1) // One .tgz file in test data
}

func TestGetVeleroBackupAndRestore(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)

	backup := release.VeleroBackup
	assert.NotNil(t, backup)

	restore := release.VeleroRestore
	assert.NotNil(t, restore)
}

func TestParseHelmChartCR(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		wantNil bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: false,
			wantNil: true,
		},
		{
			name: "valid helm chart CR",
			data: []byte(`apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: test-chart
spec:
  chart:
    chartVersion: "1.0.0"
    name: test-chart`),
			wantErr: false,
			wantNil: false,
		},
		{
			name:    "invalid yaml",
			data:    []byte(`invalid: yaml: content`),
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseHelmChartCR(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}

func TestParseWithHelmChartData(t *testing.T) {
	// Create test data with HelmChart CR and .tgz file
	helmChartYAML := `apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: test-chart
spec:
  chart:
    chartVersion: "1.0.0"
    name: test-chart
  optionalValues:
    - when: '{{repl not (empty (ConfigOption "test-option")) }}'
      recursiveMerge: true
      values: repl{{ ConfigOption "test-option" | nindent 8 }}`

	chartArchive := []byte("fake-chart-archive-content")

	testData := map[string][]byte{
		"helmchart.yaml":       []byte(helmChartYAML),
		"test-chart-1.0.0.tgz": chartArchive,
	}

	// Create tar.gz from test data
	buf := bytes.NewBuffer([]byte{})
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	for name, content := range testData {
		err := tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
		})
		assert.NoError(t, err)
		_, err = tw.Write(content)
		assert.NoError(t, err)
	}

	err := tw.Close()
	assert.NoError(t, err)
	err = gw.Close()
	assert.NoError(t, err)

	// Parse the test data
	release, err := newReleaseDataFrom(buf.Bytes())
	assert.NoError(t, err)
	assert.NotNil(t, release)

	// Verify HelmChart CRs
	assert.Len(t, release.HelmChartCRs, 1)
	assert.NotNil(t, release.HelmChartCRs[0])
	assert.Greater(t, len(release.HelmChartCRs[0]), 0)

	// Verify chart archives
	assert.Len(t, release.HelmChartArchives, 1)
	assert.Equal(t, chartArchive, release.HelmChartArchives[0])
}

func TestParseMultiDocumentVeleroYAML(t *testing.T) {
	// Create tar.gz with the multi-document file
	veleroMulti := generateReleaseTGZ(t, veleroMultiData)

	// Parse the release data
	release, err := newReleaseDataFrom(veleroMulti)
	assert.NoError(t, err)
	assert.NotNil(t, release)

	// Verify both Backup and Restore were parsed
	assert.NotNil(t, release.VeleroBackup, "VeleroBackup should be parsed")
	assert.Equal(t, "test-backup", release.VeleroBackup.Name)

	assert.NotNil(t, release.VeleroRestore, "VeleroRestore should be parsed")
	assert.Equal(t, "test-restore", release.VeleroRestore.Name)
}

func TestParseMultiDocumentMixedYAML(t *testing.T) {
	// Create tar.gz with the multi-document file
	mixedMulti := generateReleaseTGZ(t, mixedMultiData)

	// Parse the release data
	release, err := newReleaseDataFrom(mixedMulti)
	require.NoError(t, err)
	assert.NotNil(t, release)

	// Verify all resources were parsed
	assert.NotNil(t, release.Application, "Application should be parsed")
	assert.Equal(t, "Test Application", release.Application.Spec.Title)

	assert.NotNil(t, release.EmbeddedClusterConfig, "EmbeddedClusterConfig should be parsed")
	assert.Equal(t, "controller-test", release.EmbeddedClusterConfig.Spec.Roles.Controller.Name)

	assert.NotNil(t, release.HostPreflights, "HostPreflights should be parsed")
	assert.Len(t, release.HostPreflights.Collectors, 1)
	assert.Len(t, release.HostPreflights.Analyzers, 1)
}

func TestParseMultipleHelmChartCRsInSingleFile(t *testing.T) {
	helmchartsMulti := generateReleaseTGZ(t, helmchartsMultiData)

	// Parse the release data
	release, err := newReleaseDataFrom(helmchartsMulti)
	require.NoError(t, err)
	assert.NotNil(t, release)

	// Verify both HelmChart CRs were parsed
	assert.Len(t, release.HelmChartCRs, 2, "Should have 2 HelmChart CRs")

	// Parse and verify the first chart
	var chart1 map[string]any
	err = yaml.Unmarshal(release.HelmChartCRs[0], &chart1)
	assert.NoError(t, err)
	metadata1 := chart1["metadata"].(map[string]any)
	assert.Equal(t, "chart-1", metadata1["name"])

	// Parse and verify the second chart
	var chart2 map[string]any
	err = yaml.Unmarshal(release.HelmChartCRs[1], &chart2)
	assert.NoError(t, err)
	metadata2 := chart2["metadata"].(map[string]any)
	assert.Equal(t, "chart-2", metadata2["name"])
}

func generateReleaseTGZ(t *testing.T, content []byte) []byte {
	parsed := map[string]string{}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal content: %v", err)
		return nil
	}

	buf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	for name, content := range parsed {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write header: %v", err)
			return nil
		}

		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write content: %v", err)
			return nil
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
		return nil
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
		return nil
	}

	return buf.Bytes()
}
