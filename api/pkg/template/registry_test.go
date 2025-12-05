package template

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_HasLocalRegistry(t *testing.T) {
	tests := []struct {
		name             string
		registrySettings *types.RegistrySettings
		expectedResult   string
	}{
		{
			name:             "nil registry settings returns false",
			registrySettings: nil,
			expectedResult:   "false",
		},
		{
			name: "has local registry false returns false",
			registrySettings: &types.RegistrySettings{
				HasLocalRegistry: false,
			},
			expectedResult: "false",
		},
		{
			name: "has local registry true returns true",
			registrySettings: &types.RegistrySettings{
				HasLocalRegistry: true,
			},
			expectedResult: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl HasLocalRegistry }}")
			require.NoError(t, err)

			var result string
			if tt.registrySettings != nil {
				result, err = engine.Execute(nil, WithRegistrySettings(tt.registrySettings))
			} else {
				result, err = engine.Execute(nil)
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_LocalRegistryHost(t *testing.T) {
	tests := []struct {
		name             string
		registrySettings *types.RegistrySettings
		expectedResult   string
	}{
		{
			name:             "nil registry settings returns empty string",
			registrySettings: nil,
			expectedResult:   "",
		},
		{
			name: "empty host returns empty string",
			registrySettings: &types.RegistrySettings{
				LocalRegistryHost: "",
			},
			expectedResult: "",
		},
		{
			name: "host with port returns host",
			registrySettings: &types.RegistrySettings{
				LocalRegistryHost: "10.128.0.11:5000",
			},
			expectedResult: "10.128.0.11:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl LocalRegistryHost }}")
			require.NoError(t, err)

			var result string
			if tt.registrySettings != nil {
				result, err = engine.Execute(nil, WithRegistrySettings(tt.registrySettings))
			} else {
				result, err = engine.Execute(nil)
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_LocalRegistryAddress(t *testing.T) {
	tests := []struct {
		name             string
		registrySettings *types.RegistrySettings
		expectedResult   string
	}{
		{
			name:             "nil registry settings returns empty string",
			registrySettings: nil,
			expectedResult:   "",
		},
		{
			name: "empty address returns empty string",
			registrySettings: &types.RegistrySettings{
				LocalRegistryAddress: "",
			},
			expectedResult: "",
		},
		{
			name: "address with namespace returns address",
			registrySettings: &types.RegistrySettings{
				LocalRegistryAddress: "10.128.0.11:5000/myapp",
			},
			expectedResult: "10.128.0.11:5000/myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl LocalRegistryAddress }}")
			require.NoError(t, err)

			var result string
			if tt.registrySettings != nil {
				result, err = engine.Execute(nil, WithRegistrySettings(tt.registrySettings))
			} else {
				result, err = engine.Execute(nil)
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_LocalRegistryNamespace(t *testing.T) {
	tests := []struct {
		name             string
		registrySettings *types.RegistrySettings
		expectedResult   string
	}{
		{
			name:             "nil registry settings returns empty string",
			registrySettings: nil,
			expectedResult:   "",
		},
		{
			name: "empty namespace returns empty string",
			registrySettings: &types.RegistrySettings{
				LocalRegistryNamespace: "",
			},
			expectedResult: "",
		},
		{
			name: "namespace returns namespace",
			registrySettings: &types.RegistrySettings{
				LocalRegistryNamespace: "myapp",
			},
			expectedResult: "myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl LocalRegistryNamespace }}")
			require.NoError(t, err)

			var result string
			if tt.registrySettings != nil {
				result, err = engine.Execute(nil, WithRegistrySettings(tt.registrySettings))
			} else {
				result, err = engine.Execute(nil)
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_ImagePullSecretName(t *testing.T) {
	tests := []struct {
		name             string
		registrySettings *types.RegistrySettings
		expectedResult   string
	}{
		{
			name:             "nil registry settings returns empty string",
			registrySettings: nil,
			expectedResult:   "",
		},
		{
			name: "empty secret name returns empty string",
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName: "",
			},
			expectedResult: "",
		},
		{
			name: "secret name returns secret name",
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName: "test-app-registry",
			},
			expectedResult: "test-app-registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl ImagePullSecretName }}")
			require.NoError(t, err)

			var result string
			if tt.registrySettings != nil {
				result, err = engine.Execute(nil, WithRegistrySettings(tt.registrySettings))
			} else {
				result, err = engine.Execute(nil)
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_LocalRegistryImagePullSecret(t *testing.T) {
	tests := []struct {
		name             string
		registrySettings *types.RegistrySettings
		expectedResult   string
	}{
		{
			name:             "nil registry settings returns empty string",
			registrySettings: nil,
			expectedResult:   "",
		},
		{
			name: "empty secret value returns empty string",
			registrySettings: &types.RegistrySettings{
				ImagePullSecretValue: "",
			},
			expectedResult: "",
		},
		{
			name: "secret value returns secret value",
			registrySettings: &types.RegistrySettings{
				ImagePullSecretValue: "eyJhdXRocyI6eyIxMC4xMjguMC4xMTo1MDAwIjp7InVzZXJuYW1lIjoiZW1iZWRkZWQtY2x1c3RlciIsInBhc3N3b3JkIjoidGVzdC1wYXNzd29yZCIsImF1dGgiOiJaVzFpWldSa1pXUXRZMngxYzNSbGNqcDBaWE4wTFhCaGMzTjNiM0prIn19fQ==",
			},
			expectedResult: "eyJhdXRocyI6eyIxMC4xMjguMC4xMTo1MDAwIjp7InVzZXJuYW1lIjoiZW1iZWRkZWQtY2x1c3RlciIsInBhc3N3b3JkIjoidGVzdC1wYXNzd29yZCIsImF1dGgiOiJaVzFpWldSa1pXUXRZMngxYzNSbGNqcDBaWE4wTFhCaGMzTjNiM0prIn19fQ==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl LocalRegistryImagePullSecret }}")
			require.NoError(t, err)

			var result string
			if tt.registrySettings != nil {
				result, err = engine.Execute(nil, WithRegistrySettings(tt.registrySettings))
			} else {
				result, err = engine.Execute(nil)
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestEngine_RegistryFunctionsIntegrated tests multiple registry functions in a single template
func TestEngine_RegistryFunctionsIntegrated(t *testing.T) {
	registrySettings := &types.RegistrySettings{
		HasLocalRegistry:       true,
		LocalRegistryHost:      "10.128.0.11:5000",
		LocalRegistryAddress:   "10.128.0.11:5000/myapp",
		LocalRegistryNamespace: "myapp",
		ImagePullSecretName:    "test-app-registry",
		ImagePullSecretValue:   "eyJhdXRocyI6e319",
	}

	tests := []struct {
		name           string
		template       string
		expectedResult string
	}{
		{
			name:           "conditional registry logic returns local registry host",
			template:       "{{repl HasLocalRegistry | ternary LocalRegistryHost \"proxy.replicated.com\" }}",
			expectedResult: "10.128.0.11:5000",
		},
		{
			name:           "conditional registry namespace logic returns local namespace",
			template:       "{{repl HasLocalRegistry | ternary LocalRegistryNamespace \"external/path\" }}",
			expectedResult: "myapp",
		},
		{
			name:           "image repository with local registry",
			template:       "{{repl HasLocalRegistry | ternary LocalRegistryHost \"proxy.replicated.com\" }}/{{repl HasLocalRegistry | ternary LocalRegistryNamespace \"external/path\" }}/nginx",
			expectedResult: "10.128.0.11:5000/myapp/nginx",
		},
		{
			name:           "image pull secret name in yaml",
			template:       "- name: '{{repl ImagePullSecretName }}'",
			expectedResult: "- name: 'test-app-registry'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse(tt.template)
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithRegistrySettings(registrySettings))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestEngine_RegistryFunctionsWithoutSettings tests registry functions when no settings are provided
func TestEngine_RegistryFunctionsWithoutSettings(t *testing.T) {
	tests := []struct {
		name           string
		template       string
		expectedResult string
	}{
		{
			name:           "conditional registry logic falls back to proxy",
			template:       "{{repl HasLocalRegistry | ternary LocalRegistryHost \"proxy.replicated.com\" }}",
			expectedResult: "proxy.replicated.com",
		},
		{
			name:           "conditional registry namespace logic falls back to external path",
			template:       "{{repl HasLocalRegistry | ternary LocalRegistryNamespace \"external/path\" }}",
			expectedResult: "external/path",
		},
		{
			name:           "image repository without local registry",
			template:       "{{repl HasLocalRegistry | ternary LocalRegistryHost \"proxy.replicated.com\" }}/{{repl HasLocalRegistry | ternary LocalRegistryNamespace \"external/path\" }}/nginx",
			expectedResult: "proxy.replicated.com/external/path/nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse(tt.template)
			require.NoError(t, err)

			// Execute without registry settings
			result, err := engine.Execute(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
