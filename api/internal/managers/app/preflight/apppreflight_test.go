package preflight

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	preflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func TestAppPreflightManager_RunAppPreflights(t *testing.T) {
	tests := []struct {
		name               string
		opts               RunAppPreflightOptions
		initialState       types.AppPreflights
		setupMocks         func(*preflights.MockPreflightRunner)
		expectedFinalState types.State
		expectedError      string
	}{
		{
			name: "successful execution with no failures or warnings",
			opts: RunAppPreflightOptions{
				AppPreflightSpec: &troubleshootv1beta2.PreflightSpec{},
				RunOptions: preflights.RunOptions{
					PreflightBinaryPath: "/usr/local/bin/kubectl-preflight",
					ExtraPaths:          []string{"/usr/local/bin"},
				},
			},
			initialState: types.AppPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner) {
				output := &types.PreflightsOutput{}
				runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(output, "", nil)
			},
			expectedFinalState: types.StateSucceeded,
		},
		{
			name: "execution with preflight failures",
			opts: RunAppPreflightOptions{
				AppPreflightSpec: &troubleshootv1beta2.PreflightSpec{},
				RunOptions: preflights.RunOptions{
					PreflightBinaryPath: "/usr/local/bin/kubectl-preflight",
				},
			},
			initialState: types.AppPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner) {
				output := &types.PreflightsOutput{
					Fail: []types.PreflightsRecord{{
						Title:   "RBAC Check Failed",
						Message: "Insufficient RBAC permissions.",
					}},
				}
				runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(output, "", nil)
			},
			expectedFinalState: types.StateFailed,
		},
		{
			name: "execution with preflight warnings",
			initialState: types.AppPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunAppPreflightOptions{
				AppPreflightSpec: &troubleshootv1beta2.PreflightSpec{},
				RunOptions: preflights.RunOptions{
					PreflightBinaryPath: "/usr/local/bin/kubectl-preflight",
				},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner) {
				output := &types.PreflightsOutput{
					Warn: []types.PreflightsRecord{{
						Title:   "Image Pull Warning",
						Message: "Some images may take longer to pull.",
					}},
				}
				runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(output, "", nil)
			},
			expectedFinalState: types.StateSucceeded,
		},
		{
			name: "execution with both failures and warnings",
			initialState: types.AppPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunAppPreflightOptions{
				AppPreflightSpec: &troubleshootv1beta2.PreflightSpec{},
				RunOptions: preflights.RunOptions{
					PreflightBinaryPath: "/usr/local/bin/kubectl-preflight",
				},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner) {
				output := &types.PreflightsOutput{
					Fail: []types.PreflightsRecord{{
						Title:   "Connectivity Check Failed",
						Message: "Cannot reach required services.",
					}},
					Warn: []types.PreflightsRecord{{
						Title:   "Performance Warning",
						Message: "Performance may be degraded.",
					}},
				}
				runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(output, "", nil)
			},
			expectedFinalState: types.StateFailed,
		},
		{
			name: "runner execution fails",
			initialState: types.AppPreflights{
				Status: types.Status{
					State: types.StatePending,
				},
			},
			opts: RunAppPreflightOptions{
				AppPreflightSpec: &troubleshootv1beta2.PreflightSpec{},
				RunOptions: preflights.RunOptions{
					PreflightBinaryPath: "/usr/local/bin/kubectl-preflight",
				},
			},
			setupMocks: func(runner *preflights.MockPreflightRunner) {
				runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(nil, "stderr output", assert.AnError)
			},
			expectedFinalState: types.StateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRunner := &preflights.MockPreflightRunner{}
			tt.setupMocks(mockRunner)

			// Create manager using builder pattern
			manager := NewAppPreflightManager(
				WithPreflightRunner(mockRunner),
				WithAppPreflightStore(preflightstore.NewMemoryStore(preflightstore.WithAppPreflight(tt.initialState))),
				WithLogger(logger.NewDiscardLogger()),
			)

			// Execute
			err := manager.RunAppPreflights(context.Background(), tt.opts)
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			status, err := manager.GetAppPreflightStatus(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.expectedFinalState, status.State)

			// Verify mocks
			mockRunner.AssertExpectations(t)
		})
	}
}

func TestAppPreflightManager_GetAppPreflightStatus(t *testing.T) {
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
					Description: "App preflights passed",
					LastUpdated: time.Now(),
				}, nil)
			},
			expectedStatus: types.Status{
				State:       types.StateSucceeded,
				Description: "App preflights passed",
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
			manager := NewAppPreflightManager(
				WithAppPreflightStore(mockStore),
			)

			// Execute
			status, err := manager.GetAppPreflightStatus(context.Background())

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

func TestAppPreflightManager_GetAppPreflightOutput(t *testing.T) {
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
			manager := NewAppPreflightManager(
				WithAppPreflightStore(mockStore),
			)

			// Execute
			output, err := manager.GetAppPreflightOutput(context.Background())

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

func TestAppPreflightManager_GetAppPreflightTitles(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*preflightstore.MockStore)
		expectedTitles []string
		expectedError  string
	}{
		{
			name: "success",
			setupMocks: func(store *preflightstore.MockStore) {
				titles := []string{"RBAC Check", "Image Pull Check", "Connectivity Check"}
				store.On("GetTitles").Return(titles, nil)
			},
			expectedTitles: []string{"RBAC Check", "Image Pull Check", "Connectivity Check"},
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
			manager := NewAppPreflightManager(
				WithAppPreflightStore(mockStore),
			)

			// Execute
			titles, err := manager.GetAppPreflightTitles(context.Background())

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

func TestAppPreflightManager_getTitles(t *testing.T) {
	tests := []struct {
		name           string
		spec           *troubleshootv1beta2.PreflightSpec
		expectedTitles []string
	}{
		{
			name:           "nil spec returns nil",
			spec:           nil,
			expectedTitles: nil,
		},
		{
			name: "spec with nil analyzers returns nil",
			spec: &troubleshootv1beta2.PreflightSpec{
				Analyzers: nil,
			},
			expectedTitles: nil,
		},
		{
			name: "spec with empty analyzers returns empty slice",
			spec: &troubleshootv1beta2.PreflightSpec{
				Analyzers: []*troubleshootv1beta2.Analyze{},
			},
			expectedTitles: []string{},
		},
		{
			name: "spec with valid analyzers returns titles",
			spec: &troubleshootv1beta2.PreflightSpec{
				Analyzers: []*troubleshootv1beta2.Analyze{
					{
						ClusterVersion: &troubleshootv1beta2.ClusterVersion{
							AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
								CheckName: "Kubernetes Version Check",
							},
							Outcomes: []*troubleshootv1beta2.Outcome{
								{
									Pass: &troubleshootv1beta2.SingleOutcome{
										Message: "Kubernetes version is supported",
									},
								},
							},
						},
					},
					{
						NodeResources: &troubleshootv1beta2.NodeResources{
							AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
								CheckName: "Node Resources Check",
							},
							Filters: &troubleshootv1beta2.NodeResourceFilters{
								CPUCapacity:    "2",
								MemoryCapacity: "4Gi",
							},
							Outcomes: []*troubleshootv1beta2.Outcome{
								{
									Pass: &troubleshootv1beta2.SingleOutcome{
										Message: "Node has sufficient resources",
									},
								},
							},
						},
					},
				},
			},
			expectedTitles: []string{"Kubernetes Version Check", "Node Resources Check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &appPreflightManager{
				logger: logger.NewDiscardLogger(),
			}

			titles, err := manager.getTitles(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTitles, titles)
		})
	}
}
