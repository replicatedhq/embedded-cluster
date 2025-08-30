package kubernetesinstallation

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	helmcli "helm.sh/helm/v3/pkg/cli"
)

// Installation defines the interface for managing kubernetes installation
type Installation interface {
	Get() *ecv1beta1.KubernetesInstallation
	Set(installation *ecv1beta1.KubernetesInstallation)

	GetSpec() ecv1beta1.KubernetesInstallationSpec
	SetSpec(spec ecv1beta1.KubernetesInstallationSpec)

	GetStatus() ecv1beta1.KubernetesInstallationStatus
	SetStatus(status ecv1beta1.KubernetesInstallationStatus)

	AdminConsolePort() int
	ManagerPort() int
	ProxySpec() *ecv1beta1.ProxySpec

	SetAdminConsolePort(port int)
	SetManagerPort(port int)
	SetProxySpec(proxySpec *ecv1beta1.ProxySpec)

	PathToEmbeddedBinary(binaryName string) (string, error)

	SetKubernetesEnvSettings(envSettings *helmcli.EnvSettings)
	GetKubernetesEnvSettings() *helmcli.EnvSettings
}
