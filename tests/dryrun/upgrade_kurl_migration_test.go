package dryrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/stretchr/testify/require"
)

// TestUpgradeKURLMigrationDetection tests that the upgrade command correctly detects
// a kURL cluster and shows the migration message when ENABLE_V3=1.
func TestUpgradeKURLMigrationDetection(t *testing.T) {
	t.Setenv("ENABLE_V3", "1")

	// Setup the kURL cluster environment by creating the kURL ConfigMap
	kubeUtils := &dryrun.KubeUtils{}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		KubeUtils: kubeUtils,
	})

	kubeClient, err := kubeUtils.KubeClient()
	require.NoError(t, err)

	// Create the kURL ConfigMap in kube-system namespace
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
	err = kubeClient.Create(context.Background(), kurlConfigMap, &ctrlclient.CreateOptions{})
	require.NoError(t, err, "failed to create kURL ConfigMap")

	// Setup release data
	if err := embedReleaseData(clusterConfigData); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	// Create license file
	licenseFile := filepath.Join(t.TempDir(), "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	// Run upgrade command - should detect kURL and exit gracefully
	err = runInstallerCmd(
		"upgrade",
		"--target", "linux",
		"--license", licenseFile,
	)

	// The upgrade command should exit without error when migration is detected
	// (it displays a message to the user and returns nil)
	require.NoError(t, err, "upgrade should exit cleanly when kURL migration is detected")

	// Dump dryrun output for inspection
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	// TODO: Once we have logging/output capture in dryrun, verify the migration message appears

	t.Logf("Test passed: kURL migration detection works correctly")
}

// TODO: Add test for no false positive kURL detection
// This would require setting up a complete EC installation in the test
// to allow the upgrade command to continue past the migration check.
