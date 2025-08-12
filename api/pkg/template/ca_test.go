package template

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_PrivateCACert(t *testing.T) {
	tests := []struct {
		name           string
		template       string
		expectedResult string
	}{
		{
			name:           "basic template function returns constant configmap name",
			template:       "{{repl PrivateCACert }}",
			expectedResult: adminconsole.PrivateCASConfigMapName,
		},
		{
			name:           "template function in yaml context",
			template:       "configMapName: {{repl PrivateCACert }}",
			expectedResult: "configMapName: " + adminconsole.PrivateCASConfigMapName,
		},
		{
			name:           "template function in volume definition context",
			template:       "volumes:\n- name: ca-certs\n  configMap:\n    name: {{repl PrivateCACert }}",
			expectedResult: "volumes:\n- name: ca-certs\n  configMap:\n    name: " + adminconsole.PrivateCASConfigMapName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			}

			engine := NewEngine(config, WithMode(ModeGeneric))

			err := engine.Parse(tt.template)
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
			require.NoError(t, err)

			assert.Equal(t, tt.expectedResult, result)
			// Verify the result contains the expected constant value
			assert.Contains(t, result, adminconsole.PrivateCASConfigMapName)
		})
	}
}
