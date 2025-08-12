package template

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_PrivateCACert(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl PrivateCACert }}")
	require.NoError(t, err)

	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)

	// Should always return the constant ConfigMap name
	assert.Equal(t, adminconsole.PrivateCASConfigMapName, result)
	assert.Equal(t, "kotsadm-private-cas", result)
}

func TestEngine_PrivateCACertInTemplate(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	// Test within a more realistic template context
	err := engine.Parse("configmap-name: {{repl PrivateCACert }}")
	require.NoError(t, err)

	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)

	assert.Equal(t, "configmap-name: kotsadm-private-cas", result)
}