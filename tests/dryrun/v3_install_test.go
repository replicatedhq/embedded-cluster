package dryrun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV3InstallHeadless_HappyPath(t *testing.T) {
	licenseFile, configFile := setupV3HeadlessTest(t)

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	// Expect the command to fail with the specific error message
	require.EqualError(t, err, "headless installation is not yet fully implemented - coming in a future release")

	// PersistentPostRunE is not called when the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	t.Logf("Test passed: headless installation correctly returns not implemented error")
}

func TestV3InstallHeadless_Metrics(t *testing.T) {
	licenseFile, configFile := setupV3HeadlessTest(t)

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	// Expect the command to fail with the specific error message
	require.EqualError(t, err, "headless installation is not yet fully implemented - coming in a future release")

	// PersistentPostRunE is not called when the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	// --- validate metrics --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, `"entryCommand":"install"`)
				assert.Regexp(t, `"flags":".*--headless.+--license .+/license.yaml.+--target linux.*"`, payload)
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"InstallationStarted"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"InstallationFailed"`)
			},
		},
	})
}

func setupV3HeadlessTest(t *testing.T) (string, string) {
	// Set ENABLE_V3 environment variable
	t.Setenv("ENABLE_V3", "1")

	// Setup release data
	if err := embedReleaseData(clusterConfigData); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	// Initialize dryrun with mock ReplicatedAPIClient
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		ReplicatedAPIClient: &dryrun.ReplicatedAPIClient{
			License:      nil, // will return the same license that was passed in
			LicenseBytes: []byte(licenseData),
		},
	})

	// Create license file
	licenseFile := filepath.Join(t.TempDir(), "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	// Create config values file (required for headless)
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	createConfigValuesFile(t, configFile)

	return licenseFile, configFile
}

func createConfigValuesFile(t *testing.T, filename string) {
	t.Helper()

	configData := `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_host:
      value: "postgres.example.com"
`
	require.NoError(t, os.WriteFile(filename, []byte(configData), 0644))
}
