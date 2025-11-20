package e2e

import (
	"time"
)

// CMXConfig holds configuration for provisioning a CMX VM.
type CMXConfig struct {
	// Name of the VM (defaults to "ec-test-suite")
	Name string
	// Distribution to use (e.g., "ubuntu")
	Distribution string
	// Version of the distribution (e.g., "22.04")
	Version string
	// Instance type (e.g., "t3.medium")
	InstanceType string
	// Disk size in GB
	DiskSize int
	// Number of VMs to create
	Count int
	// How long to wait for VM to be ready
	Wait time.Duration
	// TTL for the VM
	TTL time.Duration
	// Network ID (optional, creates new if empty)
	NetworkID string
}

// DefaultCMXConfig returns a CMXConfig with sensible defaults for E2E tests.
func DefaultCMXConfig() *CMXConfig {
	return &CMXConfig{
		Name:         "ec-e2e-test",
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "t3.medium",
		DiskSize:     50,
		Count:        1,
		Wait:         15 * time.Minute,
		TTL:          2 * time.Hour,
	}
}

// CMXInstance represents a provisioned CMX VM instance.
type CMXInstance struct {
	// VM ID from CMX
	ID string
	// VM name
	Name string
	// Network ID
	NetworkID string
	// Private IP address
	PrivateIP string
	// SSH endpoint for connecting to the VM
	SSHEndpoint string
	// Exposed ports (port -> hostname)
	ExposedPorts map[string]string
}

// TestResult represents the result of a test execution.
type TestResult struct {
	// Name of the test scenario
	Scenario string
	// Installation mode (browser-based or headless)
	Mode string
	// Whether the test passed
	Success bool
	// Error message if test failed
	Error string
	// Duration of the test
	Duration time.Duration
	// Validation results
	ValidationResults *ValidationResult
}

// ValidationResult contains results from installation validation.
type ValidationResult struct {
	// Overall success status
	Success bool
	// Individual check results
	Checks map[string]CheckResult
}

// CheckResult represents the result of a single validation check.
type CheckResult struct {
	// Whether the check passed
	Passed bool
	// Error if check failed
	Error error
	// Additional details
	Details string
}

// InstallResult contains information about a completed installation.
type InstallResult struct {
	// Whether installation succeeded
	Success bool
	// Path to kubeconfig file
	KubeconfigPath string
	// Admin console URL (for browser-based installs)
	AdminConsoleURL string
	// UI port (for browser-based installs)
	UIPort int
	// Installation log output
	InstallationLog string
}

// PlaywrightConfig holds configuration for Playwright-based UI tests.
type PlaywrightConfig struct {
	// Installation scenario (online, airgap)
	Scenario string
	// App version to install
	AppVersion string
	// License content
	License string
	// License ID for downloading
	LicenseID string
	// Base URL for admin console
	BaseURL string
}

// HeadlessConfig holds configuration for headless (CLI) installations.
type HeadlessConfig struct {
	// Installation scenario (online, airgap)
	Scenario string
	// App version to install
	AppVersion string
	// License content
	License string
	// License ID for downloading
	LicenseID string
	// Path to config file (optional)
	ConfigFile string
}
