package airgap

import (
	"fmt"
	"io"
	"strings"
)

func (m *airgapManager) addLogs(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if err := m.airgapStore.AddLogs(msg); err != nil {
		m.logger.WithError(err).Error("add log")
	}
}

// logWriter is an io.Writer that captures output and feeds it to the logs
type logWriter struct {
	manager *airgapManager
}

func (m *airgapManager) newLogWriter() io.Writer {
	return &logWriter{
		manager: m,
	}
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	output := strings.TrimSpace(string(p))
	if output != "" {
		lw.manager.addLogs("%s", output)
	}
	return len(p), nil
}
