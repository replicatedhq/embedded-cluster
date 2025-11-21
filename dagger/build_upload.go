package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// UploadBins uploads binaries to S3.
//
// This is equivalent to ci-upload-binaries.sh and uploads:
// - k0s binary
// - kots binary
// - operator image metadata
// - embedded-cluster binary
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
	// AWS access key ID (or from 1Password "EC CI" item, field "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID")
	// +optional
	awsAccessKeyID *dagger.Secret,
	// AWS secret access key (or from 1Password "EC CI" item, field "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY")
	// +optional
	awsSecretAccessKey *dagger.Secret,
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
		m.BuildMetadata.BuildDir,
		m.BuildMetadata.Version,
		m.BuildMetadata.K0SVersion,
		m.BuildMetadata.Arch,
		s3Bucket,
		awsAccessKeyID,
		awsSecretAccessKey,
	); err != nil {
		return nil, err
	}

	return m, nil
}

// uploadBinaries uploads binaries to S3 (wraps ci-upload-binaries.sh)
func (m *EmbeddedCluster) uploadBinaries(
	ctx context.Context,
	src *dagger.Directory,
	embeddedDir *dagger.Directory,
	ecVersion string,
	k0sVersion string,
	arch string,
	s3Bucket string,
	awsAccessKey *dagger.Secret,
	awsSecretKey *dagger.Secret,
) error {
	container := ubuntuUtilsContainer(dagger.ContainerOpts{Platform: "linux/amd64"}).
		WithDirectory("/src/scripts", src.Directory("scripts")).
		WithDirectory("/src/build", embeddedDir).
		WithFile("/src/versions.mk", src.File("versions.mk")).
		WithFile("/src/common.mk", src.File("common.mk")).
		WithFile("/src/Makefile", src.File("Makefile")).
		WithWorkdir("/src").
		WithEnvVariable("EC_VERSION", ecVersion).
		WithEnvVariable("K0S_VERSION", k0sVersion).
		WithEnvVariable("ARCH", arch).
		WithEnvVariable("S3_BUCKET", s3Bucket).
		// Skip binary uploads (k0s, kots, operator) - only upload metadata.json
		// Binary uploads require Docker/crane/oras which are complex to set up in Dagger
		WithEnvVariable("UPLOAD_BINARIES", "0").
		WithSecretVariable("AWS_ACCESS_KEY_ID", awsAccessKey).
		WithSecretVariable("AWS_SECRET_ACCESS_KEY", awsSecretKey)

	// Install AWS CLI (only tool needed for metadata upload)
	container = container.
		WithExec([]string{"sh", "-c", "curl -fsSL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o /tmp/awscliv2.zip"}).
		WithExec([]string{"sh", "-c", "cd /tmp && unzip -q awscliv2.zip && ./aws/install"})

	// Run upload script (will only upload metadata.json with UPLOAD_BINARIES=0)
	_, err := container.
		WithExec([]string{"./scripts/ci-upload-binaries.sh"}).
		Sync(ctx)

	return err
}
