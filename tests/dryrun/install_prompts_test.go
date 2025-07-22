package dryrun

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	dryruntypes "github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAirgapPromptWhenChannelIsAirgapEnabledButNoBundleProvided(t *testing.T) {
	// Create a release data with airgap enabled
	airgapReleaseData := `# channel release object
channelID: "fake-channel-id"
channelSlug: "fake-channel-slug"
appSlug: "fake-app-slug"
versionLabel: "fake-version-label"
airgap: true
defaultDomains:
  replicatedAppDomain: "staging.replicated.app"
  proxyRegistryDomain: "proxy.staging.replicated.com"
  replicatedRegistryDomain: "registry.staging.replicated.com"
`

	// Create a mock prompt that simulates user declining the online installation
	mockPrompt := prompts.NewMock()
	mockPrompt.On("Confirm", "Do you want to proceed with an online installation?", false).Return(false, nil)

	// Set the test prompt
	prompts.SetTestPrompt(mockPrompt)
	defer prompts.ClearTestPrompt()

	hcli := &helm.MockClient{}

	// Since the user will decline, no helm operations should happen
	// Do not set any expectations on the helm client

	// Use the custom dryrun approach with airgap-enabled release data
	// This should fail because the user declined the prompt
	err := dryrunInstallWithCustomReleaseDataExpectError(t, &dryrun.Client{HelmClient: hcli}, clusterConfigData, airgapReleaseData)

	// Verify that the error occurred due to user declining
	assert.ErrorContains(t, err, "air gap bundle downloaded but flag not provided")

	// Verify that the prompt was called
	mockPrompt.AssertExpectations(t)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestAirgapPromptWhenChannelIsAirgapEnabledAndUserAccepts(t *testing.T) {
	// Create a release data with airgap enabled
	airgapReleaseData := `# channel release object
channelID: "fake-channel-id"
channelSlug: "fake-channel-slug"
appSlug: "fake-app-slug"
versionLabel: "fake-version-label"
airgap: true
defaultDomains:
  replicatedAppDomain: "staging.replicated.app"
  proxyRegistryDomain: "proxy.staging.replicated.com"
  replicatedRegistryDomain: "registry.staging.replicated.com"
`

	// Create a mock prompt that simulates user accepting the online installation
	mockPrompt := prompts.NewMock()
	mockPrompt.On("Confirm", "Do you want to proceed with an online installation?", false).Return(true, nil)
	// Also mock the password prompts that will be called during installation
	mockPrompt.On("Password", "Set the Admin Console password (minimum 6 characters):").Return("testpassword123", nil)
	mockPrompt.On("Password", "Confirm the Admin Console password:").Return("testpassword123", nil)

	// Set the test prompt
	prompts.SetTestPrompt(mockPrompt)
	defer prompts.ClearTestPrompt()

	hcli := &helm.MockClient{}

	mock.InOrder(
		// Installation should proceed when user accepts
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	// Use the custom dryrun approach with airgap-enabled release data
	dr := dryrunInstallWithCustomReleaseData(t, &dryrun.Client{HelmClient: hcli}, clusterConfigData, airgapReleaseData)

	// Verify that the airgap warning appears in the output
	assert.Contains(t, dr.LogOutput, "You downloaded an air gap bundle but didn't provide it with --airgap-bundle")

	// Verify that the installation proceeded
	foundInstallStart := false
	foundInstallSuccess := false
	for _, metric := range dr.Metrics {
		if metric.Title == "InstallationStarted" {
			foundInstallStart = true
		}
		if metric.Title == "InstallationSucceeded" {
			foundInstallSuccess = true
		}
	}

	assert.True(t, foundInstallStart, "InstallationStarted metric should be present")
	assert.True(t, foundInstallSuccess, "InstallationSucceeded metric should be present")

	// Verify that the prompt was called
	mockPrompt.AssertExpectations(t)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestNoAirgapPromptWhenChannelIsNotAirgapEnabled(t *testing.T) {
	// Create a release data with airgap disabled (default behavior)
	nonAirgapReleaseData := `# channel release object
channelID: "fake-channel-id"
channelSlug: "fake-channel-slug"
appSlug: "fake-app-slug"
versionLabel: "fake-version-label"
airgap: false
defaultDomains:
  replicatedAppDomain: "staging.replicated.app"
  proxyRegistryDomain: "proxy.staging.replicated.com"
  replicatedRegistryDomain: "registry.staging.replicated.com"
`

	// Create a mock prompt for admin console password (no airgap confirmation should be needed)
	mockPrompt := prompts.NewMock()
	mockPrompt.On("Password", "Set the Admin Console password (minimum 6 characters):").Return("testpassword123", nil)
	mockPrompt.On("Password", "Confirm the Admin Console password:").Return("testpassword123", nil)

	// Set the test prompt
	prompts.SetTestPrompt(mockPrompt)
	defer prompts.ClearTestPrompt()

	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons - installation should proceed normally
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	// Use the custom dryrun approach with airgap-disabled release data
	dr := dryrunInstallWithCustomReleaseData(t, &dryrun.Client{HelmClient: hcli}, clusterConfigData, nonAirgapReleaseData)

	// Verify that the installation metrics are present (indicating normal flow)
	verifyInstallationSucceeded(t, dr)

	// Verify that the airgap warning does not appear in the output
	assert.NotContains(t, dr.LogOutput, "You downloaded an air gap bundle but didn't provide it with --airgap-bundle")

	// Verify that the mock expectations were met
	mockPrompt.AssertExpectations(t)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// verifyInstallationSucceeded verifies that the installation succeeded by checking for the
// InstallationStarted and InstallationSucceeded metrics
func verifyInstallationSucceeded(t *testing.T, dr dryruntypes.DryRun) {
	foundInstallStart := false
	foundInstallSuccess := false
	for _, metric := range dr.Metrics {
		if metric.Title == "InstallationStarted" {
			foundInstallStart = true
		}
		if metric.Title == "InstallationSucceeded" {
			foundInstallSuccess = true
		}
	}
	assert.True(t, foundInstallStart, "InstallationStarted metric should be present")
	assert.True(t, foundInstallSuccess, "InstallationSucceeded metric should be present")
}

// dryrunInstallWithCustomReleaseData is a helper function that allows custom release data
func dryrunInstallWithCustomReleaseData(t *testing.T, c *dryrun.Client, clusterConfig string, releaseData string, additionalFlags ...string) dryruntypes.DryRun {
	if err := dryrunInstallWithCustomReleaseDataExpectError(t, c, clusterConfig, releaseData, additionalFlags...); err != nil {
		t.Fatalf("fail to dryrun install embedded-cluster: %v", err)
	}

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}
	return *dr
}

// dryrunInstallWithCustomReleaseDataExpectError is a helper function that expects an error during installation
func dryrunInstallWithCustomReleaseDataExpectError(t *testing.T, c *dryrun.Client, clusterConfig string, releaseData string, additionalFlags ...string) error {
	// Set custom release data
	if err := release.SetReleaseDataForTests(map[string][]byte{
		"release.yaml":        []byte(releaseData),
		"cluster-config.yaml": []byte(clusterConfig),
	}); err != nil {
		t.Fatalf("fail to set release data: %v", err)
	}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, c)

	licenseFile := filepath.Join(t.TempDir(), "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	// Return the error instead of failing the test
	return runInstallerCmd(
		append([]string{"install", "--license", licenseFile}, additionalFlags...)...,
	)
}
