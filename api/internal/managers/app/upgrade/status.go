package appupgrademanager

import (
	"fmt"
)

func (m *appUpgradeManager) addLogs(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if err := m.appUpgradeStore.AddLogs(msg); err != nil {
		m.logger.WithError(err).Error("add log")
	}
}
