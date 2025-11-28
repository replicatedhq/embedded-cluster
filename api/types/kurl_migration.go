package types

import (
	"errors"
	"net/http"
)

// kURL Migration error constants
var (
	ErrNoActiveKURLMigration            = errors.New("no active kURL migration")
	ErrKURLMigrationAlreadyStarted      = errors.New("kURL migration already started")
	ErrInvalidTransferMode              = errors.New("invalid transfer mode: must be 'copy' or 'move'")
	ErrKURLMigrationPhaseNotImplemented = errors.New("kURL migration phase execution not yet implemented")
)

// NewNotFoundError creates a 404 API error
func NewNotFoundError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusNotFound,
		Message:    err.Error(),
		err:        err,
	}
}

// KURLMigrationState represents the state of a kURL migration
type KURLMigrationState string

const (
	KURLMigrationStateNotStarted KURLMigrationState = "NotStarted"
	KURLMigrationStateInProgress KURLMigrationState = "InProgress"
	KURLMigrationStateCompleted  KURLMigrationState = "Completed"
	KURLMigrationStateFailed     KURLMigrationState = "Failed"
)

// KURLMigrationPhase represents the phase of a kURL migration
type KURLMigrationPhase string

const (
	KURLMigrationPhaseDiscovery    KURLMigrationPhase = "Discovery"
	KURLMigrationPhasePreparation  KURLMigrationPhase = "Preparation"
	KURLMigrationPhaseECInstall    KURLMigrationPhase = "ECInstall"
	KURLMigrationPhaseDataTransfer KURLMigrationPhase = "DataTransfer"
	KURLMigrationPhaseCompleted    KURLMigrationPhase = "Completed"
)

// TransferMode represents the mode for data transfer during migration
type TransferMode string

const (
	TransferModeCopy TransferMode = "copy"
	TransferModeMove TransferMode = "move"
)

// StartKURLMigrationRequest represents the request to start a kURL migration
// @Description Request body for starting a migration from kURL to Embedded Cluster
type StartKURLMigrationRequest struct {
	// TransferMode specifies whether to copy or move data during kURL migration
	TransferMode TransferMode `json:"transferMode" enums:"copy,move" example:"copy"`
	// Config contains optional installation configuration that will be merged with defaults
	Config *LinuxInstallationConfig `json:"config,omitempty" validate:"optional"`
}

// StartKURLMigrationResponse represents the response when starting a kURL migration
// @Description Response returned when a kURL migration is successfully started
type StartKURLMigrationResponse struct {
	// MigrationID is the unique identifier for this migration
	MigrationID string `json:"migrationId" example:"550e8400-e29b-41d4-a716-446655440000"`
	// Message is a user-facing message about the kURL migration status
	Message string `json:"message" example:"kURL migration started successfully"`
}

// KURLMigrationStatusResponse represents the status of a kURL migration
// @Description Current status and progress of a kURL migration
type KURLMigrationStatusResponse struct {
	// State is the current state of the kURL migration
	State KURLMigrationState `json:"state" enums:"NotStarted,InProgress,Completed,Failed" example:"InProgress"`
	// Phase is the current phase of the kURL migration process
	Phase KURLMigrationPhase `json:"phase" enums:"Discovery,Preparation,ECInstall,DataTransfer,Completed" example:"Discovery"`
	// Message is a user-facing message describing the current status
	Message string `json:"message" example:"Discovering kURL cluster configuration"`
	// Progress is the completion percentage (0-100)
	Progress int `json:"progress" example:"25"`
	// Error contains the error message if the kURL migration failed
	Error string `json:"error,omitempty" validate:"optional" example:""`
}
