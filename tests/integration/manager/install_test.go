package manager

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	kubeconfigContents = `apiVersion: v1
kind: Config
clusters:
- name: dummy
  cluster:
    api-version: v1
    server: http://example.com
contexts:
- name: dummy
  context:
    cluster: dummy
    namespace: dummy
    user: dummy
users:
- name: dummy
  user:
    token: dummy
current-context: dummy
`
)

func TestManagerInstall(t *testing.T) {
	ctx := context.Background()

	dataDir := getDataDir(t)
	runtimeconfig.SetDataDir(dataDir)

	// Write a dummy kubeconfig to the data dir
	err := os.MkdirAll(filepath.Dir(runtimeconfig.PathToKubeConfig()), 0755)
	require.NoError(t, err, "failed to create kubeconfig directory")
	err = os.WriteFile(runtimeconfig.PathToKubeConfig(), []byte(kubeconfigContents), 0644)
	require.NoError(t, err, "failed to write kubeconfig")

	manager.SetServiceName("ec")
	err = manager.Install(ctx, t.Logf)
	require.NoError(t, err, "failed to install manager")

	// Verify service files exists
	serviceFileExists := checkFileExists(t, "/etc/systemd/system/ec-manager.service")
	assert.True(t, serviceFileExists, "ec-manager.service file should exist")
	dropInDirExists := checkFileExists(t, "/etc/systemd/system/ec-manager.service.d")
	assert.True(t, dropInDirExists, "ec-manager.service.d drop-in directory should exist")

	// Verify service is enabled and running
	status := getServiceStatus(t, "ec-manager.service")
	assert.Contains(t, status, "enabled", "service should be enabled")

	// Wait for service to start and become ready
	// TODO: this should be added to the manager package
	assert.Eventually(t, func() bool {
		status := getServiceStatus(t, "ec-manager.service")
		return strings.Contains(status, "active (running)")
	}, 10*time.Second, 1*time.Second, "service should be running")

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
