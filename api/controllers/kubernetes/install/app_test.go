package install

import (
	"context"
	"errors"
	"testing"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
)

func TestInstallController_GetAppConfig(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*appconfig.MockAppConfigManager)
		expectedConfig kotsv1beta1.Config
		expectedError  bool
	}{
		{
			name: "successful get app config",
			setupMock: func(m *appconfig.MockAppConfigManager) {
				expectedConfig := kotsv1beta1.Config{
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
				m.On("Get").Return(expectedConfig, nil)
			},
			expectedConfig: kotsv1beta1.Config{
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
			},
			expectedError: false,
		},
		{
			name: "manager error",
			setupMock: func(m *appconfig.MockAppConfigManager) {
				m.On("Get").Return(kotsv1beta1.Config{}, errors.New("manager error"))
			},
			expectedConfig: kotsv1beta1.Config{},
			expectedError:  true,
		},
		{
			name: "empty config",
			setupMock: func(m *appconfig.MockAppConfigManager) {
				m.On("Get").Return(kotsv1beta1.Config{}, nil)
			},
			expectedConfig: kotsv1beta1.Config{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAppConfigManager := &appconfig.MockAppConfigManager{}
			tt.setupMock(mockAppConfigManager)

			controller := &InstallController{
				appConfigManager: mockAppConfigManager,
			}

			config, err := controller.GetAppConfig(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, kotsv1beta1.Config{}, config)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedConfig, config)
			}

			mockAppConfigManager.AssertExpectations(t)
		})
	}
}
