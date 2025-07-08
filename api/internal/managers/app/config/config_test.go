package config

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
)

func TestAppConfigManager_Get(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*configstore.MockStore)
		expectedError string
	}{
		{
			name: "successful get",
			setupMock: func(mockStore *configstore.MockStore) {
				config := kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{
						Groups: []kotsv1beta1.ConfigGroup{
							{
								Name: "settings",
								Items: []kotsv1beta1.ConfigItem{
									{
										Name:    "enable_feature",
										Type:    "bool",
										Default: multitype.FromString("0"),
									},
								},
							},
						},
					},
				}
				mockStore.On("Get").Return(config, nil)
			},
			expectedError: "",
		},
		{
			name: "store error",
			setupMock: func(mockStore *configstore.MockStore) {
				mockStore.On("Get").Return(kotsv1beta1.Config{}, errors.New("store error"))
			},
			expectedError: "store error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &configstore.MockStore{}
			tt.setupMock(mockStore)

			manager := NewAppConfigManager(
				WithAppConfigStore(mockStore),
				WithLogger(logger.NewDiscardLogger()),
			)

			result, err := manager.Get()

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestAppConfigManager_Set(t *testing.T) {
	manager := NewAppConfigManager()
	ctx := context.Background()

	// Set currently returns nil - testing the current behavior
	err := manager.Set(ctx)
	assert.NoError(t, err)
}

func TestRenderAppConfigValues(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*configstore.MockStore)
		expectedError string
		validateFunc  func(*testing.T, kotsv1beta1.ConfigValues)
	}{
		{
			name: "successful conversion with boolean fields",
			setupMock: func(mockStore *configstore.MockStore) {
				config := kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{
						Groups: []kotsv1beta1.ConfigGroup{
							{
								Name: "settings",
								Items: []kotsv1beta1.ConfigItem{
									{
										Name:    "enable_feature",
										Type:    "bool",
										Default: multitype.FromString("0"),
										Value:   multitype.FromString("1"),
									},
									{
										Name:    "show_advanced",
										Type:    "bool",
										Default: multitype.FromString("1"),
									},
								},
							},
						},
					},
				}
				mockStore.On("Get").Return(config, nil)
			},
			expectedError: "",
			validateFunc: func(t *testing.T, configValues kotsv1beta1.ConfigValues) {
				assert.Equal(t, "kots.io/v1beta1", configValues.TypeMeta.APIVersion)
				assert.Equal(t, "ConfigValues", configValues.TypeMeta.Kind)
				assert.Equal(t, "app-config", configValues.ObjectMeta.Name)

				assert.Contains(t, configValues.Spec.Values, "enable_feature")
				assert.Equal(t, "1", configValues.Spec.Values["enable_feature"].Value)
				assert.Equal(t, "0", configValues.Spec.Values["enable_feature"].Default)

				assert.Contains(t, configValues.Spec.Values, "show_advanced")
				assert.Equal(t, "1", configValues.Spec.Values["show_advanced"].Value)
				assert.Equal(t, "1", configValues.Spec.Values["show_advanced"].Default)
			},
		},
		{
			name: "empty config",
			setupMock: func(mockStore *configstore.MockStore) {
				config := kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{
						Groups: []kotsv1beta1.ConfigGroup{},
					},
				}
				mockStore.On("Get").Return(config, nil)
			},
			expectedError: "",
			validateFunc: func(t *testing.T, configValues kotsv1beta1.ConfigValues) {
				assert.Equal(t, "kots.io/v1beta1", configValues.TypeMeta.APIVersion)
				assert.Equal(t, "ConfigValues", configValues.TypeMeta.Kind)
				assert.Equal(t, "app-config", configValues.ObjectMeta.Name)
				assert.Empty(t, configValues.Spec.Values)
			},
		},
		{
			name: "store error",
			setupMock: func(mockStore *configstore.MockStore) {
				mockStore.On("Get").Return(kotsv1beta1.Config{}, errors.New("store error"))
			},
			expectedError: "store error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &configstore.MockStore{}
			tt.setupMock(mockStore)

			manager := NewAppConfigManager(
				WithAppConfigStore(mockStore),
				WithLogger(logger.NewDiscardLogger()),
			)

			result, err := manager.RenderAppConfigValues()

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, result)
				}
			}

			mockStore.AssertExpectations(t)
		})
	}
}
