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
	// Replicated app channel ID
	// +default="2lhrq5LDyoX98BdxmkHtdoqMT4P"
	appChannelID string,
	// Replicated app channel slug
	// +default="dev"
	appChannelSlug string,
	// S3 bucket for artifact URLs
	// +default="dev-embedded-cluster-bin"
	s3Bucket string,
	// GitHub token
	// +optional
	githubToken *dagger.Secret,
) (*EmbeddedCluster, error) {
	if m.BuildMetadata == nil {
		return nil, fmt.Errorf("build metadata not initialized - call WithBuildMetadata first")
	}

	if m.BuildMetadata.BuildDir == nil {
		return nil, fmt.Errorf("build directory not present - call BuildBin first")
	}

	embedded, err := m.embedRelease(
		src,
		m.BuildMetadata.BuildDir,
		releaseYamlDir,
		replicatedApp,
		appChannelID,
		appChannelSlug,
		s3Bucket,
		s3Bucket != StagingS3Bucket,
		githubToken,
	)
	if err != nil {
		return nil, err
	}

	// Update metadata with binary file
	m.BuildMetadata.BuildDir = m.BuildMetadata.BuildDir.WithFile("bin/embedded-cluster", embedded)

	return m, nil
}

// embedRelease embeds the release into the binary (wraps ci-embed-release.sh)
func (m *EmbeddedCluster) embedRelease(
	src *dagger.Directory,
	buildDir *dagger.Directory,
	releaseYamlDir string,
	replicatedApp string,
	appChannelID string,
	appChannelSlug string,
	s3Bucket string,
	usesDevBucket bool,
	githubToken *dagger.Secret,
) (*dagger.File, error) {
	dir := directoryWithCommonFiles(dag.Directory(), src)

	// Build the embedded-cluster-release-builder binary
	// This is needed by the ci-embed-release.sh script
	releaseBuilder := goBuildContainer().
		WithDirectory("/workspace", dir).
		WithExec([]string{"sh", "-c", "CGO_ENABLED=0 go build -o /tmp/embedded-cluster-release-builder e2e/embedded-cluster-release-builder/main.go"}).
		File("/tmp/embedded-cluster-release-builder")

	// Use a container with the necessary tools
	container := ubuntuUtilsContainer().
		WithDirectory("/workspace", dir).
		WithDirectory("/workspace/output", buildDir)

	// Extract the binary from the tarball to output/bin/embedded-cluster
	// The script expects the binary at output/bin/embedded-cluster
	tarballPath := fmt.Sprintf("output/build/embedded-cluster-linux-%s.tgz", m.BuildMetadata.Arch)
	container = container.
		WithExec([]string{"mkdir", "-p", "output/bin"}).
		WithExec([]string{"tar", "-xzf", tarballPath, "-C", "output/bin"}).
		WithFile("/workspace/output/bin/embedded-cluster-release-builder", releaseBuilder)

	// Set environment variables
	container = m.BuildMetadata.withEnvVariables(container).
		WithEnvVariable("APP_CHANNEL_ID", appChannelID).
		WithEnvVariable("APP_CHANNEL_SLUG", appChannelSlug).
		WithEnvVariable("RELEASE_YAML_DIR", releaseYamlDir).
		WithEnvVariable("REPLICATED_APP", replicatedApp).
		WithEnvVariable("S3_BUCKET", s3Bucket)

	if usesDevBucket {
		container = container.WithEnvVariable("USES_DEV_BUCKET", "1")
	} else {
		container = container.WithEnvVariable("USES_DEV_BUCKET", "0")
	}

	if githubToken != nil {
		container = container.WithSecretVariable("GH_TOKEN", githubToken)
	}

	// Run the embed release script
	container = container.
		WithExec([]string{"./scripts/ci-embed-release.sh"})

	return container.File("output/bin/embedded-cluster"), nil
}
