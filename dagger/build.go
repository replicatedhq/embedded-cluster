package main

import (
	"context"
	"fmt"

	"dagger/embedded-cluster/internal/dagger"
)

// BuildArtifacts represents the outputs of a build process.
// This is used as a composable data structure throughout the build pipeline.
type BuildArtifacts struct {
	// EC version (e.g., "v1.0.0")
	Version string
	// App version (e.g., "appver-dev-abc123")
	AppVersion string
	// Build directory containing all artifacts
	BuildDir *dagger.Directory
}

// BuildAndRelease builds embedded-cluster artifacts and creates a release.
//
// This is a Dagger-native implementation that avoids Docker-in-Docker by:
// - Using existing Dagger functions for operator/LAM image builds (APKO/Melange)
// - Building the binary with native Go compilation
// - Wrapping only the upload and release scripts
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  build-and-release --ec-version="v1.0.0" --app-version="appver-dev-abc123"
func (m *EmbeddedCluster) BuildAndRelease(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Version for the embedded-cluster binary (e.g., "v1.0.0" or auto-detected from git)
	// +optional
	ecVersion string,
	// K0s minor version (e.g., "1.29" or auto-detected from make)
	// +optional
	k0SMinorVersion string,
	// App version label for the Replicated release (e.g., "appver-dev-abc123" or auto-detected from git)
	// +optional
	appVersion string,
	// Release YAML directory containing Kubernetes manifests
	// +default="e2e/kots-release-install-v3"
	releaseYamlDir string,
	// Replicated app name
	// +default="embedded-cluster-smoke-test-staging-app"
	replicatedApp string,
	// Replicated app ID
	// +default="2bViecGO8EZpChcGPeW5jbWKw2B"
	appID string,
	// Replicated app channel name
	// +default="Dev"
	appChannel string,
	// Replicated app channel ID
	// +default="2lhrq5LDyoX98BdxmkHtdoqMT4P"
	appChannelID string,
	// Replicated app channel slug
	// +default="dev"
	appChannelSlug string,
	// S3 bucket for artifacts
	// +default="dev-embedded-cluster-bin"
	s3Bucket string,
	// Whether to upload binaries to S3
	// +default=true
	uploadBinaries bool,
	// Whether to skip creating the Replicated app release
	// +default=false
	skipRelease bool,
	// Architecture to build for
	// +default="amd64"
	arch string,
	// TTL.sh user prefix for image publishing
	// +default="ec-build"
	ttlShUser string,
	// AWS access key ID (or from 1Password "EC Dev" item, field "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID")
	// +optional
	awsAccessKeyID *dagger.Secret,
	// AWS secret access key (or from 1Password "EC Dev" item, field "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY")
	// +optional
	awsSecretAccessKey *dagger.Secret,
	// Replicated API token (or from 1Password "EC Dev" item, field "STAGING_REPLICATED_API_TOKEN")
	// +optional
	replicatedAPIToken *dagger.Secret,
	// GitHub token
	// +optional
	githubToken *dagger.Secret,
) (*BuildArtifacts, error) {
	// Validate secrets needed for the build
	m.buildValidateSecrets(awsAccessKeyID, awsSecretAccessKey, replicatedAPIToken, skipRelease)

	// Step 1: Build metadata using composable function
	_, err := m.WithBuildMetadata(ctx, src, nil, ecVersion, appVersion, k0SMinorVersion, arch)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize build metadata: %w", err)
	}

	// Step 2: Build dependencies using composable function
	_, err = m.BuildDeps(ctx, src, ttlShUser)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

	// Step 3: Build binary using composable function
	m, err = m.BuildBin(
		ctx,
		src,
		s3Bucket,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build binary: %w", err)
	}

	// Step 4: Embed release using composable function
	m, err = m.EmbedRelease(
		ctx,
		src,
		releaseYamlDir,
		replicatedApp,
		appID,
		appChannelID,
		appChannelSlug,
		s3Bucket,
		replicatedAPIToken,
		githubToken,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to embed release: %w", err)
	}

	// Step 5: Upload binaries (if enabled)
	if uploadBinaries {
		m, err = m.UploadBins(
			ctx,
			src,
			s3Bucket,
			awsAccessKeyID,
			awsSecretAccessKey,
			githubToken,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to upload binaries: %w", err)
		}
	}

	// Step 6: Create Replicated release (if not skipped)
	if !skipRelease {
		m, err = m.ReleaseApp(
			ctx,
			src,
			releaseYamlDir,
			replicatedApp,
			appChannel,
			s3Bucket,
			replicatedAPIToken,
			githubToken,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create release: %w", err)
		}
	}

	buildDir, err := m.BuildMetadata.ToDir()
	if err != nil {
		return nil, fmt.Errorf("failed to create build directory: %w", err)
	}

	return &BuildArtifacts{
		Version:    m.BuildMetadata.Version,
		AppVersion: m.BuildMetadata.AppVersion,
		BuildDir:   buildDir,
	}, nil
}

// buildValidateSecrets validates the secrets for the build.
func (m *EmbeddedCluster) buildValidateSecrets(awsAccessKeyID *dagger.Secret, awsSecretAccessKey *dagger.Secret, replicatedAPIToken *dagger.Secret, skipRelease bool) {
	_ = m.mustResolveSecret(awsAccessKeyID, "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID")
	_ = m.mustResolveSecret(awsSecretAccessKey, "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY")
	if !skipRelease {
		_ = m.mustResolveSecret(replicatedAPIToken, "STAGING_REPLICATED_API_TOKEN")
	}
}
