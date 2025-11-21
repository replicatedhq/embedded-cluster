package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// UploadBins uploads metadata to S3.
//
// This uploads the metadata.json file containing version information and artifact URLs.
// Binary uploads (k0s, kots, operator) are skipped due to Docker/crane/oras complexity in Dagger.
// For full binary uploads, run ci-upload-binaries.sh directly with UPLOAD_BINARIES=1.
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
		WithEnvVariable("S3_BUCKET", s3Bucket).
		// Skip binary uploads (k0s, kots, operator) - only upload metadata.json
		// Binary uploads require Docker/crane/oras which are complex to set up in Dagger
		WithEnvVariable("UPLOAD_BINARIES", "0").
		WithSecretVariable("AWS_ACCESS_KEY_ID", awsAccessKey).
		WithSecretVariable("AWS_SECRET_ACCESS_KEY", awsSecretKey)

	if githubToken != nil {
		container = container.WithSecretVariable("GH_TOKEN", githubToken)
	}

	// Run upload script (will only upload metadata.json with UPLOAD_BINARIES=0)
	_, err := container.
		WithExec([]string{"./scripts/ci-upload-binaries.sh"}).
		Sync(ctx)

	return err
}
