package install

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
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
		lw.manager.addLogs("kots", "%s", output)
	}
	return len(p), nil
}

func (m *appInstallManager) setupHelmClient() error {
	if m.hcli != nil {
		return nil
	}

	k8sVersion := m.k8sVersion
	if k8sVersion == "" {
		var err error
		k8sVersion, err = m.getK8sVersion()
		if err != nil {
			return fmt.Errorf("get k8s version: %w", err)
		}
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		KubeConfig:       m.kubeConfigPath,
		RESTClientGetter: m.restClientGetter,
		K8sVersion:       k8sVersion,
		LogFn:            m.logFn("app-helm"),
	})
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	m.hcli = hcli
	return nil
}

// getK8sVersion creates a kubernetes client and returns the kubernetes version
func (m *appInstallManager) getK8sVersion() (string, error) {
	kcli, err := clients.NewDiscoveryClient(clients.KubeClientOptions{
		RESTClientGetter: m.restClientGetter,
		KubeConfigPath:   m.kubeConfigPath,
	})
	if err != nil {
		return "", fmt.Errorf("create discovery client: %w", err)
	}
	version, err := kcli.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("get server version: %w", err)
	}
	return version.String(), nil
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
