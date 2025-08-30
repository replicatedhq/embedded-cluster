package install

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/helm"
)

// logWriter is an io.Writer that captures output and feeds it to the logs
type logWriter struct {
	manager *appInstallManager
}

func (m *appInstallManager) newLogWriter() io.Writer {
	return &logWriter{manager: m}
}

// ANSI escape sequence regex pattern
var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func (lw *logWriter) Write(p []byte) (n int, err error) {
	output := string(p)
	// Strip ANSI escape sequences
	output = ansiEscapeRegex.ReplaceAllString(output, "")
	output = strings.TrimSpace(output)
	if output != "" {
		lw.manager.addLogs("kots", "%s", output)
	}
	return len(p), nil
}

func (m *appInstallManager) setupHelmClient() error {
	if m.hcli != nil {
		return nil
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		KubernetesEnvSettings: m.kubernetesEnvSettings,
		K8sVersion:            m.k8sVersion,
		LogFn:                 m.logFn("helm"),
	})
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	m.hcli = hcli
	return nil
}

func (m *appInstallManager) logFn(component string) func(format string, v ...interface{}) {
	return func(format string, v ...interface{}) {
		m.logger.WithField("component", component).Debugf(format, v...)
		m.addLogs(component, format, v...)
	}
}

func (m *appInstallManager) addLogs(component string, format string, v ...interface{}) {
	msg := fmt.Sprintf("[%s] %s", component, fmt.Sprintf(format, v...))
	if err := m.appInstallStore.AddLogs(msg); err != nil {
		m.logger.WithError(err).Error("add log")
	}
}

func (m *appInstallManager) writeChartArchiveToTemp(chartArchive []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "helm-chart-*.tgz")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(chartArchive); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write chart archive: %w", err)
	}

	return tmpFile.Name(), nil
}
