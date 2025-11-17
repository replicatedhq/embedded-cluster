package dryrun

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/stretchr/testify/require"
)

// TestUpgradeKURLMigration tests that the upgrade command correctly detects
// a kURL cluster and shows the migration message when ENABLE_V3=1.
func TestUpgradeKURLMigration(t *testing.T) {
	t.Setenv("ENABLE_V3", "1")

	// Create the kURL kubeconfig file at the production path
	// This file doesn't need to be a valid kubeconfig since dryrun mode
	// will use the mock client. It just needs to exist for the file check.
	if err := os.MkdirAll(filepath.Dir(kubeutils.KURLKubeconfigPath), 0755); err != nil {
		t.Skipf("Skipping test: cannot create %s (needs root/Docker): %v", filepath.Dir(kubeutils.KURLKubeconfigPath), err)
	}
	require.NoError(t, os.WriteFile(kubeutils.KURLKubeconfigPath, []byte("dummy-kubeconfig"), 0644))

	tempDir := t.TempDir()

	// Setup the kURL cluster environment by creating the kURL ConfigMap
	kubeUtils := &dryrun.KubeUtils{}

	drFile := filepath.Join(tempDir, "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		KubeUtils: kubeUtils,
	})

	// Get the kURL mock cluster client to set up kURL resources
	kurlKubeClient, err := kubeUtils.KURLKubeClient()
	require.NoError(t, err)

	// Create the kURL ConfigMap in the kURL cluster
	// This simulates a kURL installation
	kurlConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kurl-config",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"kurl_install_directory": "/var/lib/kurl",
		},
	}
	err = kurlKubeClient.Create(context.Background(), kurlConfigMap, &ctrlclient.CreateOptions{})
	require.NoError(t, err, "failed to create kURL ConfigMap")

	// Setup release data
	if err := embedReleaseData(clusterConfigData); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	// Create license file
	licenseFile := filepath.Join(tempDir, "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	// Capture log output to verify migration message
	var logOutput bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(io.MultiWriter(originalOutput, &logOutput))
	defer logrus.SetOutput(originalOutput)

	// Run upgrade command - should detect kURL and exit gracefully
	err = runInstallerCmd(
		"upgrade",
		"--target", "linux",
		"--license", licenseFile,
	)

	// The upgrade command should exit without error when migration is detected
	// (it displays a message to the user and returns nil)
	require.NoError(t, err, "upgrade should exit cleanly when kURL migration is detected")

	// Verify the migration message was displayed
	output := logOutput.String()
	require.Contains(t, output, "This upgrade will be available in a future release",
		"expected migration message not found in output")
}
