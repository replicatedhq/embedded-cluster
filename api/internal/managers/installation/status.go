package installation

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *installationManager) GetStatus() (*types.Status, error) {
	return m.installationStore.GetStatus()
}

func (m *installationManager) SetStatus(status *types.Status) error {
	return m.installationStore.SetStatus(status)
}
