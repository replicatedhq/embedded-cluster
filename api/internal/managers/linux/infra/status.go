package infra

import (
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *infraManager) GetStatus() (types.Status, error) {
	return m.infraStore.GetStatus()
}

func (m *infraManager) SetStatus(status types.Status) error {
	return m.infraStore.SetStatus(status)
}

func (m *infraManager) setStatus(state types.State, description string) error {
	return m.SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *infraManager) setStatusDesc(description string) error {
	return m.infraStore.SetStatusDesc(description)
}

func (m *infraManager) setComponentStatus(name string, state types.State, description string) error {
	return m.infraStore.SetComponentStatus(name, types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
