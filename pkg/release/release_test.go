package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
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

func TestGetReleaseTitle(t *testing.T) {
	release, err := newReleaseDataFrom(testReleaseData)
	assert.NoError(t, err)
	title := release.Application.Spec.Title
	assert.NoError(t, err)
	assert.Equal(t, "Embedded Cluster Smoke Test App", title)
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
