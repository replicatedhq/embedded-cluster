package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
	"strings"
)

const (
	TroubleshootVersion = "v0.122.0"
	HelmVersion         = "v3.19.2"
	FioVersion          = "3.41"
)

// Static builds all static binaries required for the embedded-cluster installer.
// This is the Dagger equivalent of `make static`.
//
// The function builds the following binaries:
//   - k0s: Kubernetes distribution
//   - kubectl-preflight: Pre-installation checks
//   - kubectl-support_bundle: Support bundle collection tool
//   - helm: Kubernetes package manager
//   - local-artifact-mirror: Local artifact mirror for airgap installations
//   - fio: I/O performance testing tool
//   - kubectl-kots: KOTS plugin for kubectl
//
// Examples:
//
//	# Build all binaries with default settings
//	dagger call static --kzeros-version=v1.33.6+k0s.0 --arch=amd64 export --path=./cmd/installer/goods/
//
//	# Build for ARM64 architecture
//	dagger call static --kzeros-version=v1.33.6+k0s.0 --arch=arm64 export --path=./cmd/installer/goods/
//
//	# Use custom k0s binary source
//	dagger call static --kzeros-version=v1.31.12+k0s.0-ec.0 --kzeros-binary-source-override=https://custom-url/k0s-binary --arch=amd64 export --path=./cmd/installer/goods/
//
//	# Use custom KOTS binary from URL
//	dagger call static --kzeros-version=v1.33.6+k0s.0 --kots-binary-urloverride=https://s3.amazonaws.com/kots.tar.gz --arch=amd64 export --path=./cmd/installer/goods/
func (m *EmbeddedCluster) Static(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// K0s version (e.g., v1.33.6+k0s.0)
	kzerosVersion string,
	// Architecture (amd64 or arm64)
	// +default="amd64"
	arch string,
	// K0s binary source override URL
	// +optional
	kzerosBinarySourceOverride string,
	// KOTS binary URL override (e.g., ttl.sh artifact or S3 URL)
	// +optional
	kotsBinaryURLOverride string,
	// KOTS binary file override (local file)
	// +optional
	kotsBinaryFileOverride *dagger.File,
) (*dagger.Directory, error) {
	dir := dag.Directory()

	k0sFile := m.GetKZerosBinary(kzerosVersion, arch, kzerosBinarySourceOverride)
	dir = dir.WithFile("bins/k0s", k0sFile)

	preflightFile := m.GetTroubleshootPreflightBinary(arch)
	dir = dir.WithFile("bins/kubectl-preflight", preflightFile)

	supportBundleFile := m.GetTroubleshootSupportBundleBinary(arch)
	dir = dir.WithFile("bins/kubectl-support_bundle", supportBundleFile)

	helmFile := m.GetHelmBinary(arch)
	dir = dir.WithFile("bins/helm", helmFile)

	lamFile := m.GetLocalArtifactMirrorBinary(src, arch)
	dir = dir.WithFile("bins/local-artifact-mirror", lamFile)

	fioFile := m.GetFioBinary(arch)
	dir = dir.WithFile("bins/fio", fioFile)

	kotsFile, err := m.GetKotsBinary(ctx, src, arch, kotsBinaryURLOverride, kotsBinaryFileOverride)
	if err != nil {
		return nil, fmt.Errorf("failed to get kots binary: %w", err)
	}
	dir = dir.WithFile("internal/bins/kubectl-kots", kotsFile)

	return dir, nil
}

// GetKZerosBinary downloads the k0s binary from GitHub releases or a custom source.
//
// The k0s binary is the core Kubernetes distribution used by embedded-cluster.
// By default, it downloads from the official k0sproject GitHub releases, but can
// be overridden with a custom URL for patched or custom builds.
//
// Examples:
//
//	# Download standard k0s binary for amd64
//	dagger call get-kzeros-binary --kzeros-version=v1.33.6+k0s.0 --arch=amd64 export --path=./cmd/installer/goods/bins/k0s
//
//	# Download for ARM64
//	dagger call get-kzeros-binary --kzeros-version=v1.33.6+k0s.0 --arch=arm64 export --path=./cmd/installer/goods/bins/k0s
//
//	# Use custom k0s binary source
//	dagger call get-kzeros-binary --kzeros-version=v1.31.12+k0s.0-ec.0 --kzeros-binary-source-override=https://custom-url/k0s --arch=amd64 export --path=./cmd/installer/goods/bins/k0s
func (m *EmbeddedCluster) GetKZerosBinary(
	// K0s version (e.g., v1.33.6+k0s.0)
	kzerosVersion string,
	// Architecture (amd64 or arm64)
	// +default="amd64"
	arch string,
	// K0s binary source override URL
	// +optional
	kzerosBinarySourceOverride string,
) *dagger.File {
	var url string
	if kzerosBinarySourceOverride != "" {
		url = kzerosBinarySourceOverride
	} else {
		// Remove the +k0s.X suffix for the download URL
		// e.g., v1.33.6+k0s.0 -> v1.33.6+k0s.0 (same for standard releases)
		url = fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/%s/k0s-%s-%s", kzerosVersion, kzerosVersion, arch)
	}

	return dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "add", "--no-cache", "curl"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("curl --retry 5 --retry-all-errors -fL -o /k0s '%s' && chmod +x /k0s", url)}).
		File("/k0s")
}

// GetTroubleshootPreflightBinary downloads troubleshoot preflight binary.
//
// Troubleshoot provides diagnostic tools for Kubernetes clusters:
//   - preflight: Runs pre-installation checks to validate environment
//   - support-bundle: Collects diagnostic information for troubleshooting
//
// Examples:
//
//	dagger call get-troubleshoot-preflight-binary --arch=amd64 export --path=./cmd/installer/goods/bins/kubectl-preflight
func (m *EmbeddedCluster) GetTroubleshootPreflightBinary(
	// Architecture (amd64 or arm64)
	// +default="amd64"
	arch string,
) *dagger.File {
	return m.getTroubleshootBinary(arch, "preflight")
}

// GetTroubleshootSupportBundleBinary downloads troubleshoot support-bundle binary.
//
// Troubleshoot provides diagnostic tools for Kubernetes clusters:
//   - preflight: Runs pre-installation checks to validate environment
//   - support-bundle: Collects diagnostic information for troubleshooting
//
// Examples:
//
//	dagger call get-troubleshoot-support-bundle-binary --arch=arm64 export --path=./cmd/installer/goods/bins/kubectl-support_bundle
func (m *EmbeddedCluster) GetTroubleshootSupportBundleBinary(
	// Architecture (amd64 or arm64)
	// +default="amd64"
	arch string,
) *dagger.File {
	return m.getTroubleshootBinary(arch, "support-bundle")
}

func (m *EmbeddedCluster) getTroubleshootBinary(
	arch string,
	binary string,
) *dagger.File {
	url := fmt.Sprintf("https://github.com/replicatedhq/troubleshoot/releases/download/%s/%s_linux_%s.tar.gz", TroubleshootVersion, binary, arch)

	return dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "add", "--no-cache", "curl", "tar"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("curl --retry 5 --retry-all-errors -fL -o /tmp/%s.tar.gz '%s'", binary, url)}).
		WithExec([]string{"tar", "-xzf", fmt.Sprintf("/tmp/%s.tar.gz", binary), "-C", "/tmp"}).
		WithExec([]string{"mv", fmt.Sprintf("/tmp/%s", binary), fmt.Sprintf("/kubectl-%s", binary)}).
		File(fmt.Sprintf("/kubectl-%s", binary))
}

// GetHelmBinary downloads the Helm binary from the official Helm releases.
//
// Helm is the Kubernetes package manager used to deploy and manage applications
// in the embedded cluster.
//
// Examples:
//
//	# Download Helm binary for amd64
//	dagger call get-helm-binary --arch=amd64 export --path=./cmd/installer/goods/bins/helm
//
//	# Download for ARM64
//	dagger call get-helm-binary --arch=arm64 export --path=./cmd/installer/goods/bins/helm
func (m *EmbeddedCluster) GetHelmBinary(
	// Architecture (amd64 or arm64)
	// +default="amd64"
	arch string,
) *dagger.File {
	url := fmt.Sprintf("https://get.helm.sh/helm-%s-linux-%s.tar.gz", HelmVersion, arch)

	return dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "add", "--no-cache", "curl", "tar"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("curl --retry 5 --retry-all-errors -fL -o /tmp/helm.tar.gz '%s'", url)}).
		WithExec([]string{"tar", "-xzf", "/tmp/helm.tar.gz", "-C", "/tmp"}).
		WithExec([]string{"mv", fmt.Sprintf("/tmp/linux-%s/helm", arch), "/helm"}).
		File("/helm")
}

// GetLocalArtifactMirrorBinary builds the local-artifact-mirror binary from source.
//
// The local-artifact-mirror serves container images and other artifacts locally
// for airgap installations where internet access is not available.
//
// Examples:
//
//	# Build for amd64
//	dagger call get-local-artifact-mirror-binary --arch=amd64 export --path=./cmd/installer/goods/bins/local-artifact-mirror
//
//	# Build for ARM64
//	dagger call get-local-artifact-mirror-binary --arch=arm64 export --path=./cmd/installer/goods/bins/local-artifact-mirror
func (m *EmbeddedCluster) GetLocalArtifactMirrorBinary(
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Architecture (amd64 or arm64)
	// +default="amd64"
	arch string,
) *dagger.File {
	goos := "linux"

	return dag.Container().
		From(fmt.Sprintf("golang:%s", GoVersion)).
		// Cache Go modules and build artifacts to speed up subsequent builds
		WithMountedCache("/cache/go/mod", dag.CacheVolume("ec-gomod-cache"), dagger.ContainerWithMountedCacheOpts{
			Sharing: dagger.CacheSharingModeShared,
		}).
		WithMountedCache("/cache/go/build", dag.CacheVolume("ec-gobuild-cache"), dagger.ContainerWithMountedCacheOpts{
			Sharing: dagger.CacheSharingModeShared,
		}).
		WithWorkdir("/workspace/local-artifact-mirror").
		WithDirectory("/workspace", localArtifactMirrorDirectory(src)).
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", goos).
		WithEnvVariable("GOARCH", arch).
		WithEnvVariable("GOCACHE", "/cache/go/mod").
		WithEnvVariable("GOMODCACHE", "/cache/go/mod").
		WithExec([]string{"sh", "-c", fmt.Sprintf("make build OS=%s ARCH=%s", goos, arch)}).
		File(fmt.Sprintf("/workspace/local-artifact-mirror/bin/local-artifact-mirror-%s-%s", goos, arch))
}

// GetFioBinary builds the fio (Flexible I/O Tester) binary from source.
//
// FIO is used for I/O performance testing and validation during installation.
// The binary is compiled statically to ensure compatibility across different Linux distributions.
//
// Examples:
//
//	# Build for amd64
//	dagger call get-fio-binary --arch=amd64 export --path=./cmd/installer/goods/bins/fio
//
//	# Build for ARM64
//	dagger call get-fio-binary --arch=arm64 export --path=./cmd/installer/goods/bins/fio
func (m *EmbeddedCluster) GetFioBinary(
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
		WithExec([]string{"curl", "-fsSL", "-o", "fio.tar.gz", fmt.Sprintf("https://api.github.com/repos/axboe/fio/tarball/fio-%s", FioVersion)}).
		WithExec([]string{"tar", "-xzf", "fio.tar.gz", "--strip-components=1"}).
		WithExec([]string{"./configure", "--build-static", "--disable-native"}).
		WithExec([]string{"sh", "-c", "make -j$(nproc)"})

	// Extract the binary
	return buildContainer.File("/fio/fio")
}

// GetKotsBinary gets the KOTS (Kubernetes Off-The-Shelf) binary from various sources.
//
// KOTS is used for managing application deployments in the embedded cluster.
// This function mimics the Makefile logic for discovering and obtaining the KOTS binary:
//  1. If a file override is provided, use that file directly
//  2. If a URL override is provided, download from that URL (supports HTTP/HTTPS and oras)
//  3. Otherwise, read the version from metadata.yaml and extract from kotsadm image
//
// Examples:
//
//	# Get KOTS binary using default metadata.yaml version
//	dagger call get-kots-binary --arch=amd64 export --path=./cmd/installer/goods/internal/bins/kubectl-kots
//
//	# Use custom KOTS binary from S3
//	dagger call get-kots-binary --arch=amd64 --kots-binary-urloverride=https://s3.amazonaws.com/kots.tar.gz export --path=./cmd/installer/goods/internal/bins/kubectl-kots
//
//	# Use local KOTS binary file
//	dagger call get-kots-binary --arch=amd64 --kots-binary-file-override=file:./local-kots export --path=./cmd/installer/goods/internal/bins/kubectl-kots
func (m *EmbeddedCluster) GetKotsBinary(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Architecture (amd64 or arm64)
	// +default="amd64"
	arch string,
	// KOTS binary URL override (e.g., ttl.sh artifact or S3 URL)
	// +optional
	kotsBinaryURLOverride string,
	// KOTS binary file override (local file)
	// +optional
	kotsBinaryFileOverride *dagger.File,
) (*dagger.File, error) {
	// If file override is provided, use it directly
	if kotsBinaryFileOverride != nil {
		return kotsBinaryFileOverride, nil
	}

	// If URL override is provided, download from it
	if kotsBinaryURLOverride != "" {
		c := dag.Container().
			From("alpine:latest").
			WithExec([]string{"apk", "add", "--no-cache", "curl", "tar"})

		// Check if it's HTTP/HTTPS or oras URL
		if strings.HasPrefix(kotsBinaryURLOverride, "http://") || strings.HasPrefix(kotsBinaryURLOverride, "https://") {
			// Download via HTTP/HTTPS
			return c.
				WithExec([]string{"sh", "-c", fmt.Sprintf("curl --retry 5 --retry-all-errors -fL -o /tmp/kots.tar.gz '%s'", kotsBinaryURLOverride)}).
				WithExec([]string{"tar", "-xzf", "/tmp/kots.tar.gz", "-C", "/tmp"}).
				WithExec([]string{"mv", "/tmp/kots", "/kots"}).
				WithExec([]string{"chmod", "+x", "/kots"}).
				File("/kots"), nil
		}

		// Download via oras
		return c.
			WithExec([]string{"sh", "-c", "curl -fsSL https://github.com/oras-project/oras/releases/latest/download/oras_linux_amd64.tar.gz | tar -xz -C /usr/local/bin oras"}).
			WithExec([]string{"sh", "-c", fmt.Sprintf("oras pull '%s' --output /tmp", kotsBinaryURLOverride)}).
			WithExec([]string{"tar", "-xzf", "/tmp/kots.tar.gz", "-C", "/tmp"}).
			WithExec([]string{"mv", "/tmp/kots", "/kots"}).
			WithExec([]string{"chmod", "+x", "/kots"}).
			File("/kots"), nil
	}

	// Read version from metadata.yaml
	metadataContent, err := src.File("pkg/addons/adminconsole/static/metadata.yaml").Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata.yaml: %w", err)
	}

	// Extract version using awk-like parsing
	kotsVersion, err := dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "add", "--no-cache", "gawk"}).
		WithNewFile("/metadata.yaml", metadataContent).
		WithExec([]string{"sh", "-c", "awk '/^version/{print $2}' /metadata.yaml | sed -E 's/([0-9]+\\.[0-9]+\\.[0-9]+)(-ec\\.[0-9]+)?.*/\\1\\2/'"}).
		Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	kotsVersion = strings.TrimSpace(kotsVersion)
	if !strings.HasPrefix(kotsVersion, "v") {
		kotsVersion = "v" + kotsVersion
	}

	// Use crane to export the kotsadm image and extract the kots binary
	return dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "add", "--no-cache", "curl", "tar"}).
		WithExec([]string{"sh", "-c", "curl -fL https://github.com/google/go-containerregistry/releases/latest/download/go-containerregistry_Linux_x86_64.tar.gz | tar -xz -C /usr/local/bin crane"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("crane export kotsadm/kotsadm:%s --platform linux/%s - | tar -Oxf - kots > /kots", kotsVersion, arch)}).
		WithExec([]string{"chmod", "+x", "/kots"}).
		File("/kots"), nil
}
