package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// UploadBins uploads binaries and metadata to S3.
//
// This uploads:
// - metadata.json containing version information and artifact URLs
// - k0s binary (downloaded from GitHub or override URL)
// - kots binary (extracted from kotsadm image using crane)
// - operator binary (pre-extracted from operator image during build)
// - embedded-cluster tarball
//
// Requires AWS credentials via 1Password or explicit parameters.
//
// Example:
//
//	dagger call with-build-metadata with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  build-deps build-bin embed-release \
//	  upload-bins
func (m *EmbeddedCluster) UploadBins(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
	// S3 bucket for uploads
	// +default="dev-embedded-cluster-bin"
	s3Bucket string,
	// AWS access key ID (or from 1Password "EC Dev" item, field "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID")
	// +optional
	awsAccessKeyID *dagger.Secret,
	// AWS secret access key (or from 1Password "EC Dev" item, field "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY")
	// +optional
	awsSecretAccessKey *dagger.Secret,
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

	// Resolve AWS credentials
	awsAccessKeyID = m.mustResolveSecret(awsAccessKeyID, "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID")
	awsSecretAccessKey = m.mustResolveSecret(awsSecretAccessKey, "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY")

	// Upload binaries
	if err := m.uploadBinaries(
		ctx,
		src,
		s3Bucket,
		awsAccessKeyID,
		awsSecretAccessKey,
		githubToken,
	); err != nil {
		return nil, err
	}

	return m, nil
}

// uploadBinaries uploads binaries to S3 (wraps ci-upload-binaries.sh)
func (m *EmbeddedCluster) uploadBinaries(
	ctx context.Context,
	src *dagger.Directory,
	s3Bucket string,
	awsAccessKey *dagger.Secret,
	awsSecretKey *dagger.Secret,
	githubToken *dagger.Secret,
) error {
	dir := directoryWithCommonFiles(dag.Directory(), src)

	container := ubuntuUtilsContainer(dagger.ContainerOpts{Platform: "linux/amd64"})

	container = m.BuildMetadata.withEnvVariables(container).
		WithDirectory("/workspace", dir).
		// Mount the build directory which contains output/build/ with metadata and tarballs (including pre-extracted operator binary)
		WithDirectory("/workspace/output", m.BuildMetadata.BuildDir).
		// Create symlink so script can find build/metadata.json at the expected path
		WithExec([]string{"sh", "-c", "ln -sf output/build build"}).
		WithEnvVariable("S3_BUCKET", s3Bucket).
		// Enable all binary uploads (k0s, kots, operator, embedded-cluster tarball)
		WithEnvVariable("UPLOAD_BINARIES", "1").
		WithSecretVariable("AWS_ACCESS_KEY_ID", awsAccessKey).
		WithSecretVariable("AWS_SECRET_ACCESS_KEY", awsSecretKey)

	if githubToken != nil {
		container = container.WithSecretVariable("GH_TOKEN", githubToken)
	}

	// Run upload script (will upload all binaries with UPLOAD_BINARIES=1)
	_, err := container.
		WithExec([]string{"./scripts/ci-upload-binaries.sh"}).
		Sync(ctx)

	return err
}
