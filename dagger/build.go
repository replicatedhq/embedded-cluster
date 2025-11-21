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
	// Binary file
	Binary *dagger.File
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
	k0sMinorVersion string,
	// App version label for the Replicated release (e.g., "appver-dev-abc123" or auto-detected from git)
	// +optional
	appVersion string,
	// Release YAML directory containing Kubernetes manifests
	// +default="e2e/kots-release-install-v3"
	releaseYamlDir string,
	// Replicated app name
	// +default="embedded-cluster-smoke-test-staging-app"
	replicatedApp string,
	// Replicated API origin
	// +default="https://api.staging.replicated.com/vendor"
	replicatedAPIOrigin string,
	// S3 bucket for artifacts
	// +default="dev-embedded-cluster-bin"
	s3Bucket string,
	// Whether to upload binaries to S3
	// +default=true
	uploadBinaries bool,
	// Whether to skip creating the Replicated app release
	// +default=false
	skipRelease bool,
	// Whether to use dev bucket URLs in metadata
	// +default=true
	usesDevBucket bool,
	// Architecture to build for
	// +default="amd64"
	arch string,
	// Whether to use Chainguard images
	// +default=false
	useChainguard bool,
	// AWS credentials (overrides 1Password if provided)
	// +optional
	awsAccessKeyID *dagger.Secret,
	// AWS secret access key (overrides 1Password if provided)
	// +optional
	awsSecretAccessKey *dagger.Secret,
	// Replicated API token (overrides 1Password if provided)
	// +optional
	replicatedAPIToken *dagger.Secret,
) (*BuildArtifacts, error) {
	// Validate secrets
	m.buildValidateSecrets()

	// Step 1: Build metadata using composable function
	_, err := m.WithBuildMetadata(ctx, src, nil, ecVersion, appVersion, k0sMinorVersion, arch)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize build metadata: %w", err)
	}

	// Step 2: Build dependencies using composable function
	_, err = m.BuildDeps(ctx, src, "ec-build")
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
		s3Bucket,
		usesDevBucket,
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
			replicatedAPIOrigin,
			s3Bucket,
			usesDevBucket,
			replicatedAPIToken,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create release: %w", err)
		}
	}

	return &BuildArtifacts{
		Version:    m.BuildMetadata.Version,
		AppVersion: m.BuildMetadata.AppVersion,
		BuildDir:   m.BuildMetadata.BuildDir,
		Binary:     m.BuildMetadata.Binary,
	}, nil
}

// buildValidateSecrets validates the secrets for the build.
func (m *EmbeddedCluster) buildValidateSecrets() {
	if m.OnePassword == nil {
		panic(fmt.Errorf("one password not initialized - call WithOnePassword first"))
	}

	_ = m.mustResolveSecret(nil, "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID")
	_ = m.mustResolveSecret(nil, "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY")
	_ = m.mustResolveSecret(nil, "STAGING_REPLICATED_API_TOKEN")
}
