package installation

import (
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *installationManager) GetStatus() (types.Status, error) {
	return m.installationStore.GetStatus()
}

func (m *installationManager) SetStatus(status types.Status) error {
	return m.installationStore.SetStatus(status)
}

func (m *installationManager) setRunningStatus(description string) error {
	return m.SetStatus(types.Status{
		State:       types.StateRunning,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *installationManager) setFailedStatus(description string) error {
	m.logger.Error(description)

	return m.SetStatus(types.Status{
		State:       types.StateFailed,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *installationManager) setCompletedStatus(state types.State, description string) error {
	return m.SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
