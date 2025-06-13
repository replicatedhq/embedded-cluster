package runtimeconfig

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
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
	EmbeddedClusterSeaweedfsSubDir() string
	EmbeddedClusterOpenEBSLocalSubDir() string
	PathToEmbeddedClusterBinary(name string) string
	PathToKubeConfig() string
	PathToKubeletConfig() string
	EmbeddedClusterSupportSubDir() string
	PathToEmbeddedClusterSupportFile(name string) string
	WriteToDisk() error
	LocalArtifactMirrorPort() int
	AdminConsolePort() int
	ManagerPort() int
	HostCABundlePath() string
	SetDataDir(dataDir string)
	SetLocalArtifactMirrorPort(port int)
	SetAdminConsolePort(port int)
	SetManagerPort(port int)
	SetHostCABundlePath(hostCABundlePath string)
}
