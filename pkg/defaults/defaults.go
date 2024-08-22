// Package defaults holds default values for the embedded-cluster binary. For sake of
// keeping everything simple this packages exits(1) if some error occurs as
// these should not happen in the first place.
package defaults

var (
	// DefaultProvider holds the default provider and is used by the exported functions.
	DefaultProvider = NewProvider("")
)

var (
	// provider holds a global reference to the default provider.
	provider *Provider
)

// Holds the default no proxy values.
var DefaultNoProxy = []string{"localhost", "127.0.0.1", ".default", ".local", ".svc", "kubernetes", "kotsadm-rqlite"}

const ProxyRegistryAddress = "proxy.replicated.com"

const KotsadmNamespace = "kotsadm"
const SeaweedFSNamespace = "seaweedfs"
const RegistryNamespace = "registry"
const VeleroNamespace = "velero"

// BinaryName calls BinaryName on the default provider.
func BinaryName() string {
	return DefaultProvider.BinaryName()
}

// EmbeddedClusterBinsSubDir calls EmbeddedClusterBinsSubDir on the default provider.
func EmbeddedClusterBinsSubDir() string {
	return DefaultProvider.EmbeddedClusterBinsSubDir()
}

// EmbeddedClusterChartsSubDir calls EmbeddedClusterChartsSubDir on the default provider.
func EmbeddedClusterChartsSubDir() string {
	return DefaultProvider.EmbeddedClusterChartsSubDir()
}

// EmbeddedClusterImagesSubDir calls EmbeddedClusterImagesSubDir on the default provider.
func EmbeddedClusterImagesSubDir() string {
	return DefaultProvider.EmbeddedClusterImagesSubDir()
}

// EmbeddedClusterLogsSubDir calls EmbeddedClusterLogsSubDir on the default provider.
func EmbeddedClusterLogsSubDir() string {
	return DefaultProvider.EmbeddedClusterLogsSubDir()
}

// K0sBinaryPath calls K0sBinaryPath on the default provider.
func K0sBinaryPath() string {
	return DefaultProvider.K0sBinaryPath()
}

// PathToEmbeddedClusterBinary calls PathToEmbeddedClusterBinary on the default provider.
func PathToEmbeddedClusterBinary(name string) string {
	return DefaultProvider.PathToEmbeddedClusterBinary(name)
}

// PathToLog calls PathToLog on the default provider.
func PathToLog(name string) string {
	return DefaultProvider.PathToLog(name)
}

// PathToKubeConfig calls PathToKubeConfig on the default provider.
func PathToKubeConfig() string {
	return DefaultProvider.PathToKubeConfig()
}

// PreferredNodeIPAddress calls PreferredNodeIPAddress on the default provider.
func PreferredNodeIPAddress() (string, error) {
	return DefaultProvider.PreferredNodeIPAddress()
}

// TryDiscoverPublicIP calls TryDiscoverPublicIP on the default provider.
func TryDiscoverPublicIP() string {
	return DefaultProvider.TryDiscoverPublicIP()
}

// PathToK0sConfig calls PathToK0sConfig on the default provider.
func PathToK0sConfig() string {
	return DefaultProvider.PathToK0sConfig()
}

// PathToK0sStatusSocket calls PathToK0sStatusSocket on the default provider.
func PathToK0sStatusSocket() string {
	return DefaultProvider.PathToK0sStatusSocket()
}

func PathToK0sContainerdConfig() string {
	return DefaultProvider.PathToK0sContainerdConfig()
}

// EmbeddedClusterHomeDirectory calls EmbeddedClusterHomeDirectory on the default provider.
func EmbeddedClusterHomeDirectory() string {
	return DefaultProvider.EmbeddedClusterHomeDirectory()
}

// PathToEmbeddedClusterSupportFile calls PathToEmbeddedClusterSupportFile on the default provider.
func PathToEmbeddedClusterSupportFile(name string) string {
	return DefaultProvider.PathToEmbeddedClusterSupportFile(name)
}
