package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// BuildMetadata represents the metadata of a build process.
// This is used as a composable data structure throughout the build pipeline.
type BuildMetadata struct {
	// EC version (e.g., "v1.0.0")
	Version string
	// App version (e.g., "appver-dev-abc123")
	AppVersion string
	// K0s version (full version like "v1.31.1+k0s.0")
	K0SVersion string
	// K0s minor version (e.g., "31")
	K0SMinorVersion string
	// Architecture (e.g., "amd64")
	Arch string
	// Operator image repository (e.g., "ttl.sh/user/embedded-cluster-operator-image")
	OperatorImageRepo string
	// Operator image tag (e.g., "v2.12.0-k8s-1.33")
	OperatorImageTag string
	// Local artifact mirror image repository (e.g., "ttl.sh/user/embedded-cluster-local-artifact-mirror")
	LAMImageRepo string
	// Local artifact mirror image tag
	LAMImageTag string
	// Operator chart URL (e.g., "oci://ttl.sh/user/embedded-cluster-operator")
	OperatorChartURL string
	// Build directory path (e.g., "build")
	BuildDirPath string
	// Binary file path (e.g., "embedded-cluster-v1.0.0.tgz")
	BinaryPath string

	// Build directory containing all artifacts
	BuildDir *dagger.Directory `yaml:"-"`
	// Binary file
	Binary *dagger.File `yaml:"-"`
}

// WithBuildMetadata initializes the build metadata.
// This is used as a composable function throughout the build pipeline.
//
// Example (new build):
//
//	dagger call with-build-metadata \
//	  build-deps build-bin \
//	  build-metadata to-dir export --path ./output
//
// Example (resume from saved state):
//
//	dagger call with-build-metadata --dir ./output \
//	  build-metadata to-dir export --path ./output
func (m *EmbeddedCluster) WithBuildMetadata(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
	// Directory containing build metadata and artifacts (from a previous ToDir() export)
	// +optional
	dir *dagger.Directory,
	// Version for the embedded-cluster binary (auto-detected from git if not provided)
	// +optional
	ecVersion string,
	// App version label for the Replicated release (e.g., "appver-dev-abc123" or auto-detected from git if not provided)
	// +optional
	appVersion string,
	// K0s minor version (e.g., "33" or auto-detected from versions.mk if not provided)
	// +optional
	k0sMinorVersion string,
	// Architecture to build for (defaults to amd64)
	// +default="amd64"
	arch string,
) (*EmbeddedCluster, error) {
	metadata := &BuildMetadata{}

	if dir != nil {
		// Read metadata.yaml from the directory
		content, err := dir.File("metadata.yaml").Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata.yaml from directory: %w", err)
		}
		err = yaml.Unmarshal([]byte(content), &metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata yaml: %w", err)
		}

		// Restore BuildDir if present
		if metadata.BuildDirPath != "" {
			metadata.BuildDir = dir.Directory(metadata.BuildDirPath)
		}

		// Restore Binary if present
		if metadata.BinaryPath != "" {
			metadata.Binary = dir.File(metadata.BinaryPath)
		}
	}

	var err error
	metadata, err = metadata.WithVersion(ctx, src, ecVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to set version: %w", err)
	}
	metadata, err = metadata.WithAppVersion(ctx, src, appVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to set app version: %w", err)
	}
	metadata, err = metadata.WithK0sVersion(ctx, src, k0sMinorVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to set k0s version: %w", err)
	}
	metadata, err = metadata.WithArch(ctx, arch)
	if err != nil {
		return nil, fmt.Errorf("failed to set arch: %w", err)
	}

	m.BuildMetadata = metadata

	return m, nil
}

// GetVersion returns the EC version, detecting it from git if not set.
func (m *BuildMetadata) WithVersion(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
	// Version for the embedded-cluster binary (auto-detected from git if not provided)
	// +optional
	version string,
) (*BuildMetadata, error) {
	if version != "" {
		m.Version = version
	}

	if m.Version != "" {
		return m, nil
	}

	container := ubuntuUtilsContainer().
		WithDirectory("/src", src).
		WithWorkdir("/src")

	// Get EC_VERSION from git describe
	ecVersion, err := container.
		WithExec([]string{"git", "describe", "--tags", "--match=[0-9]*.[0-9]*.[0-9]*", "--abbrev=4"}).
		Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect EC_VERSION: %w", err)
	}
	ecVersion = strings.TrimSpace(ecVersion)

	m.Version = ecVersion
	return m, nil
}

// WithAppVersion returns the app version, detecting it from git if not set.
func (m *BuildMetadata) WithAppVersion(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
	// App version label for the Replicated release (e.g., "appver-dev-abc123" or auto-detected from git)
	// +optional
	appVersion string,
) (*BuildMetadata, error) {
	if appVersion != "" {
		m.AppVersion = appVersion
	}

	if m.AppVersion != "" {
		return m, nil
	}

	container := ubuntuUtilsContainer().
		WithDirectory("/src", src).
		WithWorkdir("/src")

	// Get APP_VERSION from git rev-parse
	shortSha, err := container.
		WithExec([]string{"git", "rev-parse", "--short", "HEAD"}).
		Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect APP_VERSION: %w", err)
	}
	shortSha = strings.TrimSpace(shortSha)
	appVersion = fmt.Sprintf("appver-dev-%s", shortSha)

	m.AppVersion = appVersion
	return m, nil
}

// WithK0sVersion returns the K0s version, detecting it from versions.mk if not set.
func (m *BuildMetadata) WithK0sVersion(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
	// K0s minor version (e.g., "33" or auto-detected from versions.mk if not provided)
	// +optional
	k0sMinorVersion string,
) (*BuildMetadata, error) {
	if k0sMinorVersion != "" {
		m.K0SMinorVersion = k0sMinorVersion
	}

	if m.K0SMinorVersion != "" {
		return m, nil
	}

	minorVersion, err := ubuntuUtilsContainer().
		WithFile("/src/versions.mk", src.File("versions.mk")).
		WithFile("/src/common.mk", src.File("common.mk")).
		WithFile("/src/Makefile", src.File("Makefile")).
		WithWorkdir("/src").
		WithExec([]string{"make", "print-K0S_MINOR_VERSION"}).
		Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect K0S_MINOR_VERSION: %w", err)
	}
	minorVersion = strings.TrimSpace(minorVersion)

	version, err := ubuntuUtilsContainer().
		WithFile("/src/versions.mk", src.File("versions.mk")).
		WithFile("/src/common.mk", src.File("common.mk")).
		WithFile("/src/Makefile", src.File("Makefile")).
		WithEnvVariable("K0S_MINOR_VERSION", minorVersion).
		WithWorkdir("/src").
		WithExec([]string{"make", "print-K0S_VERSION"}).
		Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect K0S_VERSION: %w", err)
	}
	version = strings.TrimSpace(version)

	m.K0SVersion = version
	m.K0SMinorVersion = minorVersion
	return m, nil
}

// WithArch returns the architecture, detecting it from the build environment if not set.
func (m *BuildMetadata) WithArch(
	ctx context.Context,
	// Architecture to build for (defaults to amd64)
	// +default="amd64"
	arch string,
) (*BuildMetadata, error) {
	if arch != "" {
		m.Arch = arch
	}

	if m.Arch != "" {
		return m, nil
	}

	arch, err := ubuntuUtilsContainer().
		WithExec([]string{"uname", "-m"}).
		Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect architecture: %w", err)
	}
	arch = strings.TrimSpace(arch)

	m.Arch = arch
	return m, nil
}

// ToDir exports the build metadata and all build artifacts to a directory.
//
// The directory will contain:
// - metadata.yaml: Serialized BuildMetadata struct
// - BuildDir contents (if present): all build artifacts
//
// Example:
//
//	dagger call with-build-metadata build-deps build-bin \
//	  build-metadata to-dir export --path ./output
func (m *BuildMetadata) ToDir() (*dagger.Directory, error) {
	dir := dag.Directory()

	// Set the build directory path before marshaling if build directory is present
	if m.BuildDir != nil {
		m.BuildDirPath = "build"
		dir = dir.WithDirectory(m.BuildDirPath, m.BuildDir)
	}

	// Set the binary file path before marshaling if binary file is present
	if m.Binary != nil {
		m.BinaryPath = fmt.Sprintf("embedded-cluster-%s.tgz", m.Version)
		dir = dir.WithFile(m.BinaryPath, m.Binary)
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	dir = dir.
		WithNewFile("metadata.yaml", string(data))

	return dir, nil
}

// withEnvVariables sets the environment variables from the build metadata in the container.
func (m *BuildMetadata) withEnvVariables(c *dagger.Container) *dagger.Container {
	return c.WithEnvVariable("EC_VERSION", m.Version).
		WithEnvVariable("APP_VERSION", m.AppVersion).
		WithEnvVariable("K0S_MINOR_VERSION", m.K0SMinorVersion).
		WithEnvVariable("K0S_VERSION", m.K0SVersion).
		WithEnvVariable("ARCH", m.Arch)
}
