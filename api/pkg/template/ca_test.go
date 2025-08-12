package template

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_PrivateCACert(t *testing.T) {
	tests := []struct {
		name           string
		configMapName  string
		template       string
		expectedResult string
	}{
		{
			name:           "Linux installation - basic template function returns configmap name",
			configMapName:  "kotsadm-private-cas",
			template:       "{{repl PrivateCACert }}",
			expectedResult: "kotsadm-private-cas",
		},
		{
			name:           "Linux installation - template function in yaml context",
			configMapName:  "kotsadm-private-cas",
			template:       "configMapName: {{repl PrivateCACert }}",
			expectedResult: "configMapName: kotsadm-private-cas",
		},
		{
			name:           "Linux installation - template function in volume definition context",
			configMapName:  "kotsadm-private-cas",
			template:       "volumes:\n- name: ca-certs\n  configMap:\n    name: {{repl PrivateCACert }}",
			expectedResult: "volumes:\n- name: ca-certs\n  configMap:\n    name: kotsadm-private-cas",
		},
		{
			name:           "Kubernetes installation - template function returns empty string (not yet implemented)",
			configMapName:  "",
			template:       "{{repl PrivateCACert }}",
			expectedResult: "",
		},
		{
			name:           "Kubernetes installation - template function in yaml context (not yet implemented)",
			configMapName:  "",
			template:       "configMapName: {{repl PrivateCACert }}",
			expectedResult: "configMapName: ",
		},
		{
			name:           "Kubernetes installation - conditional usage with if statement (not yet implemented)",
			configMapName:  "",
			template:       "{{repl if PrivateCACert }}configMapName: {{repl PrivateCACert }}{{repl end }}",
			expectedResult: "",
		},
		{
			name:           "Linux installation - conditional usage with if statement",
			configMapName:  "kotsadm-private-cas",
			template:       "{{repl if PrivateCACert }}configMapName: {{repl PrivateCACert }}{{repl end }}",
			expectedResult: "configMapName: kotsadm-private-cas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			}

			engine := NewEngine(config, 
				WithMode(ModeGeneric), 
				WithPrivateCACertConfigMapName(tt.configMapName))

			err := engine.Parse(tt.template)
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
			require.NoError(t, err)

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}