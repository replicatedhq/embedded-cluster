package install

import (
	"fmt"
	"io"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
		lw.manager.logger.WithField("component", "kots").Debug(string(p))
	}
	return len(p), nil
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
