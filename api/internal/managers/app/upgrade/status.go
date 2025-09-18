package appupgrademanager

import (
	"fmt"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *appUpgradeManager) GetStatus() (types.AppUpgrade, error) {
	return m.appUpgradeStore.Get()
}

func (m *appUpgradeManager) setStatus(state types.State, description string) error {
	return m.appUpgradeStore.SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *appUpgradeManager) addLogs(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if err := m.appUpgradeStore.AddLogs(msg); err != nil {
		m.logger.WithError(err).Error("add log")
	}
}
