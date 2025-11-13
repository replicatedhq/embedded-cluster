package install

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_orchestrator_configureApplication(t *testing.T) {
	tests := []struct {
		name                     string
		mockPatchFunc            func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
		configValues             apitypes.AppConfigValues
		expectError              bool
		expectedErrorMsg         string
		expectedLogMessages      []string
		expectedProgressMessages []string
	}{
		{
			name: "success",
			mockPatchFunc: func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
				return values, nil
			},
			configValues: apitypes.AppConfigValues{
				"hostname": apitypes.AppConfigValue{
					Value: "test.example.com",
				},
			},
			expectError:         false,
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Configuring application...",
				"Application configuration complete",
			},
		},
		{
			name: "validation errors",
			mockPatchFunc: func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
				return nil, &apitypes.APIError{
					StatusCode: 400,
					Message:    "field errors",
					Errors: []*apitypes.APIError{
						{
							Field:   "database_host",
							Message: "required field missing",
						},
						{
							Field:   "replica_count",
							Message: "value \"10\" exceeds maximum allowed value 5",
						},
						{
							Field:   "enable_ssl",
							Message: "validation rule failed: SSL requires cert_path to be set",
						},
					},
				}
			},
			configValues: apitypes.AppConfigValues{
				"database_host": apitypes.AppConfigValue{
					Value: "",
				},
				"replica_count": apitypes.AppConfigValue{
					Value: "10",
				},
			},
			expectError:         true,
			expectedErrorMsg:    "application configuration validation failed: field errors:\n  - Field 'database_host': required field missing\n  - Field 'replica_count': value \"10\" exceeds maximum allowed value 5\n  - Field 'enable_ssl': validation rule failed: SSL requires cert_path to be set",
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Configuring application...",
				"Application configuration failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockClient := &mockAPIClient{
				patchLinuxInstallAppConfigValuesFunc: tt.mockPatchFunc,
			}

			// Create logger with capture hook
			logger := logrus.New()
			logger.SetOutput(io.Discard) // Discard actual output, we only want the hook
			logCapture := test.NewLocal(logger)

			// Capture progress writer messages
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			opts := HeadlessInstallOptions{
				ConfigValues: tt.configValues,
			}

			// Execute
			err := orchestrator.configureApplication(context.Background(), opts)

			// Assert error
			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			// Assert log messages
			allMessages := []string{}
			for _, entry := range logCapture.AllEntries() {
				allMessages = append(allMessages, entry.Message)
			}
			assert.Equal(t, tt.expectedLogMessages, allMessages, "log messages should match")

			// Assert progress messages
			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages(), "progress messages should match")
		})
	}
}

func Test_orchestrator_configureInstallation(t *testing.T) {
	tests := []struct {
		name                     string
		mockConfigureFunc        func(ctx context.Context, config apitypes.LinuxInstallationConfig) (apitypes.Status, error)
		mockGetStatusFunc        func(ctx context.Context) (apitypes.Status, error)
		installationConfig       apitypes.LinuxInstallationConfig
		expectError              bool
		expectedErrorMsg         string
		expectedProgressMessages []string
	}{
		{
			name: "success",
			mockConfigureFunc: func(ctx context.Context, config apitypes.LinuxInstallationConfig) (apitypes.Status, error) {
				return apitypes.Status{State: apitypes.StateRunning}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.Status, error) {
				return apitypes.Status{
					State:       apitypes.StateSucceeded,
					Description: "Configuration complete",
				}, nil
			},
			installationConfig: apitypes.LinuxInstallationConfig{
				DataDirectory: "/var/lib/embedded-cluster",
			},
			expectError: false,
			expectedProgressMessages: []string{
				"Configuring installation settings...",
				"Installation configuration complete",
			},
		},
		{
			name: "validation errors",
			mockConfigureFunc: func(ctx context.Context, config apitypes.LinuxInstallationConfig) (apitypes.Status, error) {
				return apitypes.Status{}, &apitypes.APIError{
					StatusCode: 400,
					Message:    "installation settings validation failed",
					Errors: []*apitypes.APIError{
						{
							Message: "Pod CIDR 10.96.0.0/12 overlaps with service CIDR 10.96.0.0/16",
						},
					},
				}
			},
			installationConfig: apitypes.LinuxInstallationConfig{
				PodCIDR:     "10.96.0.0/12",
				ServiceCIDR: "10.96.0.0/16",
			},
			expectError:      true,
			expectedErrorMsg: "installation configuration validation failed: installation settings validation failed:\n  - Pod CIDR 10.96.0.0/12 overlaps with service CIDR 10.96.0.0/16",
			expectedProgressMessages: []string{
				"Configuring installation settings...",
				"Installation configuration failed",
			},
		},
		{
			name: "configuration failed status",
			mockConfigureFunc: func(ctx context.Context, config apitypes.LinuxInstallationConfig) (apitypes.Status, error) {
				return apitypes.Status{
					State:       apitypes.StateFailed,
					Description: "Network interface 'eth1' not found",
				}, nil
			},
			installationConfig: apitypes.LinuxInstallationConfig{
				NetworkInterface: "eth1",
			},
			expectError:      true,
			expectedErrorMsg: "installation configuration failed: Network interface 'eth1' not found",
			expectedProgressMessages: []string{
				"Configuring installation settings...",
				"Installation configuration failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAPIClient{
				configureLinuxInstallationFunc: tt.mockConfigureFunc,
				getLinuxInstallationStatusFunc: tt.mockGetStatusFunc,
			}

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			opts := HeadlessInstallOptions{
				LinuxInstallationConfig: tt.installationConfig,
			}

			err := orchestrator.configureInstallation(context.Background(), opts)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages())
		})
	}
}

func Test_orchestrator_runHostPreflights(t *testing.T) {
	tests := []struct {
		name                     string
		mockRunFunc              func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error)
		mockGetStatusFunc        func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error)
		ignoreFailures           bool
		expectError              bool
		expectedErrorMsg         string
		expectedLogMessages      []string
		expectedProgressMessages []string
	}{
		{
			name: "success - no failures",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateSucceeded,
						Description: "All checks passed",
					},
					Output: &apitypes.PreflightsOutput{
						Pass: []apitypes.PreflightsRecord{
							{Title: "Disk space", Message: "Sufficient disk space"},
						},
					},
				}, nil
			},
			ignoreFailures:      false,
			expectError:         false,
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Running host preflights...",
				"Host preflights passed",
			},
		},
		{
			name: "check failures without bypass",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "Completed with failures",
					},
					Output: &apitypes.PreflightsOutput{
						Fail: []apitypes.PreflightsRecord{
							{Title: "Disk space", Message: "Insufficient disk space"},
						},
					},
				}, nil
			},
			ignoreFailures:   false,
			expectError:      true,
			expectedErrorMsg: "host preflight checks completed with failures",
			expectedLogMessages: []string{
				"\n⚠ Warning: Host preflight checks completed with failures\n",
				"  [ERROR] Disk space: Insufficient disk space",
				"\nPlease correct the above issues and retry, or run with --ignore-host-preflights to bypass (not recommended).\n",
			},
			expectedProgressMessages: []string{
				"Running host preflights...",
				"Host preflights completed with failures",
			},
		},
		{
			name: "check failures with bypass",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "Completed with failures",
					},
					Output: &apitypes.PreflightsOutput{
						Fail: []apitypes.PreflightsRecord{
							{Title: "Disk space", Message: "Insufficient disk space"},
						},
					},
				}, nil
			},
			ignoreFailures: true,
			expectError:    false,
			expectedLogMessages: []string{
				"\n⚠ Warning: Host preflight checks completed with failures\n",
				"  [ERROR] Disk space: Insufficient disk space",
				"\nInstallation will continue, but the system may not meet requirements (failures bypassed with flag).\n",
			},
			expectedProgressMessages: []string{
				"Running host preflights...",
				"Host preflights completed with failures",
			},
		},
		{
			name: "execution failure",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
				return apitypes.InstallHostPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "execution failed",
					},
					Output: nil,
				}, nil
			},
			ignoreFailures:      false,
			expectError:         true,
			expectedErrorMsg:    "host preflights execution failed: execution failed",
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Running host preflights...",
				"Host preflights execution failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAPIClient{
				runLinuxInstallHostPreflightsFunc:       tt.mockRunFunc,
				getLinuxInstallHostPreflightsStatusFunc: tt.mockGetStatusFunc,
			}

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			logCapture := test.NewLocal(logger)
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			err := orchestrator.runHostPreflights(context.Background(), tt.ignoreFailures)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			allMessages := []string{}
			for _, entry := range logCapture.AllEntries() {
				allMessages = append(allMessages, entry.Message)
			}
			assert.Equal(t, tt.expectedLogMessages, allMessages)
			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages())
		})
	}
}

func Test_orchestrator_setupInfrastructure(t *testing.T) {
	tests := []struct {
		name                     string
		mockSetupFunc            func(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error)
		mockGetStatusFunc        func(ctx context.Context) (apitypes.Infra, error)
		ignoreHostPreflights     bool
		expectError              bool
		expectedErrorMsg         string
		expectedProgressMessages []string
	}{
		{
			name: "success",
			mockSetupFunc: func(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error) {
				return apitypes.Infra{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.Infra, error) {
				return apitypes.Infra{
					Status: apitypes.Status{
						State:       apitypes.StateSucceeded,
						Description: "Infrastructure ready",
					},
				}, nil
			},
			ignoreHostPreflights: false,
			expectError:          false,
			expectedProgressMessages: []string{
				"Setting up infrastructure...",
				"Infrastructure setup complete",
			},
		},
		{
			name: "setup failure",
			mockSetupFunc: func(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error) {
				return apitypes.Infra{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.Infra, error) {
				return apitypes.Infra{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "K0s failed to start: context deadline exceeded",
					},
				}, nil
			},
			ignoreHostPreflights: false,
			expectError:          true,
			expectedErrorMsg:     "infrastructure setup failed: K0s failed to start: context deadline exceeded",
			expectedProgressMessages: []string{
				"Setting up infrastructure...",
				"Infrastructure setup failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAPIClient{
				setupLinuxInfraFunc:     tt.mockSetupFunc,
				getLinuxInfraStatusFunc: tt.mockGetStatusFunc,
			}

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			err := orchestrator.setupInfrastructure(context.Background(), tt.ignoreHostPreflights)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages())
		})
	}
}

func Test_orchestrator_processAirgap(t *testing.T) {
	tests := []struct {
		name                     string
		mockProcessFunc          func(ctx context.Context) (apitypes.Airgap, error)
		mockGetStatusFunc        func(ctx context.Context) (apitypes.Airgap, error)
		expectError              bool
		expectedErrorMsg         string
		expectedProgressMessages []string
	}{
		{
			name: "success",
			mockProcessFunc: func(ctx context.Context) (apitypes.Airgap, error) {
				return apitypes.Airgap{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.Airgap, error) {
				return apitypes.Airgap{
					Status: apitypes.Status{
						State:       apitypes.StateSucceeded,
						Description: "Airgap bundle processed",
					},
				}, nil
			},
			expectError: false,
			expectedProgressMessages: []string{
				"Processing airgap bundle...",
				"Airgap processing complete",
			},
		},
		{
			name: "processing failure",
			mockProcessFunc: func(ctx context.Context) (apitypes.Airgap, error) {
				return apitypes.Airgap{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.Airgap, error) {
				return apitypes.Airgap{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "Failed to load images from bundle",
					},
				}, nil
			},
			expectError:      true,
			expectedErrorMsg: "airgap processing failed: Failed to load images from bundle",
			expectedProgressMessages: []string{
				"Processing airgap bundle...",
				"Airgap processing failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAPIClient{
				processLinuxAirgapFunc:   tt.mockProcessFunc,
				getLinuxAirgapStatusFunc: tt.mockGetStatusFunc,
			}

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			err := orchestrator.processAirgap(context.Background())

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages())
		})
	}
}

func Test_orchestrator_runAppPreflights(t *testing.T) {
	tests := []struct {
		name                     string
		mockRunFunc              func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
		mockGetStatusFunc        func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
		ignoreFailures           bool
		expectError              bool
		expectedErrorMsg         string
		expectedLogMessages      []string
		expectedProgressMessages []string
	}{
		{
			name: "success - no failures",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateSucceeded,
						Description: "All checks passed",
					},
					Output: &apitypes.PreflightsOutput{
						Pass: []apitypes.PreflightsRecord{
							{Title: "Database connectivity", Message: "Successfully connected"},
						},
					},
				}, nil
			},
			ignoreFailures:      false,
			expectError:         false,
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Running app preflights...",
				"App preflights passed",
			},
		},
		{
			name: "check failures with bypass",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "Completed with failures",
					},
					Output: &apitypes.PreflightsOutput{
						Fail: []apitypes.PreflightsRecord{
							{Title: "Database connectivity", Message: "Cannot connect to database"},
						},
					},
				}, nil
			},
			ignoreFailures: true,
			expectError:    false,
			expectedLogMessages: []string{
				"\n⚠ Warning: Application preflight checks completed with failures\n",
				"  [ERROR] Database connectivity: Cannot connect to database",
				"\nInstallation will continue, but the application may not function correctly (failures bypassed with flag).\n",
			},
			expectedProgressMessages: []string{
				"Running app preflights...",
				"App preflights completed with failures",
			},
		},
		{
			name: "check failures without bypass",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "Completed with failures",
					},
					Output: &apitypes.PreflightsOutput{
						Fail: []apitypes.PreflightsRecord{
							{Title: "Database connectivity", Message: "Cannot connect to database"},
						},
					},
				}, nil
			},
			ignoreFailures:   false,
			expectError:      true,
			expectedErrorMsg: "app preflight checks completed with failures",
			expectedLogMessages: []string{
				"\n⚠ Warning: Application preflight checks completed with failures\n",
				"  [ERROR] Database connectivity: Cannot connect to database",
				"\nPlease correct the above issues and retry, or run with --ignore-app-preflights to bypass (not recommended).\n",
			},
			expectedProgressMessages: []string{
				"Running app preflights...",
				"App preflights completed with failures",
			},
		},
		{
			name: "execution failure",
			mockRunFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
				return apitypes.InstallAppPreflightsStatusResponse{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "execution failed",
					},
					Output: nil,
				}, nil
			},
			ignoreFailures:      false,
			expectError:         true,
			expectedErrorMsg:    "app preflights execution failed: execution failed",
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Running app preflights...",
				"App preflights execution failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAPIClient{
				runLinuxInstallAppPreflightsFunc:       tt.mockRunFunc,
				getLinuxInstallAppPreflightsStatusFunc: tt.mockGetStatusFunc,
			}

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			logCapture := test.NewLocal(logger)
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			err := orchestrator.runAppPreflights(context.Background(), tt.ignoreFailures)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			allMessages := []string{}
			for _, entry := range logCapture.AllEntries() {
				allMessages = append(allMessages, entry.Message)
			}
			assert.Equal(t, tt.expectedLogMessages, allMessages)
			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages())
		})
	}
}

func Test_orchestrator_installApp(t *testing.T) {
	tests := []struct {
		name                     string
		ignoreAppPreflights      bool
		mockInstallFunc          func(t *testing.T, ctx context.Context, ignoreAppPreflights bool) (apitypes.AppInstall, error)
		mockGetStatusFunc        func(ctx context.Context) (apitypes.AppInstall, error)
		expectError              bool
		expectedErrorMsg         string
		expectedProgressMessages []string
	}{
		{
			name:                "success",
			ignoreAppPreflights: false,
			mockInstallFunc: func(t *testing.T, ctx context.Context, ignoreAppPreflights bool) (apitypes.AppInstall, error) {
				t.Helper()
				assert.Equal(t, ignoreAppPreflights, false, "ignoreAppPreflights should be false in mock install function")
				return apitypes.AppInstall{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.AppInstall, error) {
				return apitypes.AppInstall{
					Status: apitypes.Status{
						State:       apitypes.StateSucceeded,
						Description: "Application installed",
					},
				}, nil
			},
			expectError: false,
			expectedProgressMessages: []string{
				"Installing application...",
				"Application is ready",
			},
		},
		{
			name:                "success with ignoreAppPreflights",
			ignoreAppPreflights: true,
			mockInstallFunc: func(t *testing.T, ctx context.Context, ignoreAppPreflights bool) (apitypes.AppInstall, error) {
				t.Helper()
				assert.Equal(t, ignoreAppPreflights, true, "ignoreAppPreflights should be true in mock install function")
				return apitypes.AppInstall{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.AppInstall, error) {
				return apitypes.AppInstall{
					Status: apitypes.Status{
						State:       apitypes.StateSucceeded,
						Description: "Application installed",
					},
				}, nil
			},
			expectError: false,
			expectedProgressMessages: []string{
				"Installing application...",
				"Application is ready",
			},
		},
		{
			name:                "installation failure",
			ignoreAppPreflights: false,
			mockInstallFunc: func(t *testing.T, ctx context.Context, ignoreAppPreflights bool) (apitypes.AppInstall, error) {
				return apitypes.AppInstall{
					Status: apitypes.Status{State: apitypes.StatePending},
				}, nil
			},
			mockGetStatusFunc: func(ctx context.Context) (apitypes.AppInstall, error) {
				return apitypes.AppInstall{
					Status: apitypes.Status{
						State:       apitypes.StateFailed,
						Description: "timeout waiting for pods to become ready",
					},
				}, nil
			},
			expectError:      true,
			expectedErrorMsg: "application installation failed: timeout waiting for pods to become ready",
			expectedProgressMessages: []string{
				"Installing application...",
				"Application installation failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockAPIClient{
				installLinuxAppFunc: func(ctx context.Context, ignoreAppPreflights bool) (apitypes.AppInstall, error) {
					return tt.mockInstallFunc(t, ctx, ignoreAppPreflights)
				},
				getLinuxAppInstallStatusFunc: tt.mockGetStatusFunc,
			}

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			err := orchestrator.installApp(context.Background(), tt.ignoreAppPreflights)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages())
		})
	}
}

// progressMessageCapture captures progress messages from a spinner.WriteFn for testing.
// It parses spinner output (which includes ANSI codes and symbols like ○, ✔, ✗) and
// extracts just the message text. This allows tests to verify user-visible progress
// messages without dealing with terminal formatting codes.
type progressMessageCapture struct {
	messages    []string
	lastMessage string
}

// newProgressMessageCapture creates a new progress message capture helper.
// Usage:
//
//	capture := newProgressMessageCapture()
//	orchestrator := &orchestrator{
//	    progressWriter: capture.Writer(),
//	}
//	// ... perform operations that display progress ...
//	assert.Equal(t, expectedMessages, capture.Messages())
func newProgressMessageCapture() *progressMessageCapture {
	return &progressMessageCapture{
		messages: make([]string, 0),
	}
}

// Writer returns a spinner.WriteFn that captures progress messages.
// The returned function strips ANSI escape codes and spinner symbols (○, ✔, ✗)
// to extract just the message text. It deduplicates consecutive identical messages
// to avoid capturing spinner animation frames.
func (p *progressMessageCapture) Writer() spinner.WriteFn {
	return func(format string, args ...any) (int, error) {
		// Remove ANSI escape codes
		cleanFormat := strings.ReplaceAll(format, "\033[K\r", "")

		// Format the string with arguments
		var formatted string
		if len(args) > 0 {
			formatted = strings.TrimSpace(fmt.Sprintf(cleanFormat, args...))
		} else {
			formatted = strings.TrimSpace(cleanFormat)
		}

		// Extract just the message part (skip spinner/checkmark symbols)
		// Format is: "○  message" or "✔  message" or "✗  message"
		parts := strings.SplitN(formatted, "  ", 2)
		if len(parts) == 2 {
			msg := strings.TrimSpace(parts[1])
			// Only add if message changed and it's not empty
			if msg != p.lastMessage && msg != "" {
				p.messages = append(p.messages, msg)
				p.lastMessage = msg
			}
		}
		return 0, nil
	}
}

// Messages returns all captured progress messages
func (p *progressMessageCapture) Messages() []string {
	return p.messages
}
