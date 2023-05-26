package config

import (
	"path"
	"path/filepath"
)

// DataDirDefault is the default path to the data directory
const DataDirDefault = "/var/lib/replicated"

// Default returns the default configuration
func Default() Config {
	return Config{
		DataDir:       DataDirDefault,
		BinDir:        filepath.Join(DataDirDefault, "bin"),
		RunDir:        filepath.Join(DataDirDefault, "run"),
		K0sConfigFile: path.Join(DataDirDefault, "config.yaml"),
	}
}
