/*
Package config provides the configuration for the helmbin binary
*/
package config

// Config is the configuration for the helmbin binary
type Config struct {
	// DataDir is the path to the data directory
	DataDir string

	// BinDir is the path to the bin directory
	BinDir string

	// RunDir is location of supervised pid files and sockets
	RunDir string

	// K3sConfigFile is the path to the k3s config file
	K3sConfigFile string

	// K0sConfigFile is the path to the k0s config file
	K0sConfigFile string

	// KubeconfigPath is the path to the kubeconfig file
	KubeconfigPath string
}
