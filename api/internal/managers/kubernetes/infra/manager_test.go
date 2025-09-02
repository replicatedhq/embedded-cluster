package infra

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	helmcli "helm.sh/helm/v3/pkg/cli"
	metadatafake "k8s.io/client-go/metadata/fake"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewInfraManager_ClientCreation(t *testing.T) {
	tests := []struct {
		name               string
		withKubeClient     bool
		withMetadataClient bool
		withHelmClient     bool
		expectError        bool
	}{
		{
			name:        "creates all clients when none provided",
			expectError: false,
		},
		{
			name:           "creates kube and metadata clients when helm client provided",
			withHelmClient: true,
			expectError:    false,
		},
		{
			name:               "creates kube and helm clients when metadata client provided",
			withMetadataClient: true,
			expectError:        false,
		},
		{
			name:           "creates metadata and helm clients when kube client provided",
			withKubeClient: true,
			expectError:    false,
		},
		{
			name:               "creates only helm client when kube and metadata clients provided",
			withKubeClient:     true,
			withMetadataClient: true,
			expectError:        false,
		},
		{
			name:           "creates only metadata client when kube and helm clients provided",
			withKubeClient: true,
			withHelmClient: true,
			expectError:    false,
		},
		{
			name:               "creates only kube client when metadata and helm clients provided",
			withMetadataClient: true,
			withHelmClient:     true,
			expectError:        false,
		},
		{
			name:               "creates no clients when all provided",
			withKubeClient:     true,
			withMetadataClient: true,
			withHelmClient:     true,
			expectError:        false,
		},
	}

	t.Setenv("HELM_BINARY_PATH", "helm") // use the helm binary in PATH

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build options
			opts := []InfraManagerOption{
				WithKubernetesEnvSettings(helmcli.New()),
			}

			// Add pre-created clients if specified
			if tt.withKubeClient {
				opts = append(opts, WithKubeClient(fake.NewFakeClient()))
			}
			if tt.withMetadataClient {
				opts = append(opts, WithMetadataClient(metadatafake.NewSimpleMetadataClient(scheme.Scheme)))
			}
			if tt.withHelmClient {
				opts = append(opts, WithHelmClient(&helm.MockClient{}))
			}

			// Create manager
			manager, err := NewInfraManager(opts...)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, manager)
			assert.NotNil(t, manager.kcli)
			assert.NotNil(t, manager.mcli)
			assert.NotNil(t, manager.hcli)
		})
	}
}
