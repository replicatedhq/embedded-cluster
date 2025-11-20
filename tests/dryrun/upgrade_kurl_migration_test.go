package dryrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TestUpgradeKURLMigration tests that the upgrade command correctly detects
// a kURL cluster and shows the migration message when ENABLE_V3=1.
func TestUpgradeKURLMigration(t *testing.T) {
	t.Setenv("ENABLE_V3", "1")

	// Create the kURL kubeconfig file at the production path
	// This file doesn't need to be a valid kubeconfig since dryrun mode
	// will use the mock client. It just needs to exist for the file check.
	require.NoError(t, os.MkdirAll(filepath.Dir(kubeutils.KURLKubeconfigPath), 0755))
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

	// Create the kotsadm namespace for the password secret
	kotsadmNS := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kotsadm",
		},
	}
	err = kurlKubeClient.Create(context.Background(), kotsadmNS, &ctrlclient.CreateOptions{})
	require.NoError(t, err, "failed to create kotsadm namespace")

	// Create the kotsadm-password secret with the password "password"
	// This is what the migration API will read to authenticate users
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	require.NoError(t, err, "failed to generate password hash")
	kotsadmPasswordSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm-password",
			Namespace: "kotsadm",
		},
		Data: map[string][]byte{
			"passwordBcrypt": passwordHash,
		},
	}
	err = kurlKubeClient.Create(context.Background(), kotsadmPasswordSecret, &ctrlclient.CreateOptions{})
	require.NoError(t, err, "failed to create kotsadm-password secret")

	// Setup release data
	if err := embedReleaseData(clusterConfigData); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	// Create license file
	licenseFile := filepath.Join(tempDir, "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	// Test Migration API endpoints
	t.Run("migration API skeleton", func(t *testing.T) {
		testMigrationAPIEndpoints(t, tempDir, licenseFile)
	})
}

// assertEventuallyMigrationState waits for the migration status to reach the expected state
func assertEventuallyMigrationState(t *testing.T, contextMsg string, expectedState apitypes.MigrationState, getStatus func() (apitypes.MigrationState, string, error)) {
	t.Helper()

	var lastState apitypes.MigrationState
	var lastMsg string
	var lastErr error

	timeout := 10 * time.Second
	interval := 100 * time.Millisecond

	ok := assert.Eventually(t, func() bool {
		st, msg, err := getStatus()
		lastState, lastMsg, lastErr = st, msg, err
		if err != nil {
			return false
		}
		return st == expectedState
	}, timeout, interval, "%s: lastState=%s, lastMsg=%s, lastErr=%v", contextMsg, lastState, lastMsg, lastErr)

	if !ok {
		require.FailNowf(t, "did not reach expected state", "%s: expected state=%s, got state=%s with message: %s", contextMsg, expectedState, lastState, lastMsg)
	}
}

// testMigrationAPIEndpoints tests the migration API endpoints return expected skeleton responses
func testMigrationAPIEndpoints(t *testing.T, tempDir string, licenseFile string) {
	// Start the upgrade command in non-headless mode so API stays up
	// Use --yes to bypass prompts
	go func() {
		err := runInstallerCmd(
			"upgrade",
			"--target", "linux",
			"--license", licenseFile,
			"--yes",
		)
		if err != nil {
			t.Logf("upgrade command exited with error: %v", err)
		}
	}()

	ctx := t.Context()
	managerPort := 30081 // Default port for upgrade mode

	// Wait for API to be ready
	httpClient := insecureHTTPClient()
	waitForAPIReady(t, httpClient, fmt.Sprintf("https://localhost:%d/api/health", managerPort))

	// Build API client and authenticate
	c := apiclient.New(fmt.Sprintf("https://localhost:%d", managerPort), apiclient.WithHTTPClient(httpClient))

	// For upgrade mode, we need to authenticate with a password
	// The upgrade command should have a default password or we need to set one
	// Let's use a default password for testing
	password := "password"
	require.NoError(t, c.Authenticate(ctx, password), "failed to authenticate")

	// POST /api/linux/migration/start with transferMode="copy"
	startResp, err := c.StartKURLMigration(ctx, "copy", nil)
	require.NoError(t, err, "failed to start migration")
	require.NotEmpty(t, startResp.MigrationID, "migrationID should be returned")
	require.Equal(t, "migration started successfully", startResp.Message, "expected success message")

	// GET /api/linux/kurl-migration/status
	// The migration should eventually reach Failed state with the skeleton error
	assertEventuallyMigrationState(t, "migration phase execution not yet implemented", apitypes.MigrationStateFailed, func() (apitypes.MigrationState, string, error) {
		statusResp, err := c.GetKURLMigrationStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return statusResp.State, statusResp.Message, nil
	})

	// Get final status to verify error message
	finalStatus, err := c.GetKURLMigrationStatus(ctx)
	require.NoError(t, err, "failed to get migration status")
	require.Equal(t, apitypes.MigrationStateFailed, finalStatus.State, "migration should be in Failed state")
	require.Contains(t, finalStatus.Error, "migration phase execution not yet implemented",
		"expected skeleton error message in status")
}
