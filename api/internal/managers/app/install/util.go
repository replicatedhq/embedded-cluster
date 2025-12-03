package install

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
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
		lw.manager.logger.WithField("component", "kots").Debug(output)
	}
	return len(p), nil
}

// createConfigValuesFile creates a temporary file with the config values
func (m *appInstallManager) createConfigValuesFile(configValues kotsv1beta1.ConfigValues) (string, error) {
	// Use Kubernetes-specific YAML serialization to properly handle TypeMeta and ObjectMeta
	data, err := kyaml.Marshal(configValues)
	if err != nil {
		return "", fmt.Errorf("marshal config values: %w", err)
	}

	configValuesFile, err := os.CreateTemp("", "config-values*.yaml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer configValuesFile.Close()

	if _, err := configValuesFile.Write(data); err != nil {
		_ = os.Remove(configValuesFile.Name())
		return "", fmt.Errorf("write config values to temp file: %w", err)
	}

	return configValuesFile.Name(), nil
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

func (m *appInstallManager) initKubeClient() error {
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
