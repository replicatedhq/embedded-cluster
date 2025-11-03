package install

import (
	"context"
	"fmt"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
)

// HeadlessInstallOptions contains the configuration options for a headless installation
type HeadlessInstallOptions struct {
	// ConfigValues are the application config values to use for installation
	ConfigValues apitypes.AppConfigValues

	// LinuxInstallationConfig contains the installation settings for the Linux target
	LinuxInstallationConfig apitypes.LinuxInstallationConfig

	// IgnoreHostPreflights indicates whether to bypass host preflight check failures
	IgnoreHostPreflights bool

	// IgnoreAppPreflights indicates whether to bypass app preflight check failures
	IgnoreAppPreflights bool

	// AirgapBundle is the path to the airgap bundle file (empty string for online installs)
	AirgapBundle string
}

// Orchestrator defines the interface for headless installation operations.
// It orchestrates the installation process by interacting with the v3 API server
// running in-process via HTTP calls to localhost.
type Orchestrator interface {
	// RunHeadlessInstall executes a complete headless installation workflow.
	// It performs the following steps in order:
	//   1. Configure application with config values
	//   2. Configure installation settings
	//   3. Run host preflights (with optional bypass)
	//   4. Setup infrastructure
	//   5. Process airgap bundle (if provided)
	//   6. Run app preflights (with optional bypass)
	//   7. Install application
	//
	// The installation cannot be resumed if it fails after infrastructure setup begins.
	// Any failure after that point requires running 'embedded-cluster reset' and retrying.
	//
	// Returns an error if any step fails, with a descriptive message for recovery.
	RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) error
}

// NewOrchestrator creates a new Orchestrator instance
func NewOrchestrator() Orchestrator {
	return &orchestrator{}
}

// orchestrator is the default implementation of the Orchestrator interface
type orchestrator struct {
}

// RunHeadlessInstall executes a complete headless installation workflow
func (o *orchestrator) RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) error {
	// TODO(PR2): Implement real headless installation orchestration
	return fmt.Errorf("headless installation is not yet fully implemented - coming in a future release")
}
