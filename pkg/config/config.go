package config

// Config is the configuration for the helmbin binary
type Config struct {
	// DataDir is the path to the data directory
	DataDir string
	// BinDir is the path to the bin directory
	BinDir string
	// RunDir is location of supervised pid files and sockets
	RunDir string
	// K0sConfigFile is the path to the k0s config file
	K0sConfigFile string
}
