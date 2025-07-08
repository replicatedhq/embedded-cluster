package install

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallController_GetAppConfig(t *testing.T) {
	tests := []struct {
		name           string
		releaseData    *release.ReleaseData
		expectedConfig kotsv1beta1.Config
		expectedError  bool
	}{
		{
			name: "successful get app config",
			releaseData: &release.ReleaseData{
				AppConfig: &kotsv1beta1.Config{
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
			name: "empty config",
			releaseData: &release.ReleaseData{
				AppConfig: &kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{},
				},
			},
			expectedConfig: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{},
			},
			expectedError: false,
		},
		{
			name:           "nil release data",
			releaseData:    nil,
			expectedConfig: kotsv1beta1.Config{},
			expectedError:  false,
		},
		{
			name: "nil app config",
			releaseData: &release.ReleaseData{
				AppConfig: nil,
			},
			expectedConfig: kotsv1beta1.Config{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create controller using NewInstallController with release data
			controller, err := NewInstallController(
				WithReleaseData(tt.releaseData),
			)
			require.NoError(t, err)

			config, err := controller.GetAppConfig(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, kotsv1beta1.Config{}, config)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedConfig, config)
			}
		})
	}
}

func TestInstallController_SetAppConfig(t *testing.T) {
	tests := []struct {
		name          string
		releaseData   *release.ReleaseData
		configToSet   kotsv1beta1.Config
		expectedError bool
	}{
		{
			name: "successful set app config",
			releaseData: &release.ReleaseData{
				AppConfig: &kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{},
				},
			},
			configToSet: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "new-group",
							Title: "New Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "new-item",
									Type:    "text",
									Title:   "New Item",
									Default: multitype.BoolOrString{StrVal: "new-default"},
									Value:   multitype.BoolOrString{StrVal: "new-value"},
								},
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "set empty config",
			releaseData: &release.ReleaseData{
				AppConfig: &kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{},
				},
			},
			configToSet: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{},
			},
			expectedError: false,
		},
		{
			name: "set config with boolean fields",
			releaseData: &release.ReleaseData{
				AppConfig: &kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{},
				},
			},
			configToSet: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "boolean-group",
							Title: "Boolean Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "bool-item",
									Type:    "bool",
									Title:   "Boolean Item",
									Default: multitype.BoolOrString{BoolVal: true},
									Value:   multitype.BoolOrString{BoolVal: false},
								},
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name:        "set config with nil release data",
			releaseData: nil,
			configToSet: kotsv1beta1.Config{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create controller using NewInstallController with release data
			controller, err := NewInstallController(
				WithReleaseData(tt.releaseData),
			)
			require.NoError(t, err)

			// Test SetAppConfig
			err = controller.SetAppConfig(context.Background(), tt.configToSet)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify the config was set by getting it back
				retrievedConfig, getErr := controller.GetAppConfig(context.Background())
				assert.NoError(t, getErr)
				assert.Equal(t, tt.configToSet, retrievedConfig)
			}
		})
	}
}
