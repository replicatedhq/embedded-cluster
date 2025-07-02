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

func (m *installationManager) setStatus(state types.State, description string) error {
	return m.SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
