package kubernetesinstallation

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
)

// Installation defines the interface for managing kubernetes installation
type Installation interface {
	Get() *ecv1beta1.KubernetesInstallationSpec
	Set(spec *ecv1beta1.KubernetesInstallationSpec)

	AdminConsolePort() int
	ManagerPort() int
	ProxySpec() *ecv1beta1.ProxySpec

	SetAdminConsolePort(port int)
	SetManagerPort(port int)
	SetProxySpec(proxySpec *ecv1beta1.ProxySpec)
}
