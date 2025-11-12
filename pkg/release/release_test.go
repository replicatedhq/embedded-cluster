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

func TestGetChannelRelease(t *testing.T) {
	testReleaseData := generateReleaseTGZ(t, releaseData)
	release, err := newReleaseDataFrom(testReleaseData)
	require.NoError(t, err)
	channel := release.ChannelRelease
	assert.NotNil(t, channel)
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

func TestSplitYAMLDocuments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // number of expected documents
	}{
		{
			name:     "empty input",
			input:    "",
			expected: 0,
		},
		{
			name:     "single document without separator",
			input:    "apiVersion: v1\nkind: Test",
			expected: 1,
		},
		{
			name:     "separator at start of file",
			input:    "---\napiVersion: v1\nkind: Test1\n---\napiVersion: v1\nkind: Test2",
			expected: 2,
		},
		{
			name:     "separator with no surrounding newlines (original bug)",
			input:    "apiVersion: v1\nkind: Test1\n---\napiVersion: v1\nkind: Test2",
			expected: 2,
		},
		{
			name:     "multiple separators",
			input:    "---\napiVersion: v1\nkind: Test1\n---\napiVersion: v1\nkind: Test2\n---\napiVersion: v1\nkind: Test3",
			expected: 3,
		},
		{
			name:     "leading separator with kots config",
			input:    "---\napiVersion: kots.io/v1beta1\nkind: Config\nmetadata:\n  name: test\n  annotations:\n    kots.io/exclude: \"true\"",
			expected: 1,
		},
		{
			name:     "multiline string with block scalar",
			input:    "apiVersion: v1\nkind: Config\ntitle: |-\n  Line 1\n  Line 2",
			expected: 1,
		},
		{
			name:     "empty document between separators",
			input:    "---\napiVersion: v1\nkind: Test1\n---\n---\napiVersion: v1\nkind: Test2",
			expected: 2, // empty documents should be skipped
		},
		{
			name:     "separator in quoted string (should not split)",
			input:    "apiVersion: v1\ndata: \"---\"",
			expected: 1,
		},
		{
			name:     "separator in comment (should not split)",
			input:    "apiVersion: v1\n# Comment with ---\nkind: Test",
			expected: 1,
		},
		{
			name:     "windows line endings",
			input:    "---\r\napiVersion: v1\r\nkind: Test1\r\n---\r\napiVersion: v1\r\nkind: Test2",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := splitYAMLDocuments([]byte(tt.input))
			assert.NoError(t, err)
			assert.Len(t, result, tt.expected, "Expected %d documents but got %d", tt.expected, len(result))

			// Verify each document is valid (non-empty)
			for i, doc := range result {
				assert.NotEmpty(t, doc, "Document %d should not be empty", i)
				// Verify document contains actual content (not just whitespace)
				assert.NotEmpty(t, bytes.TrimSpace(doc), "Document %d should contain non-whitespace content", i)
			}
		})
	}
}

func TestParseConfigWithLeadingSeparator(t *testing.T) {
	// This test verifies the full parsing pipeline for SC-131165
	// With the old decode/re-marshal approach, this would fail at kyaml.Unmarshal
	// With YAMLReader preserving original bytes, kyaml succeeds

	configYAML := `---
apiVersion: kots.io/v1beta1
kind: Config
metadata:
  name: test-config
  annotations:
    kots.io/exclude: "true"
spec:
  groups:
    - name: intro
      title: Intro
      items:
        - name: description
          type: label
          title: |-

            Welcome to the Self-Hosted Installer!`

	testData := map[string][]byte{
		"kots-config.yaml": []byte(configYAML),
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
		require.NoError(t, err)
		_, err = tw.Write(content)
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	// Parse the release data - this is where the old implementation would fail
	release, err := newReleaseDataFrom(buf.Bytes())
	require.NoError(t, err, "Should parse config with leading separator through full pipeline")
	require.NotNil(t, release)

	// Verify the Config was actually parsed by kyaml (not just split)
	require.NotNil(t, release.AppConfig, "AppConfig should be parsed by kyaml")
	assert.Equal(t, "test-config", release.AppConfig.Name)
	require.Len(t, release.AppConfig.Spec.Groups, 1)
	assert.Equal(t, "intro", release.AppConfig.Spec.Groups[0].Name)

	// Verify multiline string was preserved (would be corrupted by decode/re-marshal)
	require.Len(t, release.AppConfig.Spec.Groups[0].Items, 1)
	assert.Contains(t, release.AppConfig.Spec.Groups[0].Items[0].Title, "Welcome to the Self-Hosted Installer")
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
