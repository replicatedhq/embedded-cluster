package install

import (
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *appInstallManager) GetStatus() (types.AppInstall, error) {
	return m.appInstallStore.Get()
}

func (m *appInstallManager) setStatus(state types.State, description string) error {
	return m.appInstallStore.SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *appInstallManager) setComponentStatus(componentName string, state types.State, description string) error {
	return m.appInstallStore.SetComponentStatus(componentName, types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
