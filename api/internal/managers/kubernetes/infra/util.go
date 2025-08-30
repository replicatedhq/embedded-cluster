package infra

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	nodeutil "k8s.io/component-helpers/node/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (m *infraManager) waitForNode(ctx context.Context, kcli client.Client) error {
	nodename, err := nodeutil.GetHostname("")
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}
	if err := kubeutils.WaitForNode(ctx, kcli, nodename, false); err != nil {
		return err
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
