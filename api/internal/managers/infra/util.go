package infra

import (
	"context"
	"fmt"
	"os"
	"strings"

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

func (m *infraManager) getHelmClient() (helm.Client, error) {
	airgapChartsPath := ""
	if m.airgapBundle != "" {
		airgapChartsPath = m.rc.EmbeddedClusterChartsSubDir()
	}
	hcli, err := helm.NewClient(helm.HelmOptions{
		KubeConfig: m.rc.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
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
