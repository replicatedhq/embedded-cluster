package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// ReleaseApp creates a Replicated app release.
//
// This is equivalent to ci-release-app.sh and creates a new release
// in the specified Replicated app and channel.
//
// Requires Replicated API token via 1Password or explicit parameter.
//
// Example:
//
//	dagger call with-build-metadata with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  build-deps build-bin embed-release upload-bins \
//	  release-app
func (m *EmbeddedCluster) ReleaseApp(
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
	// Replicated API origin
	// +default="https://api.staging.replicated.com/vendor"
	replicatedAPIOrigin string,
	// S3 bucket for artifact URLs
	// +default="dev-embedded-cluster-bin"
	s3Bucket string,
	// Whether using dev bucket
	// +default=true
	usesDevBucket bool,
	// Replicated API token (or from 1Password "EC CI" item, field "STAGING_REPLICATED_API_TOKEN")
	// +optional
	replicatedAPIToken *dagger.Secret,
) (*EmbeddedCluster, error) {
	if m.BuildMetadata == nil {
		return nil, fmt.Errorf("build metadata not initialized - call WithBuildMetadata first")
	}

	// Resolve Replicated API token
	replicatedAPIToken = m.mustResolveSecret(replicatedAPIToken, "STAGING_REPLICATED_API_TOKEN")

	// Create release
	err := m.createRelease(
		ctx,
		src,
		m.BuildMetadata.Version,
		m.BuildMetadata.AppVersion,
		releaseYamlDir,
		replicatedApp,
		replicatedAPIOrigin,
		s3Bucket,
		usesDevBucket,
		replicatedAPIToken,
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// createRelease creates a Replicated app release (wraps ci-release-app.sh)
func (m *EmbeddedCluster) createRelease(
	ctx context.Context,
	src *dagger.Directory,
	ecVersion string,
	appVersion string,
	releaseYamlDir string,
	replicatedApp string,
	replicatedAPIOrigin string,
	s3Bucket string,
	usesDevBucket bool,
	replicatedToken *dagger.Secret,
) error {
	container := ubuntuUtilsContainer().
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithEnvVariable("EC_VERSION", ecVersion).
		WithEnvVariable("APP_VERSION", appVersion).
		WithEnvVariable("RELEASE_YAML_DIR", releaseYamlDir).
		WithEnvVariable("REPLICATED_APP", replicatedApp).
		WithEnvVariable("REPLICATED_API_ORIGIN", replicatedAPIOrigin).
		WithEnvVariable("S3_BUCKET", s3Bucket).
		WithSecretVariable("REPLICATED_API_TOKEN", replicatedToken)

	if usesDevBucket {
		container = container.WithEnvVariable("USES_DEV_BUCKET", "1")
	} else {
		container = container.WithEnvVariable("USES_DEV_BUCKET", "0")
	}

	// Install Replicated CLI
	container = container.
		WithExec([]string{"sh", "-c", "curl -fsSL https://raw.githubusercontent.com/replicatedhq/replicated/main/install.sh | bash"})

	// Install Helm
	container = container.
		WithExec([]string{"sh", "-c", "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"})

	// Run release script
	_, err := container.
		WithExec([]string{"./scripts/ci-release-app.sh"}).
		Sync(ctx)

	return err
}
