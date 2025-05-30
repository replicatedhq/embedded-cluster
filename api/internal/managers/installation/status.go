package installation

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *installationManager) ReadStatus() (*types.Status, error) {
	return m.installationStore.ReadStatus()
}

func (m *installationManager) WriteStatus(status types.Status) error {
	return m.installationStore.WriteStatus(status)
}
