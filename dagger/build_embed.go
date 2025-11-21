package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// EmbedRelease embeds the KOTS release into the binary.
//
// This is equivalent to ci-embed-release.sh and updates the build directory with:
// - Embedded binary tarball
// - Updated metadata JSON
//
// Example:
//
//	dagger call with-build-metadata \
//	  build-deps build-bin \
//	  embed-release \
//	  build-metadata to-dir export --path=./output
func (m *EmbeddedCluster) EmbedRelease(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
	// Release YAML directory containing Kubernetes manifests
	// +default="e2e/kots-release-install-v3"
	releaseYamlDir string,
	// Replicated app name
	// +default="embedded-cluster-smoke-test-staging-app"
	replicatedApp string,
	// S3 bucket for artifact URLs
	// +default="dev-embedded-cluster-bin"
	s3Bucket string,
	// Whether using dev bucket
	// +default=true
	usesDevBucket bool,
) (*EmbeddedCluster, error) {
	if m.BuildMetadata == nil {
		return nil, fmt.Errorf("build metadata not initialized - call WithBuildMetadata first")
	}

	if m.BuildMetadata.BuildDir == nil {
		return nil, fmt.Errorf("build directory not present - call BuildBin first")
	}

	embedded, err := m.embedRelease(
		ctx,
		src,
		m.BuildMetadata.BuildDir,
		m.BuildMetadata.Version,
		m.BuildMetadata.AppVersion,
		releaseYamlDir,
		replicatedApp,
		s3Bucket,
		usesDevBucket,
	)
	if err != nil {
		return nil, err
	}

	// Update metadata with binary file
	m.BuildMetadata.Binary = embedded

	return m, nil
}

// embedRelease embeds the release into the binary (wraps ci-embed-release.sh)
func (m *EmbeddedCluster) embedRelease(
	ctx context.Context,
	src *dagger.Directory,
	binaryDir *dagger.Directory,
	ecVersion string,
	appVersion string,
	releaseYamlDir string,
	replicatedApp string,
	s3Bucket string,
	usesDevBucket bool,
) (*dagger.File, error) {
	// Build the embedded-cluster-release-builder binary
	// This is needed by the ci-embed-release.sh script
	releaseBuilder := goBuildContainer().
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"sh", "-c", "CGO_ENABLED=0 go build -o /tmp/embedded-cluster-release-builder e2e/embedded-cluster-release-builder/main.go"}).
		File("/tmp/embedded-cluster-release-builder")

	// Use a container with the necessary tools
	container := ubuntuUtilsContainer().
		WithDirectory("/src/.git", src.Directory(".git")).
		WithDirectory("/src/scripts", src.Directory("scripts")).
		WithDirectory("/src/e2e/helm-charts", src.Directory("e2e/helm-charts")).
		WithDirectory(fmt.Sprintf("/src/%s", releaseYamlDir), src.Directory(releaseYamlDir)).
		WithDirectory("/src/build", binaryDir).
		WithWorkdir("/src").
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "/src"})

	// Extract the binary from the tarball to output/bin/embedded-cluster
	// The script expects the binary at output/bin/embedded-cluster
	container = container.
		WithExec([]string{"mkdir", "-p", "output/bin"}).
		WithExec([]string{"sh", "-c", "tar -xzf build/embedded-cluster-linux-*.tgz -C output/bin"}).
		WithFile("/src/output/bin/embedded-cluster-release-builder", releaseBuilder)

	// Install Helm
	container = container.
		WithExec([]string{"sh", "-c", "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"})

	// Install Replicated CLI
	container = container.
		WithExec([]string{"sh", "-c", "curl -fsSL https://raw.githubusercontent.com/replicatedhq/replicated/main/install.sh | bash"})

	// Set environment variables
	container = container.
		WithEnvVariable("EC_VERSION", ecVersion).
		WithEnvVariable("APP_VERSION", appVersion).
		WithEnvVariable("RELEASE_YAML_DIR", releaseYamlDir).
		WithEnvVariable("REPLICATED_APP", replicatedApp).
		WithEnvVariable("S3_BUCKET", s3Bucket)

	if usesDevBucket {
		container = container.WithEnvVariable("USES_DEV_BUCKET", "1")
	} else {
		container = container.WithEnvVariable("USES_DEV_BUCKET", "0")
	}

	// Run the embed release script
	container = container.
		WithExec([]string{"./scripts/ci-embed-release.sh"})

	filePath := fmt.Sprintf("bin/embedded-cluster-%s.tgz", ecVersion)

	// Create final tarball from the embedded binary
	// This matches the format from ci-upload-binaries.sh
	container = container.
		WithExec([]string{"mkdir", "-p", "bin"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("tar -C output/bin -czvf %s embedded-cluster", filePath)})

	return container.File(filePath), nil
}
