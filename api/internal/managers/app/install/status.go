package install

import (
	"fmt"
)

func (m *appInstallManager) addLogs(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	if err := m.appInstallStore.AddLogs(msg); err != nil {
		m.logger.WithError(err).Error("add log")
	}
}
