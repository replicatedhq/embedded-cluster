package infra

import (
	"context"
	"fmt"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (m *infraManager) waitForNode(ctx context.Context, kcli client.Client) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}
	nodename := strings.ToLower(hostname)
	if err := kubeutils.WaitForNode(ctx, kcli, nodename, false); err != nil {
		return err
	}
	return nil
}

func (m *infraManager) kubeClient() (client.Client, error) {
	if m.kcli != nil {
		return m.kcli, nil
	}
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("create kube client: %w", err)
	}
	return kcli, nil
}

func (m *infraManager) metadataClient() (metadata.Interface, error) {
	if m.mcli != nil {
		return m.mcli, nil
	}
	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return nil, fmt.Errorf("create metadata client: %w", err)
	}
	return mcli, nil
}

func (m *infraManager) helmClient(_ kubernetesinstallation.Installation) (helm.Client, error) {
	if m.hcli != nil {
		return m.hcli, nil
	}

	airgapChartsPath := ""
	if m.airgapBundle != "" {
		// TODO: how can we support airgap?
		airgapChartsPath = "" // rc.EmbeddedClusterChartsSubDir()
	}
	hcli, err := helm.NewClient(helm.HelmOptions{
		RESTClientGetterFactory: m.restClientGetterFactory,
		K0sVersion:              versions.K0sVersion,
		AirgapPath:              airgapChartsPath,
		LogFn:                   m.logFn("helm"),
	})
	if err != nil {
		return nil, fmt.Errorf("create helm client: %w", err)
	}
	return hcli, nil
}

func (m *infraManager) getECConfigSpec() *ecv1beta1.ConfigSpec {
	if m.releaseData == nil || m.releaseData.EmbeddedClusterConfig == nil {
		return nil
	}
	return &m.releaseData.EmbeddedClusterConfig.Spec
}

func (m *infraManager) getEndUserConfigSpec() *ecv1beta1.ConfigSpec {
	if m.endUserConfig == nil {
		return nil
	}
	return &m.endUserConfig.Spec
}

// logFn creates a component-specific logging function that tags log entries with the
// component name and persists them to the infra store for client retrieval,
// as well as logs them to the structured logger.
func (m *infraManager) logFn(component string) func(format string, v ...interface{}) {
	return func(format string, v ...interface{}) {
		m.logger.WithField("component", component).Debugf(format, v...)
		m.addLogs(component, format, v...)
	}
}

func (m *infraManager) addLogs(component string, format string, v ...interface{}) {
	msg := fmt.Sprintf("[%s] %s", component, fmt.Sprintf(format, v...))
	if err := m.infraStore.AddLogs(msg); err != nil {
		m.logger.WithField("error", err).Error("add log")
	}
}
