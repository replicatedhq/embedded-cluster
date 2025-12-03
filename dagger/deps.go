package main

import (
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// BuildFio builds the fio binary
//
// This can be called standalone or used internally by the build process.
//
// Example:
//
//	dagger call build-fio --version=3.41 --arch=amd64 export --path=./fio
func (m *EmbeddedCluster) BuildFio(
	// FIO version to build
	version string,
	// Architecture to build for (amd64 or arm64)
	// +default="amd64"
	arch string,
) *dagger.File {
	// Map arch to Dagger platform
	var platform dagger.Platform
	switch arch {
	case "amd64":
		platform = "linux/amd64"
	case "arm64":
		platform = "linux/arm64"
	default:
		platform = "linux/amd64"
	}

	// Build stage - compile fio from source
	buildContainer := ubuntuUtilsContainer(dagger.ContainerOpts{Platform: platform}).
		WithExec([]string{"mkdir", "-p", "/fio"}).
		WithWorkdir("/fio").
		WithExec([]string{"curl", "-fsSL", "-o", "fio.tar.gz", fmt.Sprintf("https://api.github.com/repos/axboe/fio/tarball/fio-%s", version)}).
		WithExec([]string{"tar", "-xzf", "fio.tar.gz", "--strip-components=1"}).
		WithExec([]string{"./configure", "--build-static", "--disable-native"}).
		WithExec([]string{"sh", "-c", "make -j$(nproc)"})

	// Extract the binary
	return buildContainer.File("/fio/fio")
}

// ubuntuUtilsContainer returns a container with the necessary tools for building.
func ubuntuUtilsContainer(opts ...dagger.ContainerOpts) *dagger.Container {
	return dag.Container(opts...).
		From("ubuntu:24.04").
		WithEnvVariable("TZ", "Etc/UTC").
		WithEnvVariable("DEBIAN_FRONTEND", "noninteractive").
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y",
			"build-essential", "cmake", "curl", "gettext", "git", "gzip", "jq", "libstdc++6", "make", "pkg-config", "tar", "unzip",
		}).
		// Set the working directory to /workspace
		WithWorkdir("/workspace").
		// Configure Git to allow unsafe directories
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "/workspace"})

}
