package install

import (
	"errors"
	"testing"
	"time"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInstallController_SetAppConfigValues(t *testing.T) {
	// Create an app config for testing
	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "test-group",
					Title: "Test Group",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "test-item",
							Type:    "text",
							Title:   "Test Item",
							Default: multitype.BoolOrString{StrVal: "default"},
							Value:   multitype.BoolOrString{StrVal: "value"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		values        map[string]string
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*appconfig.MockAppConfigManager, *store.MockStore)
		expectedErr   bool
	}{
		{
			name: "successful set app config values",
			values: map[string]string{
				"test-item": "new-value",
			},
			currentState:  StateNew,
			expectedState: StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(nil),
					am.On("SetConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful set app config values from application configuration failed state",
			values: map[string]string{
				"test-item": "new-value",
			},
			currentState:  StateApplicationConfigurationFailed,
			expectedState: StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(nil),
					am.On("SetConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful set app config values from application configured state",
			values: map[string]string{
				"test-item": "new-value",
			},
			currentState:  StateApplicationConfigured,
			expectedState: StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(nil),
					am.On("SetConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "validation error",
			values: map[string]string{
				"test-item": "invalid-value",
			},
			currentState:  StateNew,
			expectedState: StateApplicationConfigurationFailed,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", appConfig, map[string]string{"test-item": "invalid-value"}).Return(errors.New("validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "set config values error",
			values: map[string]string{
				"test-item": "new-value",
			},
			currentState:  StateNew,
			expectedState: StateApplicationConfigurationFailed,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(nil),
					am.On("SetConfigValues", appConfig, map[string]string{"test-item": "new-value"}).Return(errors.New("set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "invalid state transition",
			values: map[string]string{
				"test-item": "new-value",
			},
			currentState:  StateInfrastructureInstalling,
			expectedState: StateInfrastructureInstalling,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
			},
			expectedErr: true,
		},
		{
			name: "app config not found",
			values: map[string]string{
				"test-item": "new-value",
			},
			currentState:  StateNew,
			expectedState: StateNew,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStateMachine(WithCurrentState(tt.currentState))

			mockAppConfigManager := &appconfig.MockAppConfigManager{}
			mockStore := &store.MockStore{}

			tt.setupMocks(mockAppConfigManager, mockStore)

			// For the "app config not found" test case, pass nil as AppConfig
			var releaseData *release.ReleaseData
			if tt.name == "app config not found" {
				releaseData = getTestReleaseData(nil)
			} else {
				releaseData = getTestReleaseData(&appConfig)
			}

			controller, err := NewInstallController(
				WithStateMachine(sm),
				WithAppConfigManager(mockAppConfigManager),
				WithReleaseData(releaseData),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			err = controller.SetAppConfigValues(t.Context(), tt.values)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())
			assert.False(t, sm.IsLockAcquired(), "state machine should not be locked after setting app config values")

			mockAppConfigManager.AssertExpectations(t)
			mockStore.LinuxInfraMockStore.AssertExpectations(t)
			mockStore.LinuxInstallationMockStore.AssertExpectations(t)
			mockStore.LinuxPreflightMockStore.AssertExpectations(t)
			mockStore.AppConfigMockStore.AssertExpectations(t)
		})
	}
}

func TestInstallController_GetAppConfigValues(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*appconfig.MockAppConfigManager, *store.MockStore)
		expectedValues map[string]string
		expectedErr    bool
	}{
		{
			name: "successful get app config values",
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				expectedValues := map[string]string{
					"test-item":    "test-value",
					"another-item": "another-value",
				}
				am.On("GetConfigValues").Return(expectedValues, nil)
			},
			expectedValues: map[string]string{
				"test-item":    "test-value",
				"another-item": "another-value",
			},
			expectedErr: false,
		},
		{
			name: "get config values error",
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				am.On("GetConfigValues").Return(nil, errors.New("get config values error"))
			},
			expectedValues: nil,
			expectedErr:    true,
		},
		{
			name: "empty config values",
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				am.On("GetConfigValues").Return(map[string]string{}, nil)
			},
			expectedValues: map[string]string{},
			expectedErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAppConfigManager := &appconfig.MockAppConfigManager{}
			mockStore := &store.MockStore{}

			tt.setupMocks(mockAppConfigManager, mockStore)

			controller, err := NewInstallController(
				WithAppConfigManager(mockAppConfigManager),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			values, err := controller.GetAppConfigValues(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, values)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValues, values)
			}

			mockAppConfigManager.AssertExpectations(t)
			mockStore.LinuxInfraMockStore.AssertExpectations(t)
			mockStore.LinuxInstallationMockStore.AssertExpectations(t)
			mockStore.LinuxPreflightMockStore.AssertExpectations(t)
			mockStore.AppConfigMockStore.AssertExpectations(t)
		})
	}
}
