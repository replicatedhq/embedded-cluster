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
			name:        "fails when helm client not provided",
			expectError: true,
		},
		{
			name:           "creates kube and metadata clients when only helm client provided",
			withHelmClient: true,
			expectError:    false,
		},
		{
			name:           "creates metadata client when kube and helm clients provided",
			withKubeClient: true,
			withHelmClient: true,
			expectError:    false,
		},
		{
			name:               "creates kube client when metadata and helm clients provided",
			withMetadataClient: true,
			withHelmClient:     true,
			expectError:        false,
		},
		{
			name:               "uses all provided clients when all are given",
			withKubeClient:     true,
			withMetadataClient: true,
			withHelmClient:     true,
			expectError:        false,
		},
	}

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
				// Create real helm client
				hcli, err := helm.NewClient(helm.HelmOptions{
					HelmPath:   "helm",
					K8sVersion: "v1.26.0",
				})
				require.NoError(t, err)
				opts = append(opts, WithHelmClient(hcli))
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
