package install

import (
	"fmt"
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

func (m *appInstallManager) addLogs(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	if err := m.appInstallStore.AddLogs(msg); err != nil {
		m.logger.WithError(err).Error("add log")
	}
}
