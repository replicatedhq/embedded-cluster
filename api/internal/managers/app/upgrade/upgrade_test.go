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
				opts.Namespace == "kotsadm" &&
				opts.ClusterID == "test-cluster" &&
				opts.AirgapBundle == "" && // No airgap bundle for online
				len(opts.License) == 0 && // No license for online
				opts.ChannelID == "channel-123" &&
				opts.ChannelSequence == 456 &&
				opts.SkipPreflights == true
		})).Return(nil)

		// Create manager for online deployment (no license or airgap bundle)
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithClusterID("test-cluster"),
			WithKotsCLI(mockKotsCLI),
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

		// Create manager (online deployment - no license or airgap bundle)
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithClusterID("test-cluster"),
			WithKotsCLI(mockKotsCLI),
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
				opts.Namespace == "kotsadm" &&
				opts.ClusterID == "test-cluster" &&
				opts.AirgapBundle == "airgap-bundle.tar.gz" &&
				len(opts.License) > 0 && // License provided for airgap
				opts.ChannelID == "channel-123" &&
				opts.ChannelSequence == 456 &&
				opts.SkipPreflights == true
		})).Return(nil)

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
		)
		require.NoError(t, err)

		// Execute upgrade
		err = manager.Upgrade(context.Background(), configValues)
		require.NoError(t, err)

		// Verify mock was called with correct airgap bundle
		mockKotsCLI.AssertExpectations(t)
	})
}

func TestAppUpgradeManager_GetStatus(t *testing.T) {
	// Initialize store with a default state
	store := appupgradestore.NewMemoryStore(appupgradestore.WithAppUpgrade(types.AppUpgrade{
		Status: types.Status{
			State: types.StatePending,
		},
	}))
	manager, err := NewAppUpgradeManager(
		WithLogger(logger.NewDiscardLogger()),
		WithAppUpgradeStore(store),
	)
	require.NoError(t, err)

	// Test getting status
	upgrade, err := manager.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.StatePending, upgrade.Status.State)
	assert.Equal(t, "", upgrade.Logs)
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
