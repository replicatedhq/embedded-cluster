package migration

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
)

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	assert.NotNil(t, store)

	// Should return error when no migration is initialized
	_, err := store.GetMigrationID()
	assert.ErrorIs(t, err, types.ErrNoActiveMigration)
}

func TestInitializeMigration(t *testing.T) {
	store := NewMemoryStore()

	config := types.LinuxInstallationConfig{
		AdminConsolePort: 30000,
		DataDirectory:    "/var/lib/embedded-cluster",
	}

	err := store.InitializeMigration("test-migration-id", "copy", config)
	assert.NoError(t, err)

	// Verify migration ID
	id, err := store.GetMigrationID()
	assert.NoError(t, err)
	assert.Equal(t, "test-migration-id", id)

	// Verify transfer mode
	mode, err := store.GetTransferMode()
	assert.NoError(t, err)
	assert.Equal(t, "copy", mode)

	// Verify config
	retrievedConfig, err := store.GetConfig()
	assert.NoError(t, err)
	assert.Equal(t, config.AdminConsolePort, retrievedConfig.AdminConsolePort)
	assert.Equal(t, config.DataDirectory, retrievedConfig.DataDirectory)

	// Verify initial status
	status, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.MigrationStateNotStarted, status.State)
	assert.Equal(t, types.MigrationPhaseDiscovery, status.Phase)
	assert.Equal(t, "", status.Message)
	assert.Equal(t, 0, status.Progress)
	assert.Equal(t, "", status.Error)
}

func TestInitializeMigrationTwice(t *testing.T) {
	store := NewMemoryStore()

	config := types.LinuxInstallationConfig{}

	err := store.InitializeMigration("first-id", "copy", config)
	assert.NoError(t, err)

	// Second initialization should fail
	err = store.InitializeMigration("second-id", "move", config)
	assert.ErrorIs(t, err, types.ErrMigrationAlreadyStarted)
}

func TestSetState(t *testing.T) {
	store := NewMemoryStore()

	// Should return error when no migration is initialized
	err := store.SetState(types.MigrationStateInProgress)
	assert.ErrorIs(t, err, types.ErrNoActiveMigration)

	// Initialize migration
	config := types.LinuxInstallationConfig{}
	err = store.InitializeMigration("test-id", "copy", config)
	assert.NoError(t, err)

	// Update state
	err = store.SetState(types.MigrationStateInProgress)
	assert.NoError(t, err)

	status, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.MigrationStateInProgress, status.State)
}

func TestSetPhase(t *testing.T) {
	store := NewMemoryStore()
	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	assert.NoError(t, err)

	err = store.SetPhase(types.MigrationPhasePreparation)
	assert.NoError(t, err)

	status, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.MigrationPhasePreparation, status.Phase)
}

func TestSetMessage(t *testing.T) {
	store := NewMemoryStore()
	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	assert.NoError(t, err)

	err = store.SetMessage("Preparing migration")
	assert.NoError(t, err)

	status, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, "Preparing migration", status.Message)
}

func TestSetProgress(t *testing.T) {
	store := NewMemoryStore()
	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	assert.NoError(t, err)

	err = store.SetProgress(50)
	assert.NoError(t, err)

	status, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, 50, status.Progress)
}

func TestSetError(t *testing.T) {
	store := NewMemoryStore()
	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	assert.NoError(t, err)

	err = store.SetError("migration failed")
	assert.NoError(t, err)

	status, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, "migration failed", status.Error)
}

func TestStoreOptions(t *testing.T) {
	config := types.LinuxInstallationConfig{
		AdminConsolePort: 30000,
	}

	status := types.MigrationStatusResponse{
		State:    types.MigrationStateInProgress,
		Phase:    types.MigrationPhaseECInstall,
		Message:  "Installing EC",
		Progress: 75,
	}

	store := NewMemoryStore(
		WithMigrationID("pre-initialized"),
		WithTransferMode("move"),
		WithConfig(config),
		WithStatus(status),
	)

	// Verify migration is pre-initialized
	id, err := store.GetMigrationID()
	assert.NoError(t, err)
	assert.Equal(t, "pre-initialized", id)

	mode, err := store.GetTransferMode()
	assert.NoError(t, err)
	assert.Equal(t, "move", mode)

	retrievedConfig, err := store.GetConfig()
	assert.NoError(t, err)
	assert.Equal(t, config.AdminConsolePort, retrievedConfig.AdminConsolePort)

	retrievedStatus, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, status.State, retrievedStatus.State)
	assert.Equal(t, status.Phase, retrievedStatus.Phase)
	assert.Equal(t, status.Message, retrievedStatus.Message)
	assert.Equal(t, status.Progress, retrievedStatus.Progress)
}

func TestDeepCopyPreventsConfigMutation(t *testing.T) {
	store := NewMemoryStore()

	config := types.LinuxInstallationConfig{
		AdminConsolePort: 30000,
		DataDirectory:    "/var/lib/ec",
	}

	err := store.InitializeMigration("test-id", "copy", config)
	assert.NoError(t, err)

	// Get config and modify it (to verify deep copy prevents mutation)
	retrievedConfig, err := store.GetConfig()
	assert.NoError(t, err)
	retrievedConfig.AdminConsolePort = 99999
	retrievedConfig.DataDirectory = "/tmp/modified"

	// Verify the local modifications were made
	assert.Equal(t, 99999, retrievedConfig.AdminConsolePort)
	assert.Equal(t, "/tmp/modified", retrievedConfig.DataDirectory)

	// Verify original config in store is unchanged (proving deep copy works)
	retrievedConfig2, err := store.GetConfig()
	assert.NoError(t, err)
	assert.Equal(t, 30000, retrievedConfig2.AdminConsolePort)
	assert.Equal(t, "/var/lib/ec", retrievedConfig2.DataDirectory)
}

func TestDeepCopyPreventsStatusMutation(t *testing.T) {
	store := NewMemoryStore()
	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	assert.NoError(t, err)

	err = store.SetMessage("Original message")
	assert.NoError(t, err)

	// Get status and modify it (to verify deep copy prevents mutation)
	status, err := store.GetStatus()
	assert.NoError(t, err)
	status.Message = "Modified message"
	status.Progress = 99

	// Verify the local modifications were made
	assert.Equal(t, "Modified message", status.Message)
	assert.Equal(t, 99, status.Progress)

	// Verify original status in store is unchanged (proving deep copy works)
	status2, err := store.GetStatus()
	assert.NoError(t, err)
	assert.Equal(t, "Original message", status2.Message)
	assert.Equal(t, 0, status2.Progress)
}
