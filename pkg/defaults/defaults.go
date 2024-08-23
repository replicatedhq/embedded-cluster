// Package defaults holds default values for the embedded-cluster binary. For sake of
// keeping everything simple this packages exits(1) if some error occurs as
// these should not happen in the first place.
package defaults

var (
	// provider holds a global reference to the default provider.
	provider *Provider
)

// Holds the default no proxy values.
var DefaultNoProxy = []string{"localhost", "127.0.0.1", ".default", ".local", ".svc", "kubernetes", "kotsadm-rqlite"}

var ProxyRegistryAddress = "proxy.replicated.com"

const KotsadmNamespace = "kotsadm"
const SeaweedFSNamespace = "seaweedfs"
const RegistryNamespace = "registry"
const VeleroNamespace = "velero"

// def returns a global reference to the default provider. creates one if not
// already created.
func def() *Provider {
	if provider == nil {
		provider = NewProvider("")
	}
	return provider
}

// BinaryName calls BinaryName on the default provider.
func BinaryName() string {
	return def().BinaryName()
}

// EmbeddedClusterBinsSubDir calls EmbeddedClusterBinsSubDir on the default provider.
func EmbeddedClusterBinsSubDir() string {
	return def().EmbeddedClusterBinsSubDir()
}

// EmbeddedClusterChartsSubDir calls EmbeddedClusterChartsSubDir on the default provider.
func EmbeddedClusterChartsSubDir() string {
	return def().EmbeddedClusterChartsSubDir()
}

// EmbeddedClusterImagesSubDir calls EmbeddedClusterImagesSubDir on the default provider.
func EmbeddedClusterImagesSubDir() string {
	return def().EmbeddedClusterImagesSubDir()
}

// EmbeddedClusterLogsSubDir calls EmbeddedClusterLogsSubDir on the default provider.
func EmbeddedClusterLogsSubDir() string {
	return def().EmbeddedClusterLogsSubDir()
}

// K0sBinaryPath calls K0sBinaryPath on the default provider.
func K0sBinaryPath() string {
	return def().K0sBinaryPath()
}

// PathToEmbeddedClusterBinary calls PathToEmbeddedClusterBinary on the default provider.
func PathToEmbeddedClusterBinary(name string) string {
	return def().PathToEmbeddedClusterBinary(name)
}

// PathToLog calls PathToLog on the default provider.
func PathToLog(name string) string {
	return def().PathToLog(name)
}

// PathToKubeConfig calls PathToKubeConfig on the default provider.
func PathToKubeConfig() string {
	return def().PathToKubeConfig()
}

// PreferredNodeIPAddress calls PreferredNodeIPAddress on the default provider.
func PreferredNodeIPAddress() (string, error) {
	return def().PreferredNodeIPAddress()
}

// TryDiscoverPublicIP calls TryDiscoverPublicIP on the default provider.
func TryDiscoverPublicIP() string {
	return def().TryDiscoverPublicIP()
}

// PathToK0sConfig calls PathToK0sConfig on the default provider.
func PathToK0sConfig() string {
	return def().PathToK0sConfig()
}

// PathToK0sStatusSocket calls PathToK0sStatusSocket on the default provider.
func PathToK0sStatusSocket() string {
	return def().PathToK0sStatusSocket()
}

func PathToK0sContainerdConfig() string {
	return def().PathToK0sContainerdConfig()
}

// EmbeddedClusterHomeDirectory calls EmbeddedClusterHomeDirectory on the default provider.
func EmbeddedClusterHomeDirectory() string {
	return def().EmbeddedClusterHomeDirectory()
}

// PathToEmbeddedClusterSupportFile calls PathToEmbeddedClusterSupportFile on the default provider.
func PathToEmbeddedClusterSupportFile(name string) string {
	return def().PathToEmbeddedClusterSupportFile(name)
}
