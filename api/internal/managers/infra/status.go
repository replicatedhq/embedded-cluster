package infra

import (
	"fmt"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *infraManager) GetStatus() (*types.Status, error) {
	return m.infraStore.GetStatus()
}

func (m *infraManager) SetStatus(status types.Status) error {
	return m.infraStore.SetStatus(status)
}

func (m *infraManager) installDidRun() (bool, error) {
	currStatus, err := m.GetStatus()
	if err != nil {
		return false, fmt.Errorf("get status: %w", err)
	}
	if currStatus == nil {
		return false, nil
	}
	if currStatus.State == "" {
		return false, nil
	}
	if currStatus.State == types.StatePending {
		return false, nil
	}
	return true, nil
}

func (m *infraManager) setStatus(state types.State, description string) error {
	return m.SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *infraManager) setComponentStatus(name string, state types.State, description string) error {
	if state == types.StateRunning {
		// update the overall status to reflect the current component
		if err := m.setStatus(types.StateRunning, fmt.Sprintf("%s %s", description, name)); err != nil {
			m.logger.Errorf("Failed to set status: %v", err)
		}
	}
	return m.infraStore.SetComponentStatus(name, &types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
