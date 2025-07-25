package infra

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metadatafake "k8s.io/client-go/metadata/fake"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewInfraManager_ClientCreation(t *testing.T) {
	tests := []struct {
		name               string
		setupMock          func(*clients.MockRESTClientGetter)
		withKubeClient     bool
		withMetadataClient bool
		withHelmClient     bool
		expectError        bool
	}{
		{
			name: "creates all clients when none provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// kube client and metadata client creation
				mock.On("ToRESTConfig").Return(&rest.Config{}, nil).Times(2)
			},
			expectError: false,
		},
		{
			name: "creates kube and metadata clients when helm client provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// kube client and metadata client creation
				mock.On("ToRESTConfig").Return(&rest.Config{}, nil).Times(2)
			},
			withHelmClient: true,
			expectError:    false,
		},
		{
			name: "creates kube and helm clients when metadata client provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// kube client creation
				mock.On("ToRESTConfig").Return(&rest.Config{}, nil).Times(1)
			},
			withMetadataClient: true,
			expectError:        false,
		},
		{
			name: "creates metadata and helm clients when kube client provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// metadata client creation
				mock.On("ToRESTConfig").Return(&rest.Config{}, nil).Times(1)
			},
			withKubeClient: true,
			expectError:    false,
		},
		{
			name: "creates only helm client when kube and metadata clients provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// No ToRESTConfig calls expected
			},
			withKubeClient:     true,
			withMetadataClient: true,
			expectError:        false,
		},
		{
			name: "creates only metadata client when kube and helm clients provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// metadata client creation
				mock.On("ToRESTConfig").Return(&rest.Config{}, nil).Times(1)
			},
			withKubeClient: true,
			withHelmClient: true,
			expectError:    false,
		},
		{
			name: "creates only kube client when metadata and helm clients provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// kube client creation
				mock.On("ToRESTConfig").Return(&rest.Config{}, nil).Times(1)
			},
			withMetadataClient: true,
			withHelmClient:     true,
			expectError:        false,
		},
		{
			name: "creates no clients when all provided",
			setupMock: func(mock *clients.MockRESTClientGetter) {
				// No ToRESTConfig calls expected
			},
			withKubeClient:     true,
			withMetadataClient: true,
			withHelmClient:     true,
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock RESTClientGetter
			mockRestClientGetter := &clients.MockRESTClientGetter{}
			tt.setupMock(mockRestClientGetter)

			// Build options
			opts := []InfraManagerOption{
				WithRESTClientGetter(mockRestClientGetter),
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

			// Verify mock expectations
			mockRestClientGetter.AssertExpectations(t)
		})
	}
}

func TestNewInfraManager_ToRESTConfigError(t *testing.T) {
	tests := []struct {
		name               string
		withKubeClient     bool
		withMetadataClient bool
		withHelmClient     bool
		expectedError      string
	}{
		{
			name:               "kube client creation fails",
			withMetadataClient: true,
			expectedError:      "create kube client:",
		},
		{
			name:           "metadata client creation fails",
			withKubeClient: true,
			expectedError:  "create metadata client:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock RESTClientGetter that returns error
			mockRestClientGetter := &clients.MockRESTClientGetter{}
			mockRestClientGetter.On("ToRESTConfig").Return((*rest.Config)(nil), assert.AnError)

			// Build options
			opts := []InfraManagerOption{
				WithRESTClientGetter(mockRestClientGetter),
			}

			// Add pre-created clients if specified
			if tt.withKubeClient {
				opts = append(opts, WithKubeClient(fake.NewFakeClient()))
			}
			if tt.withMetadataClient {
				opts = append(opts, WithMetadataClient(metadatafake.NewSimpleMetadataClient(scheme.Scheme)))
			}
			opts = append(opts, WithHelmClient(&helm.MockClient{}))

			// Create manager
			manager, err := NewInfraManager(opts...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Nil(t, manager)

			// Verify mock expectations
			mockRestClientGetter.AssertExpectations(t)
		})
	}
}

func TestNewInfraManager_WithoutRESTClientGetter(t *testing.T) {
	// Test that creating manager without RESTClientGetter fails when clients need to be created
	manager, err := NewInfraManager()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "a valid kube config is required to create a kube client")
	assert.Nil(t, manager)
}

func TestNewInfraManager_WithAllClientsProvided(t *testing.T) {
	// Test that when all clients are provided, no RESTClientGetter is needed
	opts := []InfraManagerOption{
		WithKubeClient(fake.NewFakeClient()),
		WithMetadataClient(metadatafake.NewSimpleMetadataClient(scheme.Scheme)),
		WithHelmClient(&helm.MockClient{}),
	}

	manager, err := NewInfraManager(opts...)

	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.kcli)
	assert.NotNil(t, manager.mcli)
	assert.NotNil(t, manager.hcli)
}
