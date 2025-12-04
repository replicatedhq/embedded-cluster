package main

import (
	"context"
	_ "embed"
	"fmt"

	"dagger/embedded-cluster/internal/dagger"

	"go.yaml.in/yaml/v3"
)

//go:embed assets/config-values.yaml
var configValuesFileContent string

// E2eRunHeadless runs a headless installation E2E test.
//
// This method provisions a fresh CMX VM, performs a headless installation via CLI,
// validates the installation, and cleans up the VM afterward. It supports both
// online and airgap installation scenarios.
//
// The test:
// - Provisions an Ubuntu 22.04 VM with r1.large instance type (8GB RAM, 4 CPUs)
// - For airgap: applies network policy to block internet access
// - Downloads and installs embedded-cluster via CLI with license
// - Validates installation success using Kubernetes client
// - Returns comprehensive test results including validation details
//
// Example (online):
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  e-2-e-run-headless --scenario=online --app-version=appver-dev-xpXCTO --kube-version=1.33 --license-file=./local-dev/ethan-dev-license.yaml
//
// Example (airgap):
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  e-2-e-run-headless --scenario=airgap --app-version=appver-dev-xpXCTO --kube-version=1.33 --license-file=./local-dev/ethan-dev-license.yaml
func (m *EmbeddedCluster) E2eRunHeadless(
	ctx context.Context,
	// Scenario (online or airgap)
	scenario string,
	// App version to install
	appVersion string,
	// Expected Kubernetes version (e.g., "1.31")
	kubeVersion string,
	// License file
	licenseFile *dagger.File,
	// Skip cleanup
	// +default=false
	skipCleanup bool,
	// CMX API token
	// +optional
	cmxToken *dagger.Secret,
	// SSH key
	// +optional
	sshKey *dagger.Secret,
) (*TestResult, error) {
	startTime := ctx.Value("startTime")
	if startTime == nil {
		// Track start time for duration calculation
		ctx = context.WithValue(ctx, "startTime", fmt.Sprintf("%d", 0))
	}

	mode := "headless"

	// Log test start
	fmt.Printf("Starting E2E test: scenario=%s mode=%s app-version=%s kube-version=%s\n",
		scenario, mode, appVersion, kubeVersion)

	// Provision a fresh CMX VM for testing
	fmt.Printf("Provisioning CMX VM for %s %s test...\n", scenario, mode)
	vm, err := m.ProvisionCmxVm(
		ctx,
		fmt.Sprintf("ec-e2e-%s-%s", scenario, mode),
		"ubuntu",
		"22.04",
		"r1.large", // 8GB RAM, 4 CPUs - enough for single-node cluster
		50,         // 50GB disk
		"10m",      // 10 minute wait for VM to be ready
		"2h",       // 2 hour TTL
		cmxToken,
		sshKey,
	)
	if err != nil {
		return &TestResult{
			Scenario: scenario,
			Mode:     mode,
			Success:  false,
			Error:    fmt.Sprintf("failed to provision VM: %v", err),
		}, err
	}

	// Ensure VM is cleaned up after test completes
	defer func() {
		if skipCleanup {
			return
		}
		fmt.Printf("Cleaning up CMX VM %s...\n", vm.VmID)
		if _, cleanupErr := vm.Cleanup(ctx); cleanupErr != nil {
			fmt.Printf("Warning: failed to cleanup VM %s: %v\n", vm.VmID, cleanupErr)
		}
	}()

	// Download and prepare embedded-cluster release
	if err := vm.PrepareRelease(ctx, scenario, appVersion, licenseFile); err != nil {
		return nil, fmt.Errorf("prepare release: %w", err)
	}

	// For airgap scenarios, apply network policy to block internet access
	if scenario == "airgap" {
		fmt.Printf("Applying airgap network policy on VM %s...\n", vm.VmID)
		if err := vm.ApplyAirgapNetworkPolicy(ctx); err != nil {
			return &TestResult{
				Scenario: scenario,
				Mode:     mode,
				Success:  false,
				Error:    fmt.Sprintf("failed to apply airgap network policy: %v", err),
			}, err
		}
	}

	// Run headless installation
	fmt.Printf("Running headless installation on VM %s...\n", vm.VmID)
	installResult, err := vm.InstallHeadless(
		ctx,
		scenario,
		appVersion,
		licenseFile,
		dag.File("config-values.yaml", configValuesFileContent),
	)
	if err != nil {
		return &TestResult{
			Scenario: scenario,
			Mode:     mode,
			Success:  false,
			Error:    fmt.Sprintf("installation failed: %v", err),
		}, err
	}

	if !installResult.Success {
		return &TestResult{
			Scenario: scenario,
			Mode:     mode,
			Success:  false,
			Error:    "installation reported failure",
		}, fmt.Errorf("installation failed")
	}

	// Validate installation
	fmt.Printf("Validating installation on VM %s...\n", vm.VmID)
	validationResult := vm.Validate(
		ctx,
		scenario,
		kubeVersion,
		appVersion,
	)

	// Build final test result
	testResult := &TestResult{
		Scenario:          scenario,
		Mode:              mode,
		Success:           validationResult.Success,
		ValidationResults: validationResult,
	}

	if !validationResult.Success {
		testResult.Error = "validation checks failed"
		fmt.Printf("Test FAILED: %s %s test validation failed\n", scenario, mode)
		return testResult, fmt.Errorf("validation checks failed")
	}

	fmt.Printf("Test PASSED: %s %s test completed successfully\n", scenario, mode)
	return testResult, nil
}

func parseLicense(ctx context.Context, licenseFile *dagger.File) (contents string, licenseID string, channelID string, err error) {
	contents, err = licenseFile.Contents(ctx)
	if err != nil {
		return
	}
	var license struct {
		Spec struct {
			LicenseID string `yaml:"licenseID"`
			ChannelID string `yaml:"channelID"`
		} `yaml:"spec"`
	}
	if err = yaml.Unmarshal([]byte(contents), &license); err != nil {
		return
	}
	licenseID = license.Spec.LicenseID
	channelID = license.Spec.ChannelID
	return
}
