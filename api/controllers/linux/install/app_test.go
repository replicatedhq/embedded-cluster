package install

import (
	"errors"
	"testing"
	"time"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInstallController_PatchAppConfigValues(t *testing.T) {
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		values        types.AppConfigValues
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*appconfig.MockAppConfigManager, *store.MockStore)
		expectedErr   bool
	}{
		{
			name: "successful set app config values",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-value"},
			},
			currentState:  StateNew,
			expectedState: StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful set app config values from application configuration failed state",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-value"},
			},
			currentState:  StateApplicationConfigurationFailed,
			expectedState: StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful set app config values from application configured state",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-value"},
			},
			currentState:  StateApplicationConfigured,
			expectedState: StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "validation error",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "invalid-value"},
			},
			currentState:  StateNew,
			expectedState: StateApplicationConfigurationFailed,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "invalid-value"}}).Return(errors.New("validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "set config values error",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-value"},
			},
			currentState:  StateNew,
			expectedState: StateApplicationConfigurationFailed,
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-value"}}).Return(errors.New("set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "invalid state transition",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-value"},
			},
			currentState:  StateInfrastructureInstalling,
			expectedState: StateInfrastructureInstalling,
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

			controller, err := NewInstallController(
				WithStateMachine(sm),
				WithAppConfigManager(mockAppConfigManager),
				WithReleaseData(getTestReleaseData(&appConfig)),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			err = controller.PatchAppConfigValues(t.Context(), tt.values)

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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		setupMocks     func(*appconfig.MockAppConfigManager, *store.MockStore)
		expectedValues types.AppConfigValues
		expectedErr    bool
	}{
		{
			name: "successful get app config values",
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				expectedValues := types.AppConfigValues{
					"test-item":    types.AppConfigValue{Value: "value"},
					"another-item": types.AppConfigValue{Value: "another-value"},
				}
				am.On("GetConfigValues", false).Return(expectedValues, nil)
			},
			expectedValues: types.AppConfigValues{
				"test-item":    types.AppConfigValue{Value: "value"},
				"another-item": types.AppConfigValue{Value: "another-value"},
			},
			expectedErr: false,
		},
		{
			name: "get config values error",
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				am.On("GetConfigValues", false).Return(nil, errors.New("get config values error"))
			},
			expectedValues: nil,
			expectedErr:    true,
		},
		{
			name: "empty config values",
			setupMocks: func(am *appconfig.MockAppConfigManager, st *store.MockStore) {
				am.On("GetConfigValues", false).Return(types.AppConfigValues{}, nil)
			},
			expectedValues: types.AppConfigValues{},
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
				WithReleaseData(getTestReleaseData(&appConfig)),
			)
			require.NoError(t, err)

			values, err := controller.GetAppConfigValues(t.Context(), false)

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
