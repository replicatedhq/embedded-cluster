package airgap

import "github.com/replicatedhq/embedded-cluster/api/types"

// GetStatus returns the current airgap processing status
func (m *airgapManager) GetStatus() (types.Airgap, error) {
	return m.airgapStore.Get()
}

func (m *airgapManager) setStatus(state types.State, description string) error {
	status := types.Status{
		State:       state,
		Description: description,
	}
	return m.airgapStore.SetStatus(status)
}
