package template

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_namespace(t *testing.T) {
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(context.Background(), nil)
	require.NoError(t, err)

	tests := []struct {
		name           string
		expectedResult string
	}{
		{
			name:           "returns kotsadm namespace",
			expectedResult: kotsadmNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl Namespace }}")
			require.NoError(t, err)

			result, err := engine.Execute(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_namespaceIntegrated(t *testing.T) {
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(context.Background(), nil)
	require.NoError(t, err)

	tests := []struct {
		name           string
		template       string
		expectedResult string
	}{
		{
			name:           "namespace in string concatenation",
			template:       "{{repl Namespace }}-suffix",
			expectedResult: kotsadmNamespace + "-suffix",
		},
		{
			name:           "namespace in conditional logic",
			template:       `{{repl if eq Namespace "kotsadm" }}correct{{repl else }}incorrect{{repl end }}`,
			expectedResult: "correct",
		},
		{
			name:           "namespace value check",
			template:       "prefix-{{repl Namespace }}",
			expectedResult: "prefix-" + kotsadmNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse(tt.template)
			require.NoError(t, err)

			result, err := engine.Execute(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
