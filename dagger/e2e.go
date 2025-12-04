package main

import (
	"context"
	_ "embed"
	"encoding/json"
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
//	  e-2-e-run-headless --scenario=online --app-version=appver-dev-xpXCTO --kube-version=1.33 --license-file=./local-dev/ethan-dev-license.yaml \
//	  export --path=./e2e-results-online
//
// Example (airgap):
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  e-2-e-run-headless --scenario=airgap --app-version=appver-dev-xpXCTO --kube-version=1.33 --license-file=./local-dev/ethan-dev-license.yaml \
//	  export --path=./e2e-results-airgap
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
	// CMX API token
	// +optional
	cmxToken *dagger.Secret,
	// SSH key
	// +optional
	sshKey *dagger.Secret,
	// Skip cleanup
	// +default=false
	skipCleanup bool,
	// Skip support bundle collection
	// +default=false
	skipSupportBundleCollection bool,
) (resultsDir *dagger.Directory) {
	mode := "headless"

	// Initialize test result that will be built up throughout the function
	testResult := &TestResult{
		Scenario: scenario,
		Mode:     mode,
		Success:  false,
	}

	// Initialize results directory
	resultsDir = dag.Directory()

	// Log test start
	fmt.Printf("Starting E2E test: scenario=%s mode=%s app-version=%s kube-version=%s\n",
		scenario, mode, appVersion, kubeVersion)

	// Provision a fresh CMX VM for testing
	fmt.Printf("Provisioning CMX VM for %s %s test...\n", scenario, mode)
	vm, provisionErr := m.ProvisionCmxVm(
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
	if provisionErr != nil {
		testResult.Error = fmt.Sprintf("failed to provision VM: %v", provisionErr)
		resultJSON, _ := json.MarshalIndent(testResult, "", "  ")
		resultsDir = resultsDir.WithNewFile("result.json", string(resultJSON))
		return
	}

	fmt.Printf("Provisioned VM: %s\n", vm.VmID)
	testResult.VMID = vm.VmID

	// Defer function to collect support bundle and cleanup VM
	defer func() {
		// Collect support bundle before cleanup
		if vm != nil && !skipSupportBundleCollection {
			fmt.Printf("Collecting support bundle from VM %s...\n", vm.VmID)
			supportBundle, bundleErr := vm.CollectClusterSupportBundle(ctx)
			if bundleErr != nil {
				fmt.Printf("Warning: failed to collect support bundle: %v\n", bundleErr)
				resultsDir = resultsDir.WithNewFile("support-bundle-error.txt", fmt.Sprintf("Failed to collect support bundle: %v", bundleErr))
			} else {
				resultsDir = resultsDir.WithFile("support-bundle.tar.gz", supportBundle)
			}

			hostSupportBundle, hostBundleErr := vm.CollectHostSupportBundle(ctx)
			if hostBundleErr != nil {
				fmt.Printf("Warning: failed to collect host support bundle: %v\n", hostBundleErr)
				resultsDir = resultsDir.WithNewFile("host-support-bundle-error.txt", fmt.Sprintf("Failed to collect host support bundle: %v", hostBundleErr))
			} else {
				resultsDir = resultsDir.WithFile("host-support-bundle.tar.gz", hostSupportBundle)
			}
		}

		// Marshal final test result to JSON
		resultJSON, marshalErr := json.MarshalIndent(testResult, "", "  ")
		if marshalErr != nil {
			fmt.Printf("Warning: failed to marshal test result: %v\n", marshalErr)
			return
		}
		resultsDir = resultsDir.WithNewFile("result.json", string(resultJSON))

		// Cleanup VM
		if skipCleanup {
			return
		}
		fmt.Printf("Cleaning up CMX VM %s...\n", vm.VmID)
		if _, cleanupErr := vm.Cleanup(ctx); cleanupErr != nil {
			fmt.Printf("Warning: failed to cleanup VM %s: %v\n", vm.VmID, cleanupErr)
		}
	}()

	// Download and prepare embedded-cluster release
	if prepareErr := vm.PrepareRelease(ctx, scenario, appVersion, licenseFile); prepareErr != nil {
		testResult.Error = fmt.Sprintf("failed to prepare release: %v", prepareErr)
		return
	}

	// For airgap scenarios, apply network policy to block internet access
	if scenario == "airgap" {
		fmt.Printf("Applying airgap network policy on VM %s...\n", vm.VmID)
		if airgapErr := vm.ApplyAirgapNetworkPolicy(ctx); airgapErr != nil {
			testResult.Error = fmt.Sprintf("failed to apply airgap network policy: %v", airgapErr)
			return
		}
	}

	// Run headless installation
	fmt.Printf("Running headless installation on VM %s...\n", vm.VmID)
	installResult, installErr := vm.InstallHeadless(
		ctx,
		scenario,
		appVersion,
		licenseFile,
		dag.File("config-values.yaml", configValuesFileContent),
	)
	if installErr != nil {
		testResult.Error = fmt.Sprintf("installation failed: %v", installErr)
		return
	}

	if !installResult.Success {
		testResult.Error = "installation reported failure"
		return
	}

	// Validate installation
	fmt.Printf("Validating installation on VM %s...\n", vm.VmID)
	validationResult := vm.Validate(
		ctx,
		scenario,
		kubeVersion,
		appVersion,
	)

	// Update final test result
	testResult.Success = validationResult.Success
	testResult.ValidationResults = validationResult

	if !validationResult.Success {
		testResult.Error = "validation checks failed"
		fmt.Printf("Test FAILED: %s %s test validation failed\n", scenario, mode)
	} else {
		fmt.Printf("Test PASSED: %s %s test completed successfully\n", scenario, mode)
	}

	return
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
