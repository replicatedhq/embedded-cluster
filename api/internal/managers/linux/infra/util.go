package infra

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
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

// setupClients initializes the kube, metadata, and helm clients if they are not already set.
// We need to do it after the infra manager is initialized to ensure that the runtime config is available and we already have a cluster setup
func (m *infraManager) setupClients(kubeConfigPath string, airgapChartsPath string) error {
	if m.kcli == nil {
		kcli, err := clients.NewKubeClient(clients.KubeClientOptions{KubeConfigPath: kubeConfigPath})
		if err != nil {
			return fmt.Errorf("create kube client: %w", err)
		}
		m.kcli = kcli
	}

	if m.mcli == nil {
		mcli, err := clients.NewMetadataClient(clients.KubeClientOptions{KubeConfigPath: kubeConfigPath})
		if err != nil {
			return fmt.Errorf("create metadata client: %w", err)
		}
		m.mcli = mcli
	}

	if m.hcli == nil {
		airgapPath := ""
		if m.airgapBundle != "" {
			airgapPath = airgapChartsPath
		}
		hcli, err := helm.NewClient(helm.HelmOptions{
			KubeConfig: kubeConfigPath,
			K8sVersion: versions.K0sVersion,
			AirgapPath: airgapPath,
			LogFn:      m.logFn("helm"),
		})
		if err != nil {
			return fmt.Errorf("create helm client: %w", err)
		}
		m.hcli = hcli
	}

	return nil
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
		m.logger.WithError(err).Error("add log")
	}
}
