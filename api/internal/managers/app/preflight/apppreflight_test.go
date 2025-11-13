package preflight

import (
	"context"
	"fmt"
	"testing"
	"time"

	preflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAppPreflightManager_RunAppPreflights(t *testing.T) {
	tests := []struct {
		name           string
		opts           RunAppPreflightOptions
		initialState   types.AppPreflights
		setupMocks     func(*preflights.MockPreflightRunner)
		expectedError  string
		expectedTitles []string
		expectedOutput *types.PreflightsOutput
	}{
		{
			name: "successful execution",
			opts: RunAppPreflightOptions{
				AppPreflightSpec: &troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{
								AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
									CheckName: "Kubernetes Version Check",
								},
							},
						},
						{
							NodeResources: &troubleshootv1beta2.NodeResources{
								AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
									CheckName: "Node Resources Check",
								},
							},
						},
						{
							ImagePullSecret: &troubleshootv1beta2.ImagePullSecret{
								AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
									CheckName: "Image Pull Secret Check",
								},
							},
						},
					},
				},
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
				output := &types.PreflightsOutput{
					Pass: []types.PreflightsRecord{{
						Title:   "Kubernetes Version Check",
						Message: "Kubernetes version is supported",
					}},
					Warn: []types.PreflightsRecord{{
						Title:   "Node Resources Check",
						Message: "Node resources are below recommended levels",
					}},
					Fail: []types.PreflightsRecord{{
						Title:   "Image Pull Secret Check",
						Message: "Image pull secret is invalid",
					}},
				}
				runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(output, "", nil)
			},
			expectedTitles: []string{
				"Kubernetes Version Check",
				"Node Resources Check",
				"Image Pull Secret Check",
			},
			expectedOutput: &types.PreflightsOutput{
				Pass: []types.PreflightsRecord{{
					Title:   "Kubernetes Version Check",
					Message: "Kubernetes version is supported",
				}},
				Warn: []types.PreflightsRecord{{
					Title:   "Node Resources Check",
					Message: "Node resources are below recommended levels",
				}},
				Fail: []types.PreflightsRecord{{
					Title:   "Image Pull Secret Check",
					Message: "Image pull secret is invalid",
				}},
			},
		},
		{
			name: "runner execution fails",
			opts: RunAppPreflightOptions{
				AppPreflightSpec: &troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{
								AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
									CheckName: "Kubernetes Version Check",
								},
							},
						},
					},
				},
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
				runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(nil, "", fmt.Errorf("failed to execute preflights"))
			},
			expectedError: "failed to execute preflights",
			expectedTitles: []string{
				"Kubernetes Version Check",
			},
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
			output, err := manager.RunAppPreflights(t.Context(), tt.opts)
			if tt.expectedError != "" {
				require.Error(t, err, "expected error running app preflights")
				assert.Contains(t, err.Error(), tt.expectedError, "error mismatch")
			} else {
				require.NoError(t, err, "unexpected error running app preflights")
			}

			assert.Equal(t, tt.expectedOutput, output, "output mismatch")

			titles, err := manager.GetAppPreflightTitles(t.Context())
			require.NoError(t, err, "failed to get titles")
			assert.Equal(t, tt.expectedTitles, titles, "titles mismatch")

			output, err = manager.GetAppPreflightOutput(t.Context())
			require.NoError(t, err, "failed to get output")
			assert.Equal(t, tt.expectedOutput, output, "output mismatch")

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
