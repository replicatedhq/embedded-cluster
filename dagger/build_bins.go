package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
	"strings"
)

// BuildBin builds the web UI and binary (without embedding the release).
//
// This is equivalent to ci-build-bin.sh and builds:
// - Web UI (React/TypeScript frontend)
// - Go binary with embedded web assets
// - Metadata JSON
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-build-metadata --dir ./output \
//	  build-bin \
//	  build-metadata to-dir export --path=./output
func (m *EmbeddedCluster) BuildBin(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
	// S3 bucket for artifact URLs
	// +default="dev-embedded-cluster-bin"
	s3Bucket string,
) (*EmbeddedCluster, error) {
	if m.BuildMetadata == nil {
		return nil, fmt.Errorf("build metadata is required")
	}
	if m.BuildMetadata.Version == "" {
		return nil, fmt.Errorf("version is required")
	}
	if m.BuildMetadata.K0SMinorVersion == "" {
		return nil, fmt.Errorf("k0s minor version is required")
	}
	if m.BuildMetadata.Arch == "" {
		return nil, fmt.Errorf("arch is required")
	}

	// Build web UI
	webBuild := m.BuildWeb(ctx, src)

	// Build binary
	buildResult, err := m.buildBinary(
		ctx,
		src,
		webBuild,
		s3Bucket,
		s3Bucket != StagingS3Bucket,
	)
	if err != nil {
		return nil, err
	}

	// Update metadata with build artifacts
	m.BuildMetadata.BuildDir = buildResult

	return m, nil
}

// buildBinary builds the embedded-cluster binary (ports ci-build-bin.sh logic)
func (m *EmbeddedCluster) buildBinary(
	ctx context.Context,
	src *dagger.Directory,
	webBuild *dagger.Directory,
	s3Bucket string,
	usesDevBucket bool,
) (*dagger.Directory, error) {
	// Use cached build environment
	builder := m.buildEnv(src)

	// Set build environment variables
	builder = m.BuildMetadata.withEnvVariables(builder).
		WithEnvVariable("IMAGES_REGISTRY_SERVER", "ttl.sh")

	// Run make build-deps (generate CRDs and dependencies)
	builder = builder.
		WithExec([]string{"make", "build-deps"})

	// Build buildtools binary (needed for updating addon metadata)
	buildtools := m.buildBuildtools(ctx, builder)
	builder = builder.
		WithFile("/workspace/output/bin/buildtools", buildtools)

	// Update operator metadata with the built operator image
	// buildtools expects INPUT_OPERATOR_IMAGE without the tag (it will extract it from the chart)
	operatorImageWithoutTag := m.BuildMetadata.OperatorImageRepo

	// The chart version matches the operator image tag (without 'v' prefix)
	// This should match what was published in PublishOperatorChart
	chartVersionForOCI := strings.TrimPrefix(m.BuildMetadata.OperatorImageTag, "v")

	builder = builder.
		WithEnvVariable("INPUT_OPERATOR_CHART_URL", m.BuildMetadata.OperatorChartURL).
		WithEnvVariable("INPUT_OPERATOR_CHART_VERSION", chartVersionForOCI).
		WithEnvVariable("INPUT_OPERATOR_IMAGE", operatorImageWithoutTag).
		WithExec([]string{"./output/bin/buildtools", "update", "addon", "embeddedclusteroperator"})

	// Construct metadata URLs
	var k0sBinaryURL, kotsBinaryURL, operatorBinaryURL string
	if usesDevBucket {
		k0sBinaryURL = fmt.Sprintf("https://%s.s3.amazonaws.com/k0s-binaries/%s-%s", s3Bucket, m.BuildMetadata.K0SVersion, m.BuildMetadata.Arch)

		// Get KOTS version
		kotsVersion, err := builder.
			WithExec([]string{"make", "print-KOTS_VERSION"}).
			Stdout(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get KOTS_VERSION: %w", err)
		}
		kotsVersion = strings.TrimSpace(kotsVersion)
		kotsBinaryURL = fmt.Sprintf("https://%s.s3.amazonaws.com/kots-binaries/%s-%s.tar.gz", s3Bucket, kotsVersion, m.BuildMetadata.Arch)

		operatorVersion := strings.TrimPrefix(m.BuildMetadata.Version, "v")
		operatorBinaryURL = fmt.Sprintf("https://%s.s3.amazonaws.com/operator-binaries/%s-%s.tar.gz", s3Bucket, operatorVersion, m.BuildMetadata.Arch)
	}

	// Build fio binary using Dagger (avoids Docker-in-Docker)
	fioVersion := "3.41"
	fioBinary := m.BuildFio(fioVersion, m.BuildMetadata.Arch)
	builder = builder.
		WithFile(fmt.Sprintf("/workspace/output/bins/fio-%s-%s", fioVersion, m.BuildMetadata.Arch), fioBinary)

	// Build the embedded-cluster binary
	localArtifactMirrorImage := fmt.Sprintf("proxy.replicated.com/anonymous/%s", m.BuildMetadata.LAMImageRepo)

	// Mount web build directory
	builder = builder.WithDirectory("/workspace/web", webBuild)

	builder = builder.
		WithEnvVariable("METADATA_K0S_BINARY_URL_OVERRIDE", k0sBinaryURL).
		WithEnvVariable("METADATA_KOTS_BINARY_URL_OVERRIDE", kotsBinaryURL).
		WithEnvVariable("METADATA_OPERATOR_BINARY_URL_OVERRIDE", operatorBinaryURL).
		WithEnvVariable("LOCAL_ARTIFACT_MIRROR_IMAGE", localArtifactMirrorImage).
		WithExec([]string{"make", fmt.Sprintf("embedded-cluster-linux-%s", m.BuildMetadata.Arch)})

	// Copy binary to preserve original
	builder = builder.
		WithExec([]string{"cp", "output/bin/embedded-cluster", "output/bin/embedded-cluster-original"})

	// Create tarball
	builder = builder.
		WithExec([]string{"mkdir", "-p", "output/build"}).
		WithExec([]string{"tar", "-C", "output/bin", "-czvf", fmt.Sprintf("output/build/embedded-cluster-linux-%s.tgz", m.BuildMetadata.Arch), "embedded-cluster"})

	// Generate metadata
	builder = builder.
		WithExec([]string{"sh", "-c", "./output/bin/embedded-cluster version metadata > output/build/metadata.json"})

	// Extract operator binary from the operator image (for upload to S3)
	// This avoids needing Docker in the upload step
	operatorImageFullName := fmt.Sprintf("%s:%s", m.BuildMetadata.OperatorImageRepo, m.BuildMetadata.OperatorImageTag)
	operatorVersion := strings.TrimPrefix(m.BuildMetadata.Version, "v")
	builder = builder.
		WithExec([]string{"mkdir", "-p", "output/operator-bin"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("crane export %s --platform linux/%s - | tar -xf - -C output/operator-bin manager", operatorImageFullName, m.BuildMetadata.Arch)}).
		WithExec([]string{"mv", "output/operator-bin/manager", "output/operator-bin/operator"}).
		WithExec([]string{"tar", "-C", "output/operator-bin", "-czvf", fmt.Sprintf("output/build/%s.tar.gz", operatorVersion), "operator"})

	return builder.Directory("/workspace/output"), nil
}

// BuildWeb builds the web UI using official Node.js image
func (m *EmbeddedCluster) BuildWeb(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.Directory {
	// Create cache volume for npm to avoid re-downloading packages
	npmCache := dag.CacheVolume("ec-npm-cache")

	dir := directoryWithCommonFiles(dag.Directory(), src)

	// The web build needs api/docs as a sibling directory (../api/docs)
	// Create a directory structure with both web and api/docs
	return nodeBuildContainer().
		WithMountedCache("/root/.npm", npmCache).
		WithDirectory("/workspace", dir).
		WithWorkdir("/workspace/web").
		// Install dependencies (cached via npm cache)
		WithExec([]string{"npm", "ci"}).
		// Build production bundle
		WithExec([]string{"npm", "run", "build"}).
		// Return the built web directory
		Directory("/workspace/web")
}

// BuildBuildtools builds the buildtools binary (needed for updating addon metadata)
func (m *EmbeddedCluster) BuildBuildtools(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.File {
	builder := m.buildEnv(src)
	return m.buildBuildtools(ctx, builder)
}

func (m *EmbeddedCluster) buildBuildtools(ctx context.Context, builder *dagger.Container) *dagger.File {
	return builder.
		WithEnvVariable("CGO_ENABLED", "0").
		WithExec([]string{"mkdir", "-p", "output/bin"}).
		WithExec([]string{"go", "build", "-tags", GoBuildTags, "-o", "output/bin/buildtools", "./cmd/buildtools"}).
		File("output/bin/buildtools")
}

// BuildFio builds the fio binary (replicates fio/Dockerfile logic)
//
// This can be called standalone or used internally by the build process.
//
// Example:
//
//	dagger call build-fio --version=3.41 --arch=amd64 export --path=./fio
func (m *EmbeddedCluster) BuildFio(
	// FIO version to build
	version string,
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
		WithExec([]string{"curl", "-fsSL", "-o", "fio.tar.gz", fmt.Sprintf("https://api.github.com/repos/axboe/fio/tarball/fio-%s", version)}).
		WithExec([]string{"tar", "-xzf", "fio.tar.gz", "--strip-components=1"}).
		WithExec([]string{"./configure", "--build-static", "--disable-native"}).
		WithExec([]string{"sh", "-c", "make -j$(nproc)"})

	// Extract the binary
	return buildContainer.File("/fio/fio")
}

// PublishOperatorChart packages and publishes the operator Helm chart using the Helm Dagger module
func (m *EmbeddedCluster) PublishOperatorChart(
	ctx context.Context,
	src *dagger.Directory,
	ecVersion string,
	imageName string,
	chartRemote string,
) error {
	dir := directoryWithCommonFiles(dag.Directory(), src)

	// The chart version should not have the 'v' prefix
	imageTag := strings.ReplaceAll(ecVersion, "+", "-")
	chartVersion := strings.TrimPrefix(ecVersion, "v")

	// Template the Chart.yaml and values.yaml files using envsubst
	// We need to create a container to do the templating, then export the directory
	templatedChart := ubuntuUtilsContainer().
		WithDirectory("/workspace", dir).
		WithWorkdir("/workspace/operator/charts/embedded-cluster-operator").
		// Template Chart.yaml.tmpl -> Chart.yaml
		WithEnvVariable("CHART_VERSION", chartVersion).
		WithExec([]string{"sh", "-c", "envsubst < Chart.yaml.tmpl > Chart.yaml"}).
		// Template values.yaml.tmpl -> values.yaml
		WithEnvVariable("IMAGE_NAME", imageName).
		WithEnvVariable("IMAGE_TAG", imageTag).
		WithExec([]string{"sh", "-c", "envsubst < values.yaml.tmpl > values.yaml"}).
		// Export the templated chart directory
		Directory("/workspace/operator/charts/embedded-cluster-operator")

	// Package the chart using the Helm Dagger module
	chartPackage := dag.Helm().Package(templatedChart)

	// Push the chart to the OCI registry
	err := dag.Helm().Push(ctx, chartPackage, chartRemote)
	if err != nil {
		return fmt.Errorf("push chart: %w", err)
	}

	return nil
}

// buildEnv creates a Go build environment with caching for faster builds.
// This caches both Go module downloads (GOMODCACHE) and build artifacts (GOCACHE).
func (m *EmbeddedCluster) buildEnv(
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.Container {
	dir := directoryWithCommonFiles(dag.Directory(), src)

	// Create cache volumes for Go modules and build cache
	goModCache := dag.CacheVolume("ec-gomodcache")
	goBuildCache := dag.CacheVolume("ec-gobuildcache")

	return goBuildContainer().
		// Install additional tools needed for the build
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "make", "git", "jq", "tar", "gzip", "curl"}).
		// Install crane (needed to extract kots binary from kotsadm image)
		WithExec([]string{"sh", "-c", "curl -fsSL https://github.com/google/go-containerregistry/releases/latest/download/go-containerregistry_Linux_x86_64.tar.gz | tar -xzf - -C /usr/local/bin crane"}).
		// Mount source code
		WithDirectory("/workspace", dir).
		// Configure Go caching
		WithMountedCache("/go/pkg/mod", goModCache).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", goBuildCache).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		// Pre-download modules to populate cache
		WithExec([]string{"go", "mod", "download"})
}
