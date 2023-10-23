// Package defaults holds default values for the embedded-cluster binary. For sake of
// keeping everything simple this packages exits(1) if some error occurs as
// these should not happen in the first place.
package defaults

var (
	// Version holds the HelmVM version.
	Version = "v0.0.0"
	// K0sVersion holds the version of k0s binary we are embedding. this is
	// set at compile time via ldflags.
	K0sVersion = "0.0.0"
	// provider holds a global reference to the default provider.
	provider *Provider
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

// K0sctlBinsSubDir calls K0sctlBinsSubDir on the default provider.
func K0sctlBinsSubDir() string {
	return def().K0sctlBinsSubDir()
}

// HelmVMBinsSubDir calls HelmVMBinsSubDir on the default provider.
func HelmVMBinsSubDir() string {
	return def().HelmVMBinsSubDir()
}

// HelmVMLogsSubDir calls HelmVMLogsSubDir on the default provider.
func HelmVMLogsSubDir() string {
	return def().HelmVMLogsSubDir()
}

// K0sctlApplyLogPath calls K0sctlApplyLogPath on the default provider.
func K0sctlApplyLogPath() string {
	return def().K0sctlApplyLogPath()
}

// SSHKeyPath calls SSHKeyPath on the default provider.
func SSHKeyPath() string {
	return def().SSHKeyPath()
}

// SSHAuthorizedKeysPath calls SSHAuthorizedKeysPath on the default provider.
func SSHAuthorizedKeysPath() string {
	return def().SSHAuthorizedKeysPath()
}

// ConfigSubDir calls ConfigSubDir on the default provider.
func ConfigSubDir() string {
	return def().ConfigSubDir()
}

// K0sBinaryPath calls K0sBinaryPath on the default provider.
func K0sBinaryPath() string {
	return def().K0sBinaryPath()
}

// PathToK0sctlBinary calls PathToK0sctlBinary on the default provider.
func PathToK0sctlBinary(name string) string {
	return def().PathToK0sctlBinary(name)
}

// PathToHelmVMBinary calls PathToHelmVMBinary on the default provider.
func PathToHelmVMBinary(name string) string {
	return def().PathToHelmVMBinary(name)
}

// PathToLog calls PathToLog on the default provider.
func PathToLog(name string) string {
	return def().PathToLog(name)
}

// PathToConfig calls PathToConfig on the default provider.
func PathToConfig(name string) string {
	return def().PathToConfig(name)
}

// FileNameForImage calls FileNameForImage on the default provider.
func FileNameForImage(img string) string {
	return def().FileNameForImage(img)
}

// PreferredNodeIPAddress calls PreferredNodeIPAddress on the default provider.
func PreferredNodeIPAddress() (string, error) {
	return def().PreferredNodeIPAddress()
}

// TryDiscoverPublicIP calls TryDiscoverPublicIP on the default provider.
func TryDiscoverPublicIP() string {
	return def().TryDiscoverPublicIP()
}

// DecentralizedInstall calls DecentralizedInstall on the default provider.
func DecentralizedInstall() bool {
	return def().DecentralizedInstall()
}

// SetInstallAsDecentralized calls SetInstallAsDecentralized on the default provider.
func SetInstallAsDecentralized() error {
	return def().SetInstallAsDecentralized()
}

// HelmChartSubDir calls HelmChartSubDir on the default provider.
func HelmChartSubDir() string {
	return def().HelmChartSubDir()
}

// PathToHelmChart calls PathToHelmChart on the default provider.
func PathToHelmChart(name string, version string) string {
	return def().PathToHelmChart(name, version)
}

// IsUpgrade determines if we are upgrading a cluster judging by the existence
// or not of a kubeconfig file in the configuration directory.
func IsUpgrade() bool {
	return def().IsUpgrade()
}
