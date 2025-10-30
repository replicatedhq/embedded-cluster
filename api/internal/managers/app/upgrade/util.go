package appupgrademanager

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
func (m *appUpgradeManager) log(fields any, format string, v ...any) {
	logger := m.logger.WithField("component", "kots")
	if fields != nil {
		f, err := json.Marshal(fields)
		if err == nil {
			logger.WithField("fields", string(f)).Debugf(format, v...)
		} else {
			logger.Debugf(format, v...)
		}
	} else {
		logger.Debugf(format, v...)
	}
	m.addLogs(format, v...)
}

func (m *appUpgradeManager) initKubeClient() error {
	if m.kcli == nil {
		var restClientGetter genericclioptions.RESTClientGetter
		if m.kubernetesEnvSettings != nil {
			restClientGetter = m.kubernetesEnvSettings.RESTClientGetter()
		}

		kcli, err := clients.NewKubeClient(clients.KubeClientOptions{RESTClientGetter: restClientGetter})
		if err != nil {
			return fmt.Errorf("create kube client: %w", err)
		}
		m.kcli = kcli
	}

	return nil
}
