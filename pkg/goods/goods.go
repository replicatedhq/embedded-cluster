// Package goods handles embedded static assets. Things like writing them
// down to disk, return them as a parsed list, etc.
package goods

import (
	"embed"
)

var (
	// materializer is our default instace of the artifact materializer.
	materializer = NewMaterializer("")
	//go:embed bins/*
	binfs embed.FS
	//go:embed support/*
	supportfs embed.FS
	//go:embed systemd/*
	systemdfs embed.FS
	//go:embed internal/bins/*
	internalBinfs embed.FS
)

// K0sBinarySHA256 returns the SHA256 checksum of the embedded k0s binary.
func K0sBinarySHA256() (string, error) {
	return materializer.K0sBinarySHA256()
}

// Materialize writes to disk all embedded assets using the default materializer.
func Materialize() error {
	return materializer.Materialize()
}

// MaterializeCalicoNetworkManagerConfig is a helper function that uses the default materializer.
func MaterializeCalicoNetworkManagerConfig() error {
	return materializer.CalicoNetworkManagerConfig()
}

// MaterializeLocalArtifactMirrorUnitFile is a helper function that uses the default materializer.
func MaterializeLocalArtifactMirrorUnitFile() error {
	return materializer.LocalArtifactMirrorUnitFile()
}

// MaterializeInternalBinary is a helper for the default materializer.
func MaterializeInternalBinary(name string) (string, error) {
	return materializer.InternalBinary(name)
}
