package config

import "path/filepath"

// Default returns the default configuration
func Default() Config {
	return Config{
		DataDir:        DataDirDefault,
		BinDir:         filepath.Join(DataDirDefault, "bin"),
		RunDir:         filepath.Join(DataDirDefault, "run"),
		K0sConfigFile:  "/etc/replicated/k0s/config.yaml",
		K3sConfigFile:  "/etc/replicated/k3s/config.yaml",
		KubeconfigPath: "/etc/replicated/kubeconfig.yaml",
	}
}
