package types

import (
	"errors"
	"net/http"
)

// Migration error constants
var (
	ErrNoActiveMigration            = errors.New("no active migration")
	ErrMigrationAlreadyStarted      = errors.New("migration already started")
	ErrInvalidTransferMode          = errors.New("invalid transfer mode: must be 'copy' or 'move'")
	ErrMigrationPhaseNotImplemented = errors.New("migration phase execution not yet implemented")
)

// NewNotFoundError creates a 404 API error
func NewNotFoundError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusNotFound,
		Message:    err.Error(),
		err:        err,
	}
}

// MigrationState represents the state of a migration
type MigrationState string

const (
	MigrationStateNotStarted MigrationState = "NotStarted"
	MigrationStateInProgress MigrationState = "InProgress"
	MigrationStateCompleted  MigrationState = "Completed"
	MigrationStateFailed     MigrationState = "Failed"
)

// MigrationPhase represents the phase of a migration
type MigrationPhase string

const (
	MigrationPhaseDiscovery    MigrationPhase = "Discovery"
	MigrationPhasePreparation  MigrationPhase = "Preparation"
	MigrationPhaseECInstall    MigrationPhase = "ECInstall"
	MigrationPhaseDataTransfer MigrationPhase = "DataTransfer"
	MigrationPhaseCompleted    MigrationPhase = "Completed"
)

// TransferMode represents the mode for data transfer during migration
type TransferMode string

const (
	TransferModeCopy TransferMode = "copy"
	TransferModeMove TransferMode = "move"
)

// StartMigrationRequest represents the request to start a migration
// @Description Request body for starting a migration from kURL to Embedded Cluster
type StartMigrationRequest struct {
	// TransferMode specifies whether to copy or move data during migration
	TransferMode TransferMode `json:"transferMode" enums:"copy,move" example:"copy"`
	// Config contains optional installation configuration that will be merged with defaults
	Config *LinuxInstallationConfig `json:"config,omitempty" validate:"optional"`
}

// StartMigrationResponse represents the response when starting a migration
// @Description Response returned when a migration is successfully started
type StartMigrationResponse struct {
	// MigrationID is the unique identifier for this migration
	MigrationID string `json:"migrationId" example:"550e8400-e29b-41d4-a716-446655440000"`
	// Message is a user-facing message about the migration status
	Message string `json:"message" example:"Migration started successfully"`
}

// MigrationStatusResponse represents the status of a migration
// @Description Current status and progress of a migration
type MigrationStatusResponse struct {
	// State is the current state of the migration
	State MigrationState `json:"state" enums:"NotStarted,InProgress,Completed,Failed" example:"InProgress"`
	// Phase is the current phase of the migration process
	Phase MigrationPhase `json:"phase" enums:"Discovery,Preparation,ECInstall,DataTransfer,Completed" example:"Discovery"`
	// Message is a user-facing message describing the current status
	Message string `json:"message" example:"Discovering kURL cluster configuration"`
	// Progress is the completion percentage (0-100)
	Progress int `json:"progress" example:"25"`
	// Error contains the error message if the migration failed
	Error string `json:"error,omitempty" validate:"optional" example:""`
}
