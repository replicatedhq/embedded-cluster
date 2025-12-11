package install

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

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

// setupClients initializes the kube, metadata, and helm clients if they are not already set.
func (m *appInstallManager) setupClients() error {
	var restClientGetter genericclioptions.RESTClientGetter
	if m.kubernetesEnvSettings != nil {
		restClientGetter = m.kubernetesEnvSettings.RESTClientGetter()
	}

	if m.kcli == nil {
		kcli, err := clients.NewKubeClient(clients.KubeClientOptions{RESTClientGetter: restClientGetter})
		if err != nil {
			return fmt.Errorf("create kube client: %w", err)
		}
		m.kcli = kcli
	}

	if m.mcli == nil && restClientGetter != nil {
		mcli, err := clients.NewMetadataClient(clients.KubeClientOptions{RESTClientGetter: restClientGetter})
		if err != nil {
			return fmt.Errorf("create metadata client: %w", err)
		}
		m.mcli = mcli
	}

	if m.hcli == nil {
		return fmt.Errorf("helm client is required")
	}

	return nil
}
