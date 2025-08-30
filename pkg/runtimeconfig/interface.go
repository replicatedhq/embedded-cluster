package runtimeconfig

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	helmcli "helm.sh/helm/v3/pkg/cli"
)

// RuntimeConfig defines the interface for managing runtime configuration
type RuntimeConfig interface {
	Get() *ecv1beta1.RuntimeConfigSpec
	Set(spec *ecv1beta1.RuntimeConfigSpec)

	Cleanup()

	EmbeddedClusterHomeDirectory() string
	EmbeddedClusterTmpSubDir() string
	EmbeddedClusterBinsSubDir() string
	EmbeddedClusterChartsSubDir() string
	EmbeddedClusterChartsSubDirNoCreate() string
	EmbeddedClusterImagesSubDir() string
	EmbeddedClusterK0sSubDir() string
	EmbeddedClusterSeaweedFSSubDir() string
	EmbeddedClusterOpenEBSLocalSubDir() string
	PathToEmbeddedClusterBinary(name string) string
	PathToKubeConfig() string
	PathToKubeletConfig() string
	EmbeddedClusterSupportSubDir() string
	PathToEmbeddedClusterSupportFile(name string) string

	SetEnv() error
	WriteToDisk() error

	LocalArtifactMirrorPort() int
	AdminConsolePort() int
	ManagerPort() int
	ProxySpec() *ecv1beta1.ProxySpec
	NetworkInterface() string
	GlobalCIDR() string
	PodCIDR() string
	ServiceCIDR() string
	NodePortRange() string
	HostCABundlePath() string

	SetDataDir(dataDir string)
	SetLocalArtifactMirrorPort(port int)
	SetAdminConsolePort(port int)
	SetManagerPort(port int)
	SetProxySpec(proxySpec *ecv1beta1.ProxySpec)
	SetNetworkSpec(networkSpec ecv1beta1.NetworkSpec)
	SetHostCABundlePath(hostCABundlePath string)

	GetKubernetesEnvSettings() *helmcli.EnvSettings
}
