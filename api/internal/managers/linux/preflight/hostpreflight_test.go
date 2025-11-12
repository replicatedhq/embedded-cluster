package preflight

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	preflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func Test_buildPrepareHostPreflightOptions(t *testing.T) {
	tests := []struct {
		name        string
		opts        PrepareHostPreflightOptions
		runtimeSpec *ecv1beta1.RuntimeConfigSpec
		nodeIP      string
		assertOpts  func(t *testing.T, result preflights.PrepareHostPreflightOptions)
	}{
		{
			name: "with proxy configuration and global CIDR",
			opts: PrepareHostPreflightOptions{
				ReplicatedAppURL:       "https://replicated.app",
				ProxyRegistryURL:       "proxy.registry.url",
				HostPreflightSpec:      &troubleshootv1beta2.HostPreflightSpec{},
				EmbeddedClusterConfig:  &ecv1beta1.Config{},
				TCPConnectionsRequired: []string{"6443", "2379"},
				IsAirgap:               false,
				IsJoin:                 false,
				IsUI:                   true,
			},
			runtimeSpec: &ecv1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
				AdminConsole: ecv1beta1.AdminConsoleSpec{
					Port: 30000,
				},
				LocalArtifactMirror: ecv1beta1.LocalArtifactMirrorSpec{
					Port: 50000,
				},
				Network: ecv1beta1.NetworkSpec{
					NetworkInterface: "eth0",
					PodCIDR:          "10.244.0.0/16",
					ServiceCIDR:      "10.96.0.0/12",
					GlobalCIDR:       "10.128.0.0/16",
					NodePortRange:    "80-32767",
				},
				Proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy:8080",
					HTTPSProxy: "https://proxy:8080",
					NoProxy:    "localhost,127.0.0.1",
				},
			},
			nodeIP: "192.0.100.1",
			assertOpts: func(t *testing.T, result preflights.PrepareHostPreflightOptions) {
				assert.Equal(t, "https://replicated.app", result.ReplicatedAppURL)
				assert.Equal(t, "proxy.registry.url", result.ProxyRegistryURL)
				assert.Equal(t, 30000, result.AdminConsolePort)
				assert.Equal(t, 50000, result.LocalArtifactMirrorPort)
				assert.Equal(t, "/var/lib/embedded-cluster", result.DataDir)
				assert.Equal(t, "/var/lib/embedded-cluster/k0s", result.K0sDataDir)
				assert.Equal(t, "/var/lib/embedded-cluster/openebs-local", result.OpenEBSDataDir)
				assert.Equal(t, "10.244.0.0/16", result.PodCIDR)
				assert.Equal(t, "10.96.0.0/12", result.ServiceCIDR)
				assert.NotNil(t, result.GlobalCIDR)
				assert.Equal(t, "10.128.0.0/16", *result.GlobalCIDR)
				assert.NotNil(t, result.Proxy)
				assert.Equal(t, "http://proxy:8080", result.Proxy.HTTPProxy)
				assert.Equal(t, "https://proxy:8080", result.Proxy.HTTPSProxy)
				assert.Equal(t, "localhost,127.0.0.1", result.Proxy.NoProxy)
				assert.Equal(t, "192.0.100.1", result.NodeIP)
				assert.False(t, result.IsAirgap)
				assert.False(t, result.IsJoin)
				assert.True(t, result.IsUI)
				assert.Equal(t, []string{"6443", "2379"}, result.TCPConnectionsRequired)
			},
		},
		{
			name: "without proxy configuration and without global CIDR",
			opts: PrepareHostPreflightOptions{
				ReplicatedAppURL:       "https://replicated.app",
				HostPreflightSpec:      &troubleshootv1beta2.HostPreflightSpec{},
				TCPConnectionsRequired: []string{"6443"},
				IsAirgap:               true,
				IsJoin:                 true,
			},
			runtimeSpec: &ecv1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
				AdminConsole: ecv1beta1.AdminConsoleSpec{
					Port: 30000,
				},
				LocalArtifactMirror: ecv1beta1.LocalArtifactMirrorSpec{
					Port: 50000,
				},
				Network: ecv1beta1.NetworkSpec{
					NetworkInterface: "eth0",
					PodCIDR:          "10.244.0.0/16",
					ServiceCIDR:      "10.96.0.0/12",
					NodePortRange:    "80-32767",
				},
			},
			nodeIP: "192.0.100.1",
			assertOpts: func(t *testing.T, result preflights.PrepareHostPreflightOptions) {
				assert.Nil(t, result.Proxy)
				assert.Nil(t, result.GlobalCIDR)
				assert.True(t, result.IsAirgap)
				assert.True(t, result.IsJoin)
				assert.Equal(t, "192.0.100.1", result.NodeIP)
				assert.Equal(t, []string{"6443"}, result.TCPConnectionsRequired)
			},
		},
		{
			name: "with custom k0s and openebs data dirs",
			opts: PrepareHostPreflightOptions{
				ReplicatedAppURL:  "https://replicated.app",
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			runtimeSpec: &ecv1beta1.RuntimeConfigSpec{
				DataDir:                "/custom/data",
				K0sDataDirOverride:     "/custom/k0s",
				OpenEBSDataDirOverride: "/custom/openebs",
				AdminConsole: ecv1beta1.AdminConsoleSpec{
					Port: 30000,
				},
				LocalArtifactMirror: ecv1beta1.LocalArtifactMirrorSpec{
					Port: 50000,
				},
				Network: ecv1beta1.NetworkSpec{
					NetworkInterface: "eth0",
					PodCIDR:          "10.244.0.0/16",
					ServiceCIDR:      "10.96.0.0/12",
					NodePortRange:    "80-32767",
				},
			},
			nodeIP: "192.0.100.1",
			assertOpts: func(t *testing.T, result preflights.PrepareHostPreflightOptions) {
				assert.Equal(t, "/custom/data", result.DataDir)
				assert.Equal(t, "/custom/k0s", result.K0sDataDir)
				assert.Equal(t, "/custom/openebs", result.OpenEBSDataDir)
			},
		},
		{
			name: "with airgap storage space calculation",
			opts: PrepareHostPreflightOptions{
				ReplicatedAppURL:  "https://replicated.app",
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
				IsAirgap:          true,
				AirgapInfo: &kotsv1beta1.Airgap{
					Spec: kotsv1beta1.AirgapSpec{
						UncompressedSize: 1024 * 1024 * 1024, // 1Gi
					},
				},
				EmbeddedAssetsSize: 500 * 1024 * 1024, // 500Mi
			},
			runtimeSpec: &ecv1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
				AdminConsole: ecv1beta1.AdminConsoleSpec{
					Port: 30000,
				},
				LocalArtifactMirror: ecv1beta1.LocalArtifactMirrorSpec{
					Port: 50000,
				},
				Network: ecv1beta1.NetworkSpec{
					NetworkInterface: "eth0",
					PodCIDR:          "10.244.0.0/16",
					ServiceCIDR:      "10.96.0.0/12",
					NodePortRange:    "80-32767",
				},
			},
			nodeIP: "192.0.100.1",
			assertOpts: func(t *testing.T, result preflights.PrepareHostPreflightOptions) {
				assert.NotEmpty(t, result.ControllerAirgapStorageSpace)
				assert.True(t, result.IsAirgap)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create real runtime config
			rc := runtimeconfig.New(tt.runtimeSpec)

			// Execute
			result := buildPrepareHostPreflightOptions(rc, tt.opts, tt.nodeIP)

			// Assert common fields
			assert.Equal(t, tt.opts.HostPreflightSpec, result.HostPreflightSpec)

			// Run custom assertions
			if tt.assertOpts != nil {
				tt.assertOpts(t, result)
			}
		})
	}
}

func TestHostPreflightManager_RunHostPreflights(t *testing.T) {
	tests := []struct {
		name               string
		opts               RunHostPreflightOptions
		initialState       types.HostPreflights
		setupMocks         func(*preflights.MockPreflightRunner, runtimeconfig.RuntimeConfig)
		expectedFinalState types.State
		// This is the expected error message returned by the RunHostPreflights method, synchronously
		expectedError string
	}{
		{
			name: "successful execution with no failures or warnings",
			opts: RunHostPreflightOptions{
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			initialState: types.HostPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner, rc runtimeconfig.RuntimeConfig) {
				// Mock successful preflight execution
				output := &types.PreflightsOutput{}
				runner.On("RunHostPreflights", mock.Anything, mock.Anything, preflightRunOptionsFromRC(rc)).Return(output, "", nil)
			},
			expectedFinalState: types.StateSucceeded,
		},
		{
			name: "execution with preflight failures",
			opts: RunHostPreflightOptions{
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			initialState: types.HostPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner, rc runtimeconfig.RuntimeConfig) {
				// Mock failed preflight execution
				output := &types.PreflightsOutput{
					Fail: []types.PreflightsRecord{{
						Title:   "Sample Failure",
						Message: "This is a sample failure message.",
					}},
				}

				runner.On("RunHostPreflights", mock.Anything, mock.Anything, preflightRunOptionsFromRC(rc)).Return(output, "", nil)
			},
			expectedFinalState: types.StateFailed,
		},
		{
			name: "execution with preflight warnings",
			initialState: types.HostPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunHostPreflightOptions{
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner, rc runtimeconfig.RuntimeConfig) {
				// Mock preflight execution with warnings
				output := &types.PreflightsOutput{
					Warn: []types.PreflightsRecord{{
						Title:   "Sample Warning",
						Message: "This is a sample warning message.",
					}},
				}

				runner.On("RunHostPreflights", mock.Anything, mock.Anything, preflightRunOptionsFromRC(rc)).Return(output, "", nil)
			},
			expectedFinalState: types.StateSucceeded,
		},
		{
			name: "execution with both failures and warnings",
			initialState: types.HostPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunHostPreflightOptions{
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner, rc runtimeconfig.RuntimeConfig) {
				// Mock preflight execution with both failures and warnings
				output := &types.PreflightsOutput{
					Fail: []types.PreflightsRecord{{
						Title:   "Sample Failure",
						Message: "This is a sample failure message.",
					}},
					Warn: []types.PreflightsRecord{{
						Title:   "Sample Warning",
						Message: "This is a sample warning message.",
					}},
				}

				runner.On("RunHostPreflights", mock.Anything, mock.Anything, preflightRunOptionsFromRC(rc)).Return(output, "", nil)
			},
			expectedFinalState: types.StateFailed,
		},
		{
			name: "runner execution fails",
			initialState: types.HostPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunHostPreflightOptions{
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner, rc runtimeconfig.RuntimeConfig) {
				// Mock runner failure
				runner.On("RunHostPreflights", mock.Anything, mock.Anything, preflightRunOptionsFromRC(rc)).Return(nil, "stderr output", assert.AnError)
			},
			expectedFinalState: types.StateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRunner := &preflights.MockPreflightRunner{}

			// Create runtime config
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())

			tt.setupMocks(mockRunner, rc)

			// Create manager using builder pattern
			manager := NewHostPreflightManager(
				WithPreflightRunner(mockRunner),
				WithHostPreflightStore(preflightstore.NewMemoryStore(preflightstore.WithHostPreflight(tt.initialState))),
				WithLogger(logger.NewDiscardLogger()),
			)

			// Execute
			err := manager.RunHostPreflights(context.Background(), rc, tt.opts)
			// If there's an error we don't need to wait for async execution
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			status, err := manager.GetHostPreflightStatus(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.expectedFinalState, status.State)

			// Additional verification that calls were made in the correct order
			mockRunner.AssertExpectations(t)
		})
	}
}

func TestHostPreflightManager_GetHostPreflightStatus(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*preflightstore.MockStore)
		expectedStatus types.Status
		expectedError  string
	}{
		{
			name: "success",
			setupMocks: func(store *preflightstore.MockStore) {
				store.On("GetStatus").Return(types.Status{
					State:       types.StateSucceeded,
					Description: "Host preflights passed",
					LastUpdated: time.Now(),
				}, nil)
			},
			expectedStatus: types.Status{
				State:       types.StateSucceeded,
				Description: "Host preflights passed",
			},
		},
		{
			name: "error from store",
			setupMocks: func(store *preflightstore.MockStore) {
				store.On("GetStatus").Return(nil, fmt.Errorf("store error"))
			},
			expectedError: "store error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := &preflightstore.MockStore{}
			tt.setupMocks(mockStore)

			// Create manager using builder pattern
			manager := NewHostPreflightManager(
				WithHostPreflightStore(mockStore),
			)

			// Execute
			status, err := manager.GetHostPreflightStatus(context.Background())

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Equal(t, types.Status{}, status)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus.State, status.State)
				assert.Equal(t, tt.expectedStatus.Description, status.Description)
			}

			// Verify mocks
			mockStore.AssertExpectations(t)
		})
	}
}

func TestHostPreflightManager_GetHostPreflightOutput(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*preflightstore.MockStore)
		expectedOutput *types.PreflightsOutput
		expectedError  string
	}{
		{
			name: "success",
			setupMocks: func(store *preflightstore.MockStore) {
				output := &types.PreflightsOutput{}
				store.On("GetOutput").Return(output, nil)
			},
			expectedOutput: &types.PreflightsOutput{},
		},
		{
			name: "error from store",
			setupMocks: func(store *preflightstore.MockStore) {
				store.On("GetOutput").Return(nil, fmt.Errorf("store error"))
			},
			expectedError: "store error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := &preflightstore.MockStore{}
			tt.setupMocks(mockStore)

			// Create manager using builder pattern
			manager := NewHostPreflightManager(
				WithHostPreflightStore(mockStore),
			)

			// Execute
			output, err := manager.GetHostPreflightOutput(context.Background())

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, output)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, output)
			}

			// Verify mocks
			mockStore.AssertExpectations(t)
		})
	}
}

func TestHostPreflightManager_GetHostPreflightTitles(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*preflightstore.MockStore)
		expectedTitles []string
		expectedError  string
	}{
		{
			name: "success",
			setupMocks: func(store *preflightstore.MockStore) {
				titles := []string{"Memory Check", "Disk Space Check", "Network Check"}
				store.On("GetTitles").Return(titles, nil)
			},
			expectedTitles: []string{"Memory Check", "Disk Space Check", "Network Check"},
		},
		{
			name: "success with empty titles",
			setupMocks: func(store *preflightstore.MockStore) {
				store.On("GetTitles").Return([]string{}, nil)
			},
			expectedTitles: []string{},
		},
		{
			name: "error from store",
			setupMocks: func(store *preflightstore.MockStore) {
				store.On("GetTitles").Return(nil, fmt.Errorf("store error"))
			},
			expectedError: "store error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := &preflightstore.MockStore{}
			tt.setupMocks(mockStore)

			// Create manager using builder pattern
			manager := NewHostPreflightManager(
				WithHostPreflightStore(mockStore),
			)

			// Execute
			titles, err := manager.GetHostPreflightTitles(context.Background())

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, titles)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTitles, titles)
			}

			// Verify mocks
			mockStore.AssertExpectations(t)
		})
	}
}

func TestHostPreflightManager_ClearHostPreflightResults(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*preflightstore.MockStore)
		expectedError string
	}{
		{
			name: "success",
			setupMocks: func(store *preflightstore.MockStore) {
				store.On("Clear").Return(nil)
			},
		},
		{
			name: "error from store",
			setupMocks: func(store *preflightstore.MockStore) {
				store.On("Clear").Return(fmt.Errorf("clear error"))
			},
			expectedError: "clear error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStore := &preflightstore.MockStore{}
			tt.setupMocks(mockStore)

			// Create manager using builder pattern
			manager := NewHostPreflightManager(
				WithHostPreflightStore(mockStore),
			)

			// Execute
			err := manager.ClearHostPreflightResults(context.Background())

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			// Verify mocks
			mockStore.AssertExpectations(t)
		})
	}
}

func preflightRunOptionsFromRC(rc runtimeconfig.RuntimeConfig) preflights.RunOptions {
	return preflights.RunOptions{
		PreflightBinaryPath: rc.PathToEmbeddedClusterBinary("kubectl-preflight"),
		ProxySpec:           rc.ProxySpec(),
		ExtraPaths:          []string{rc.EmbeddedClusterBinsSubDir()},
	}
}
