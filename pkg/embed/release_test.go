package embed

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewReleaseDataFrom(t *testing.T) {
	release, err := NewReleaseDataFrom([]byte{})
	assert.NoError(t, err)
	assert.NotNil(t, release)
	cfg, err := release.GetEmbeddedClusterConfig()
	assert.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestGetApplication(t *testing.T) {
	data, err := os.ReadFile("testdata/release.tar.gz")
	assert.NoError(t, err)
	release, err := NewReleaseDataFrom(data)
	assert.NoError(t, err)
	app, err := release.GetApplication()
	assert.NoError(t, err)
	assert.NotNil(t, app)
}

func TestGetEmbeddedClusterConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/release.tar.gz")
	assert.NoError(t, err)
	release, err := NewReleaseDataFrom(data)
	assert.NoError(t, err)
	app, err := release.GetEmbeddedClusterConfig()
	assert.NoError(t, err)
	assert.NotNil(t, app)
}

func TestGetNonExistentLicense(t *testing.T) {
	data, err := os.ReadFile("testdata/release.tar.gz")
	assert.NoError(t, err)
	release, err := NewReleaseDataFrom(data)
	assert.NoError(t, err)
	app, err := release.GetLicense()
	assert.NoError(t, err)
	assert.Nil(t, app)
}
