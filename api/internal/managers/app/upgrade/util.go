package appupgrademanager

import (
	"encoding/json"
	"io"
	"strings"
)

// logWriter is an io.Writer that captures output and feeds it to the logs
type logWriter struct {
	manager *appUpgradeManager
}

func (m *appUpgradeManager) newLogWriter() io.Writer {
	return &logWriter{manager: m}
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	output := strings.TrimSpace(string(p))
	if output != "" {
		lw.manager.log(nil, "[kots] %s", output)
	}
	return len(p), nil
}

// log logs a message to the structured logger and adds it to the logs store
func (m *appUpgradeManager) log(fields interface{}, format string, v ...interface{}) {
	if fields != nil {
		f, err := json.Marshal(fields)
		if err == nil {
			m.logger.WithField("fields", string(f)).Debugf(format, v...)
		} else {
			m.logger.Debugf(format, v...)
		}
	} else {
		m.logger.Debugf(format, v...)
	}
	m.addLogs(format, v...)
}
