package preflight

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	preflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func TestHostPreflightManager_PrepareHostPreflights(t *testing.T) {
	tests := []struct {
		name          string
		opts          PrepareHostPreflightOptions
		runtimeSpec   *ecv1beta1.RuntimeConfigSpec
		setupMocks    func(*preflights.MockPreflightRunner, *utils.MockNetUtils)
		expectedHPF   *troubleshootv1beta2.HostPreflightSpec
		expectedError string
		assertResult  func(t *testing.T, hpf *troubleshootv1beta2.HostPreflightSpec)
	}{
		{
			name: "success with proxy configuration",
			opts: PrepareHostPreflightOptions{
				ReplicatedAppURL:       "https://replicated.app",
				ProxyRegistryURL:       "proxy.registry.url",
				HostPreflightSpec:      &troubleshootv1beta2.HostPreflightSpec{},
				EmbeddedClusterConfig:  &ecv1beta1.Config{},
				TCPConnectionsRequired: []string{"6443", "2379"},
				IsAirgap:               false,
				IsJoin:                 false,
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
			setupMocks: func(runner *preflights.MockPreflightRunner, netUtils *utils.MockNetUtils) {
				netUtils.On("FirstValidAddress", "eth0").Return("192.0.100.1", nil)
				runner.On("Prepare", mock.Anything, mock.MatchedBy(func(opts preflights.PrepareOptions) bool {
					return opts.AdminConsolePort == 30000 &&
						opts.LocalArtifactMirrorPort == 50000 &&
						opts.DataDir == "/var/lib/embedded-cluster" &&
						opts.PodCIDR == "10.244.0.0/16" &&
						opts.ServiceCIDR == "10.96.0.0/12" &&
						*opts.GlobalCIDR == "10.128.0.0/16" &&
						opts.Proxy != nil &&
						opts.Proxy.HTTPProxy == "http://proxy:8080" &&
						!opts.IsAirgap &&
						!opts.IsJoin &&
						opts.K0sDataDir == "/var/lib/embedded-cluster/k0s" &&
						opts.OpenEBSDataDir == "/var/lib/embedded-cluster/openebs-local"
				})).Return(&troubleshootv1beta2.HostPreflightSpec{}, nil)
			},
			expectedHPF: &troubleshootv1beta2.HostPreflightSpec{},
		},
		{
			name: "success without proxy configuration",
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
			setupMocks: func(runner *preflights.MockPreflightRunner, netUtils *utils.MockNetUtils) {
				netUtils.On("FirstValidAddress", "eth0").Return("192.0.100.1", nil)
				runner.On("Prepare", mock.Anything, mock.MatchedBy(func(opts preflights.PrepareOptions) bool {
					return opts.Proxy == nil &&
						opts.GlobalCIDR == nil &&
						opts.IsAirgap &&
						opts.IsJoin
				})).Return(&troubleshootv1beta2.HostPreflightSpec{}, nil)
			},
			expectedHPF: &troubleshootv1beta2.HostPreflightSpec{},
		},
		{
			name: "success with custom k0s and openebs data dirs",
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
			setupMocks: func(runner *preflights.MockPreflightRunner, netUtils *utils.MockNetUtils) {
				netUtils.On("FirstValidAddress", "eth0").Return("192.0.100.1", nil)
				runner.On("Prepare", mock.Anything, mock.MatchedBy(func(opts preflights.PrepareOptions) bool {
					return opts.DataDir == "/custom/data" &&
						opts.K0sDataDir == "/custom/k0s" &&
						opts.OpenEBSDataDir == "/custom/openebs"
				})).Return(&troubleshootv1beta2.HostPreflightSpec{}, nil)
			},
			expectedHPF: &troubleshootv1beta2.HostPreflightSpec{},
		},
		{
			name: "error when runner prepare fails",
			opts: PrepareHostPreflightOptions{},
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
			setupMocks: func(runner *preflights.MockPreflightRunner, netUtils *utils.MockNetUtils) {
				netUtils.On("FirstValidAddress", "eth0").Return("192.0.100.1", nil)
				runner.On("Prepare", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("prepare failed"))
			},
			expectedError: "prepare host preflights: prepare failed",
		},
		{
			name: "error when determining the node IP fails",
			opts: PrepareHostPreflightOptions{},
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
			setupMocks: func(runner *preflights.MockPreflightRunner, netUtils *utils.MockNetUtils) {
				netUtils.On("FirstValidAddress", "eth0").Return("", assert.AnError)
			},
			expectedError: "determine node ip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRunner := &preflights.MockPreflightRunner{}
			mockStore := &preflightstore.MockStore{}
			mockNetUtils := &utils.MockNetUtils{}

			// Create real runtime config
			rc := runtimeconfig.New(tt.runtimeSpec)

			tt.setupMocks(mockRunner, mockNetUtils)

			// Create manager using builder pattern
			manager := NewHostPreflightManager(
				WithPreflightRunner(mockRunner),
				WithHostPreflightStore(mockStore),
				WithLogger(logger.NewDiscardLogger()),
				WithNetUtils(mockNetUtils),
			)

			// Execute
			hpf, err := manager.PrepareHostPreflights(context.Background(), rc, tt.opts)

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, hpf)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedHPF, hpf)

				if tt.assertResult != nil {
					tt.assertResult(t, hpf)
				}
			}

			// Verify mocks
			mockRunner.AssertExpectations(t)
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
				output := &types.HostPreflightsOutput{}
				runner.On("Run", mock.Anything, mock.Anything, rc).Return(output, "", nil)

				// Mock save operations in order
				runner.On("SaveToDisk", output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")).Return(nil)
				runner.On("CopyBundleTo", rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")).Return(nil)
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
				output := &types.HostPreflightsOutput{
					Fail: []types.HostPreflightsRecord{{
						Title:   "Sample Failure",
						Message: "This is a sample failure message.",
					}},
				}

				runner.On("Run", mock.Anything, mock.Anything, rc).Return(output, "", nil)

				// Mock save operations
				runner.On("SaveToDisk", output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")).Return(nil)
				runner.On("CopyBundleTo", rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")).Return(nil)
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
				output := &types.HostPreflightsOutput{
					Warn: []types.HostPreflightsRecord{{
						Title:   "Sample Warning",
						Message: "This is a sample warning message.",
					}},
				}
				runner.On("Run", mock.Anything, mock.Anything, rc).Return(output, "", nil)

				// Mock save operations
				runner.On("SaveToDisk", output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")).Return(nil)
				runner.On("CopyBundleTo", rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")).Return(nil)
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
				output := &types.HostPreflightsOutput{
					Fail: []types.HostPreflightsRecord{{
						Title:   "Sample Failure",
						Message: "This is a sample failure message.",
					}},
					Warn: []types.HostPreflightsRecord{{
						Title:   "Sample Warning",
						Message: "This is a sample warning message.",
					}},
				}
				runner.On("Run", mock.Anything, mock.Anything, rc).Return(output, "", nil)

				// Mock save operations
				runner.On("SaveToDisk", output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")).Return(nil)
				runner.On("CopyBundleTo", rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")).Return(nil)
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
				runner.On("Run", mock.Anything, mock.Anything, rc).Return(nil, "stderr output", assert.AnError)
			},
			expectedFinalState: types.StateFailed,
		},
		{
			name: "SaveToDisk fails but execution continues",
			initialState: types.HostPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunHostPreflightOptions{
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner, rc runtimeconfig.RuntimeConfig) {
				// Mock successful preflight execution
				output := &types.HostPreflightsOutput{}
				runner.On("Run", mock.Anything, mock.Anything, rc).Return(output, "", nil)

				// Mock save operations - SaveToDisk fails but execution continues
				runner.On("SaveToDisk", output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")).Return(assert.AnError)
				runner.On("CopyBundleTo", rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")).Return(nil)
			},
			expectedFinalState: types.StateSucceeded,
		},
		{
			name: "CopyBundleTo fails but execution continues",
			initialState: types.HostPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunHostPreflightOptions{
				HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner, rc runtimeconfig.RuntimeConfig) {
				// Mock successful preflight execution
				output := &types.HostPreflightsOutput{}
				runner.On("Run", mock.Anything, mock.Anything, rc).Return(output, "", nil)

				// Mock save operations - CopyBundleTo fails but execution continues
				runner.On("SaveToDisk", output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")).Return(nil)
				runner.On("CopyBundleTo", rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")).Return(assert.AnError)
			},
			expectedFinalState: types.StateSucceeded,
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
		expectedOutput *types.HostPreflightsOutput
		expectedError  string
	}{
		{
			name: "success",
			setupMocks: func(store *preflightstore.MockStore) {
				output := &types.HostPreflightsOutput{}
				store.On("GetOutput").Return(output, nil)
			},
			expectedOutput: &types.HostPreflightsOutput{},
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
