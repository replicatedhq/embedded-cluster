package manager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInstall(t *testing.T) {
	tmpDir := t.TempDir()

	// Set the systemd unit base path to our temp directory
	systemd.SetSystemDUnitBasePath(tmpDir)
	defer func() {
		systemd.SetSystemDUnitBasePath(systemd.DefaultSystemDUnitBasePath)
	}()

	runtimeconfig.SetDataDir(tmpDir + "/ec-data-dir")
	defer func() {
		runtimeconfig.SetDataDir(ecv1beta1.DefaultDataDir)
	}()

	SetServiceName("test-service")
	defer func() {
		SetServiceName(DefaultServiceName)
	}()
	unitName := "test-service-manager.service"

	// Create a mock DBus
	mockDBus := &systemd.MockDBus{}
	systemd.Set(mockDBus)

	// Set up expectations for the mock
	mockDBus.On("Reload", mock.Anything).Return(nil)
	mockDBus.On("EnableAndStart", mock.Anything, unitName).Return(nil)

	discardLogger := func(string, ...interface{}) {}

	// Run the Install function
	err := Install(context.Background(), discardLogger)
	assert.NoError(t, err)

	// Verify the unit file was written correctly
	unitFilePath := filepath.Join(tmpDir, unitName)
	contents, err := os.ReadFile(unitFilePath)
	assert.NoError(t, err)
	assert.Equal(t, _systemdUnitFileContents, contents)

	// Verify the drop-in file was written correctly
	dropInPath := filepath.Join(tmpDir, unitName+".d", "embedded-cluster.conf")
	dropInContents, err := os.ReadFile(dropInPath)
	assert.NoError(t, err)
	assert.Contains(t, string(dropInContents), "ExecStart="+tmpDir+"/ec-data-dir/bin/manager start")

	// Verify the mock was called as expected
	mockDBus.AssertExpectations(t)
}

func TestUninstall(t *testing.T) {
	tmpDir := t.TempDir()

	// Set the systemd unit base path to our temp directory
	systemd.SetSystemDUnitBasePath(tmpDir)
	defer func() {
		systemd.SetSystemDUnitBasePath(systemd.DefaultSystemDUnitBasePath)
	}()

	SetServiceName("test-service")
	defer func() {
		SetServiceName(DefaultServiceName)
	}()
	unitName := "test-service-manager.service"

	// Create the unit file and drop-in directory that we'll be removing
	unitFilePath := filepath.Join(tmpDir, unitName)
	dropInDir := filepath.Join(tmpDir, unitName+".d")
	dropInFile := filepath.Join(dropInDir, "embedded-cluster.conf")

	err := os.WriteFile(unitFilePath, []byte("test content"), 0644)
	assert.NoError(t, err)

	err = os.MkdirAll(dropInDir, 0755)
	assert.NoError(t, err)

	err = os.WriteFile(dropInFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Create a mock DBus
	mockDBus := &systemd.MockDBus{}
	systemd.Set(mockDBus)

	// Set up expectations for the mock
	mockDBus.On("UnitExists", mock.Anything, unitName).Return(true, nil)
	mockDBus.On("Stop", mock.Anything, unitName).Return(nil)
	mockDBus.On("Disable", mock.Anything, unitName).Return(nil)

	discardLogger := func(string, ...interface{}) {}

	// Run the Uninstall function
	err = Uninstall(context.Background(), discardLogger)
	assert.NoError(t, err)

	// Verify the unit file and drop-in directory were removed
	_, err = os.Stat(unitFilePath)
	assert.True(t, os.IsNotExist(err), "unit file should be removed")

	_, err = os.Stat(dropInDir)
	assert.True(t, os.IsNotExist(err), "drop-in directory should be removed")

	// Verify the mock was called as expected
	mockDBus.AssertExpectations(t)
}

func TestSystemdUnitFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Set the systemd unit base path to our temp directory
	systemd.SetSystemDUnitBasePath(tmpDir)
	defer func() {
		systemd.SetSystemDUnitBasePath(systemd.DefaultSystemDUnitBasePath)
	}()

	// Test with default service name
	expected := filepath.Join(tmpDir, "manager-manager.service")
	actual := SystemdUnitFilePath()
	assert.Equal(t, expected, actual, "default service name path mismatch")

	// Test with custom service name
	SetServiceName("custom")
	defer func() {
		SetServiceName(DefaultServiceName)
	}()
	expected = filepath.Join(tmpDir, "custom-manager.service")
	actual = SystemdUnitFilePath()
	assert.Equal(t, expected, actual, "custom service name path mismatch")
}
