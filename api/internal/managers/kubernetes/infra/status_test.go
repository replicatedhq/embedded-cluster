package infra

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metadatafake "k8s.io/client-go/metadata/fake"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInfraWithLogs(t *testing.T) {
	manager, err := NewInfraManager(WithKubeClient(fake.NewFakeClient()), WithMetadataClient(metadatafake.NewSimpleMetadataClient(scheme.Scheme)), WithHelmClient(&helm.MockClient{}))
	require.NoError(t, err)

	// Add some logs through the internal logging mechanism
	logFn := manager.logFn("test")
	logFn("Test log message")
	logFn("Another log message with arg: %s", "value")

	// Get the infra and verify logs are included
	infra, err := manager.Get()
	assert.NoError(t, err)
	assert.Contains(t, infra.Logs, "[test] Test log message")
	assert.Contains(t, infra.Logs, "[test] Another log message with arg: value")
}
