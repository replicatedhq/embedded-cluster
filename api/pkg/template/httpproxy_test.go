package template

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_HTTPProxy(t *testing.T) {
	mockIC := &MockInstallationConfig{}
	mockIC.On("ProxySpec").Return(&ecv1beta1.ProxySpec{
		HTTPProxy:  "http://proxy.example.com",
		HTTPSProxy: "https://proxy.example.com",
		NoProxy:    "localhost,127.0.0.1",
	})

	engine := NewEngine(nil)
	err := engine.Parse("{{repl HTTPProxy }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "http://proxy.example.com", result)

	err = engine.Parse("{{repl HTTPSProxy }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "https://proxy.example.com", result)

	err = engine.Parse("{{repl NoProxy }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "localhost,127.0.0.1", result)

	mockIC.AssertExpectations(t)
}

func TestEngine_HTTPProxy_NilProxySpec(t *testing.T) {
	mockIC := &MockInstallationConfig{}
	mockIC.On("ProxySpec").Return(nil)

	engine := NewEngine(nil)
	err := engine.Parse("{{repl HTTPProxy }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "", result)

	err = engine.Parse("{{repl HTTPSProxy }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "", result)

	err = engine.Parse("{{repl NoProxy }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "", result)

	mockIC.AssertExpectations(t)
}

func TestEngine_HTTPProxy_EmptyProxySpec(t *testing.T) {
	mockIC := &MockInstallationConfig{}
	mockIC.On("ProxySpec").Return(&ecv1beta1.ProxySpec{})

	engine := NewEngine(nil)
	err := engine.Parse("{{repl HTTPProxy }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "", result)

	err = engine.Parse("{{repl HTTPSProxy }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "", result)

	err = engine.Parse("{{repl NoProxy }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, mockIC)
	require.NoError(t, err)
	assert.Equal(t, "", result)

	mockIC.AssertExpectations(t)
}
