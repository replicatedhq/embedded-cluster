package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAppConfigManager_GetConfigValues(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*configstore.MockStore)
		expectedResult kotsv1beta1.ConfigValues
		expectedError  string
	}{
		{
			name: "successful conversion with boolean defaults",
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
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enable_feature": {
							Value:   "0",
							Default: "0",
						},
						"show_advanced": {
							Value:   "1",
							Default: "1",
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "handles missing defaults gracefully",
			setupMock: func(mockStore *configstore.MockStore) {
				config := kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{
						Groups: []kotsv1beta1.ConfigGroup{
							{
								Name: "settings",
								Items: []kotsv1beta1.ConfigItem{
									{
										Name: "enable_feature",
										Type: "bool",
										// No default field
									},
								},
							},
						},
					},
				}
				mockStore.On("Get").Return(config, nil)
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enable_feature": {
							Value:   "",
							Default: "",
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "preserves user-set values over defaults",
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
										Value:   multitype.FromString("1"), // User set to "1"
									},
								},
							},
						},
					},
				}
				mockStore.On("Get").Return(config, nil)
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enable_feature": {
							Value:   "1", // User's value takes precedence
							Default: "0", // Original default preserved
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "handles empty config gracefully",
			setupMock: func(mockStore *configstore.MockStore) {
				config := kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{
						Groups: []kotsv1beta1.ConfigGroup{},
					},
				}
				mockStore.On("Get").Return(config, nil)
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
			expectedError: "",
		},
		{
			name: "handles store error gracefully",
			setupMock: func(mockStore *configstore.MockStore) {
				mockStore.On("Get").Return(kotsv1beta1.Config{}, errors.New("store error"))
			},
			expectedResult: kotsv1beta1.ConfigValues{},
			expectedError:  "store error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh mock for each test case
			mockStore := &configstore.MockStore{}
			tt.setupMock(mockStore)

			manager := NewAppConfigManager(
				WithAppConfigStore(mockStore),
				WithLogger(logger.NewDiscardLogger()),
			)

			result, err := manager.GetConfigValues()

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Equal(t, tt.expectedResult, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockStore.AssertExpectations(t)
		})
	}
}
