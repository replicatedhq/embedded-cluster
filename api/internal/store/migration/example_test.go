package migration_test

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func ExampleStore_complete_migration_flow() {
	// Create a new global store
	globalStore := store.NewMemoryStore()

	// Get the migration store
	migrationStore := globalStore.MigrationStore()

	// Initialize a new migration
	config := types.LinuxInstallationConfig{
		AdminConsolePort: 30000,
		DataDirectory:    "/var/lib/embedded-cluster",
	}

	err := migrationStore.InitializeMigration("migration-123", "copy", config)
	if err != nil {
		fmt.Printf("Error initializing migration: %v\n", err)
		return
	}

	// Get migration ID
	id, err := migrationStore.GetMigrationID()
	if err != nil {
		fmt.Printf("Error getting migration ID: %v\n", err)
		return
	}
	fmt.Printf("Migration ID: %s\n", id)

	// Update migration state and phase
	migrationStore.SetState(types.MigrationStateInProgress)
	migrationStore.SetPhase(types.MigrationPhaseDiscovery)
	migrationStore.SetMessage("Discovering kURL cluster configuration")
	migrationStore.SetProgress(10)

	// Get status
	status, err := migrationStore.GetStatus()
	if err != nil {
		fmt.Printf("Error getting status: %v\n", err)
		return
	}

	fmt.Printf("State: %s\n", status.State)
	fmt.Printf("Phase: %s\n", status.Phase)
	fmt.Printf("Message: %s\n", status.Message)
	fmt.Printf("Progress: %d%%\n", status.Progress)

	// Continue migration
	migrationStore.SetPhase(types.MigrationPhasePreparation)
	migrationStore.SetMessage("Preparing migration environment")
	migrationStore.SetProgress(25)

	// Complete migration
	migrationStore.SetState(types.MigrationStateCompleted)
	migrationStore.SetPhase(types.MigrationPhaseCompleted)
	migrationStore.SetMessage("Migration completed successfully")
	migrationStore.SetProgress(100)

	// Output:
	// Migration ID: migration-123
	// State: InProgress
	// Phase: Discovery
	// Message: Discovering kURL cluster configuration
	// Progress: 10%
}
