package manager

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerInstall(t *testing.T) {
	ctx := context.Background()

	runtimeconfig.SetDataDir(getDataDir(t))

	manager.SetServiceName("ec")
	err := manager.Install(ctx, t.Logf)
	require.NoError(t, err, "failed to install manager")

	// Verify service files exists
	serviceFileExists := checkFileExists(t, "/etc/systemd/system/ec-manager.service")
	assert.True(t, serviceFileExists, "ec-manager.service file should exist")
	dropInDirExists := checkFileExists(t, "/etc/systemd/system/ec-manager.service.d")
	assert.True(t, dropInDirExists, "ec-manager.service.d drop-in directory should exist")

	// Wait for service to start and become ready
	// TODO: this should be added to the manager package
	time.Sleep(5 * time.Second)

	// Verify service is enabled and running
	status := getServiceStatus(t, "ec-manager.service")
	assert.Contains(t, status, "enabled", "service should be enabled")
	assert.Contains(t, status, "active (running)", "service should be running")

	err = manager.Uninstall(ctx, t.Logf)
	require.NoError(t, err, "failed to uninstall manager")

	// Verify service files do not exist
	serviceFileExists = checkFileNotExists(t, "/etc/systemd/system/ec-manager.service")
	assert.True(t, serviceFileExists, "ec-manager.service file should not exist")
	dropInDirExists = checkFileNotExists(t, "/etc/systemd/system/ec-manager.service.d")
	assert.True(t, dropInDirExists, "ec-manager.service.d drop-in directory should not exist")

	// Verify service is disabled and not running
	status = getServiceStatus(t, "ec-manager.service")
	assert.Contains(t, status, "could not be found", "service should be removed")
}

func getDataDir(t *testing.T) string {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		t.Fatal("DATA_DIR must be set")
	}
	return dataDir
}

func checkFileExists(t *testing.T, path string) bool {
	err := exec.Command("test", "-e", path).Run()
	return err == nil
}

func checkFileNotExists(t *testing.T, path string) bool {
	err := exec.Command("test", "-e", path).Run()
	return err != nil
}

func getServiceStatus(t *testing.T, service string) string {
	cmd := exec.Command("systemctl", "status", service)
	output, _ := cmd.CombinedOutput()
	return string(output)
}
