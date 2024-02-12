// Package defaults holds default values for the embedded-cluster binary. For sake of
// keeping everything simple this packages exits(1) if some error occurs as
// these should not happen in the first place.
package defaults

var (
	// Version holds the EmbeddedCluster version.
	Version = "v0.0.0"
	// K0sVersion holds the version of k0s binary we are embedding. this is
	// set at compile time via ldflags.
	K0sVersion = "0.0.0"
	// provider holds a global reference to the default provider.
	provider *Provider
	// K0sBinaryURL holds an alternative URL from where to download the k0s
	// binary that has been embedded in this version of the binary. If this
	// is empty then it means we have shipped the official k0s binary. This
	// is set at compile time via ldflags.
	K0sBinaryURL = ""
)

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

// EmbeddedClusterLogsSubDir calls EmbeddedClusterLogsSubDir on the default provider.
func EmbeddedClusterLogsSubDir() string {
	return def().EmbeddedClusterLogsSubDir()
}

// EmbeddedClusterConfigSubDir calls ConfigSubDir on the default provider.
func EmbeddedClusterConfigSubDir() string {
	return def().EmbeddedClusterConfigSubDir()
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

// PathToConfig calls PathToConfig on the default provider.
func PathToConfig(name string) string {
	return def().PathToConfig(name)
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

// EmbeddedClusterHomeDirectory calls EmbeddedClusterHomeDirectory on the default provider.
func EmbeddedClusterHomeDirectory() string {
	return def().EmbeddedClusterHomeDirectory()
}
