package install

import (
	"io"
	"strings"
)

// logWriter is an io.Writer that captures output and feeds it to the logs
type logWriter struct {
	manager *appInstallManager
}

func (m *appInstallManager) newLogWriter() io.Writer {
	return &logWriter{manager: m}
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	output := strings.TrimSpace(string(p))
	if output != "" {
		lw.manager.addLogs("[kots] %s", output)
	}
	return len(p), nil
}