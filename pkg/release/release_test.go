package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"
)

// Global test data to avoid regenerating for each test
var testReleaseData []byte

func init() {
	data, err := generateReleaseTGZ()
	if err != nil {
		panic("Failed to generate test release data: " + err.Error())
	}
	testReleaseData = data
}

func Test_newReleaseDataFrom(t *testing.T) {
	release, err := newReleaseDataFrom([]byte{})
	assert.NoError(t, err)
	assert.NotNil(t, release)
	cfg := release.EmbeddedClusterConfig
	assert.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestGetApplication(t *testing.T) {
	release, err := newReleaseDataFrom(testReleaseData)
	assert.NoError(t, err)
	app := release.Application
	assert.NoError(t, err)
	assert.NotNil(t, app)
}

func TestGetEmbeddedClusterConfig(t *testing.T) {
	release, err := newReleaseDataFrom(testReleaseData)
	assert.NoError(t, err)
	cfg := release.EmbeddedClusterConfig
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestGetHostPreflights(t *testing.T) {
	release, err := newReleaseDataFrom(testReleaseData)
	assert.NoError(t, err)
	preflights := release.HostPreflights
	assert.NoError(t, err)
	assert.NotNil(t, preflights)
}

func TestGetAppTitle(t *testing.T) {
	release, err := newReleaseDataFrom(testReleaseData)
	assert.NoError(t, err)
	title := release.Application.Spec.Title
	assert.NoError(t, err)
	assert.Equal(t, "Embedded Cluster Smoke Test App", title)
}

func TestGetHelmChartCRs(t *testing.T) {
	release, err := newReleaseDataFrom(testReleaseData)
	assert.NoError(t, err)
	helmCharts := release.HelmChartCRs
	assert.NoError(t, err)
	assert.NotNil(t, helmCharts)
	assert.Len(t, helmCharts, 1) // One HelmChart CR in test data
}

func TestGetHelmChartArchives(t *testing.T) {
	release, err := newReleaseDataFrom(testReleaseData)
	assert.NoError(t, err)
	archives := release.HelmChartArchives
	assert.NoError(t, err)
	assert.NotNil(t, archives)
	assert.Len(t, archives, 1) // One .tgz file in test data
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

func generateReleaseTGZ() ([]byte, error) {
	content, err := os.ReadFile("testdata/release.yaml")
	if err != nil {
		return nil, err
	}

	parsed := map[string]string{}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		return nil, err
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
			return nil, err
		}

		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
