package template

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_HTTPProxy(t *testing.T) {
	tests := []struct {
		name           string
		httpProxy      string
		expectedResult string
	}{
		{
			name:           "empty proxy returns empty string",
			httpProxy:      "",
			expectedResult: "",
		},
		{
			name:           "http proxy url is returned",
			httpProxy:      "http://proxy.example.com:8080",
			expectedResult: "http://proxy.example.com:8080",
		},
		{
			name:           "https proxy url is returned",
			httpProxy:      "https://secure-proxy.example.com:3128",
			expectedResult: "https://secure-proxy.example.com:3128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl HTTPProxy }}")
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{
				HTTPProxy: tt.httpProxy,
			}))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_HTTPSProxy(t *testing.T) {
	tests := []struct {
		name           string
		httpsProxy     string
		expectedResult string
	}{
		{
			name:           "empty proxy returns empty string",
			httpsProxy:     "",
			expectedResult: "",
		},
		{
			name:           "https proxy url is returned",
			httpsProxy:     "https://proxy.example.com:8080",
			expectedResult: "https://proxy.example.com:8080",
		},
		{
			name:           "http proxy url is returned",
			httpsProxy:     "http://insecure-proxy.example.com:3128",
			expectedResult: "http://insecure-proxy.example.com:3128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl HTTPSProxy }}")
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{
				HTTPSProxy: tt.httpsProxy,
			}))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_NoProxy(t *testing.T) {
	tests := []struct {
		name           string
		noProxy        string
		expectedResult string
	}{
		{
			name:           "empty no proxy returns empty string",
			noProxy:        "",
			expectedResult: "",
		},
		{
			name:           "single host is returned",
			noProxy:        "localhost",
			expectedResult: "localhost",
		},
		{
			name:           "multiple hosts are returned",
			noProxy:        "localhost,127.0.0.1,.example.com",
			expectedResult: "localhost,127.0.0.1,.example.com",
		},
		{
			name:           "ip addresses and domains are returned",
			noProxy:        "10.0.0.0/8,192.168.0.0/16,localhost,.local",
			expectedResult: "10.0.0.0/8,192.168.0.0/16,localhost,.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric))
			err := engine.Parse("{{repl NoProxy }}")
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{
				NoProxy: tt.noProxy,
			}))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_ProxyState_MultipleExecuteCalls(t *testing.T) {
	// Create a single engine instance and parse a template that uses all proxy functions
	engine := NewEngine(nil, WithMode(ModeGeneric))
	err := engine.Parse("HTTP:{{repl HTTPProxy }} HTTPS:{{repl HTTPSProxy }} NO:{{repl NoProxy }}")
	require.NoError(t, err)

	// First execution with initial proxy options
	result1, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{
		HTTPProxy:  "http://proxy1.example.com:8080",
		HTTPSProxy: "https://proxy1.example.com:8443",
		NoProxy:    "localhost,127.0.0.1",
	}))
	require.NoError(t, err)
	assert.Equal(t, "HTTP:http://proxy1.example.com:8080 HTTPS:https://proxy1.example.com:8443 NO:localhost,127.0.0.1", result1)

	// Second execution with no options - should clear previous state
	result2, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "HTTP: HTTPS: NO:", result2)

	// Third execution with different proxy options - should update state
	result3, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{
		HTTPProxy:  "http://proxy2.example.com:3128",
		HTTPSProxy: "https://proxy2.example.com:3129",
		NoProxy:    ".internal,.local",
	}))
	require.NoError(t, err)
	assert.Equal(t, "HTTP:http://proxy2.example.com:3128 HTTPS:https://proxy2.example.com:3129 NO:.internal,.local", result3)

	// Fourth execution with partial options - should only set provided options, others should be empty
	result4, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{
		HTTPProxy: "http://proxy3.example.com:8080",
	}))
	require.NoError(t, err)
	assert.Equal(t, "HTTP:http://proxy3.example.com:8080 HTTPS: NO:", result4)

	// Fifth execution with no options again - should clear all state
	result5, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "HTTP: HTTPS: NO:", result5)
}
