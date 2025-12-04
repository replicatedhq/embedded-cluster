package main

import "fmt"

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

// ValidationResult contains the results of all validation checks.
type ValidationResult struct {
	// Whether all validation checks passed
	Success bool
	// Kubernetes cluster health check result
	ClusterHealth *CheckResult
	// Installation CRD status check result
	InstallationCRD *CheckResult
	// Application deployment check result
	AppDeployment *CheckResult
	// Admin console components check result
	AdminConsole *CheckResult
	// Data directory configuration check result
	DataDirectories *CheckResult
	// Pod and job health check result
	PodsAndJobs *CheckResult
}

func (v *ValidationResult) String() string {
	return fmt.Sprintf(
		"ValidationResult{\n  Success: %t\n  ClusterHealth: %s\n  InstallationCRD: %s\n  AppDeployment: %s\n  AdminConsole: %s\n  DataDirectories: %s\n  PodsAndJobs: %s\n}",
		v.Success,
		v.ClusterHealth.String(),
		v.InstallationCRD.String(),
		v.AppDeployment.String(),
		v.AdminConsole.String(),
		v.DataDirectories.String(),
		v.PodsAndJobs.String(),
	)
}

// CheckResult contains the result of a single validation check.
type CheckResult struct {
	// Whether the check passed
	Passed bool
	// Error message if the check failed (empty if passed)
	ErrorMessage string
	// Additional context or details about the check
	Details string
}

func (c *CheckResult) String() string {
	return fmt.Sprintf("CheckResult{Passed: %t, ErrorMessage: %q, Details: %q}", c.Passed, c.ErrorMessage, c.Details)
}

// TestResult contains the result of an E2E test execution.
type TestResult struct {
	// Test scenario (online, airgap)
	Scenario string
	// Installation mode (headless, browser-based)
	Mode string
	// Whether the test succeeded
	Success bool
	// Error message if test failed
	Error string
	// Test execution duration
	Duration string
	// VM ID used for the test (for cleanup or support bundle collection)
	VMID string
	// Validation results from the test
	ValidationResults *ValidationResult
}

func (t *TestResult) String() string {
	return fmt.Sprintf("TestResult{Scenario: %s, Mode: %s, Success: %t, Error: %q, Duration: %s, VMID: %s, ValidationResults: %s}",
		t.Scenario, t.Mode, t.Success, t.Error, t.Duration, t.VMID, t.ValidationResults)
}
