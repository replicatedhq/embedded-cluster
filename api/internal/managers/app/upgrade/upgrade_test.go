package appupgrademanager

import (
	"context"
	"testing"

	appupgradestore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAppUpgradeManager_Upgrade(t *testing.T) {
	// Setup environment variable for V3
	t.Setenv("ENABLE_V3", "1")

	// Create test release data
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			AppSlug:         "test-app",
			VersionLabel:    "v1.0.0",
			ChannelID:       "channel-123",
			ChannelSequence: 456,
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "replicated.app",
			},
		},
	}

	// Set up release data globally so AppSlug() returns the correct value for v3
	err := release.SetReleaseDataForTests(map[string][]byte{
		"channelrelease.yaml": []byte("# channel release object\nappSlug: test-app"),
	})
	require.NoError(t, err)

	t.Run("Successful online upgrade", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{
			Spec: kotsv1beta1.ConfigValuesSpec{
				Values: map[string]kotsv1beta1.ConfigValue{
					"key1": {
						Value: "value1",
					},
				},
			},
		}

		// Create mock deployer for online deployment (no airgap bundle or license)
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Deploy", mock.MatchedBy(func(opts kotscli.DeployOptions) bool {
			return opts.AppSlug == "test-app" &&
				opts.Namespace == "test-app" &&
				opts.ClusterID == "test-cluster" &&
				opts.AirgapBundle == "" && // No airgap bundle for online
				len(opts.License) == 0 && // No license for online
				opts.ChannelID == "channel-123" &&
				opts.ChannelSequence == 456 &&
				opts.SkipPreflights == true
		})).Return(nil)

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create manager for online deployment (no license or airgap bundle)
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithClusterID("test-cluster"),
			WithKotsCLI(mockKotsCLI),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Execute upgrade
		err = manager.Upgrade(context.Background(), configValues)
		require.NoError(t, err)

		// Verify final status
		status, err := store.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, types.StateSucceeded, status.State)
		assert.Equal(t, "Upgrade complete", status.Description)

		// Verify mock was called
		mockKotsCLI.AssertExpectations(t)
	})

	t.Run("Failed upgrade should set failed status", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{}

		// Create mock deployer that fails
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Deploy", mock.Anything).Return(assert.AnError)

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create manager (online deployment - no license or airgap bundle)
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithClusterID("test-cluster"),
			WithKotsCLI(mockKotsCLI),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Execute upgrade
		err = manager.Upgrade(context.Background(), configValues)
		require.Error(t, err)

		// Verify failed status
		status, err := store.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, types.StateFailed, status.State)
		assert.Contains(t, status.Description, assert.AnError.Error())

		// Verify mock was called
		mockKotsCLI.AssertExpectations(t)
	})

	t.Run("Successful airgap upgrade", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{}

		// Create mock deployer for airgap deployment
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Deploy", mock.MatchedBy(func(opts kotscli.DeployOptions) bool {
			return opts.AppSlug == "test-app" &&
				opts.Namespace == "test-app" &&
				opts.ClusterID == "test-cluster" &&
				opts.AirgapBundle == "airgap-bundle.tar.gz" &&
				len(opts.License) > 0 && // License provided for airgap
				opts.ChannelID == "channel-123" &&
				opts.ChannelSequence == 456 &&
				opts.SkipPreflights == true
		})).Return(nil)

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create manager with airgap bundle and license
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithLicense([]byte("test-license")),
			WithClusterID("test-cluster"),
			WithAirgapBundle("airgap-bundle.tar.gz"),
			WithKotsCLI(mockKotsCLI),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Execute upgrade
		err = manager.Upgrade(context.Background(), configValues)
		require.NoError(t, err)

		// Verify mock was called with correct airgap bundle
		mockKotsCLI.AssertExpectations(t)
	})
}

func TestAppUpgradeManager_NewWithOptions(t *testing.T) {
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			AppSlug: "test-app",
		},
	}

	store := appupgradestore.NewMemoryStore()

	manager, err := NewAppUpgradeManager(
		WithLogger(logger.NewDiscardLogger()),
		WithAppUpgradeStore(store),
		WithReleaseData(releaseData),
		WithClusterID("test-cluster-id"),
		WithAirgapBundle("test-bundle.tar.gz"),
	)

	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, releaseData, manager.releaseData)
	assert.Equal(t, "test-cluster-id", manager.clusterID)
	assert.Equal(t, "test-bundle.tar.gz", manager.airgapBundle)
}
