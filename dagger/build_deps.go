package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
	"strings"
)

// BuildDeps builds and publishes operator and LAM images and charts.
//
// This is equivalent to ci-build-deps.sh and builds:
// - Operator image (using APKO/Melange)
// - Operator Helm chart (published to OCI registry)
// - Local artifact mirror image
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-build-metadata \
//	  build-deps build-metadata to-dir export --path ./output
func (m *EmbeddedCluster) BuildDeps(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// TTL.sh user prefix for image publishing
	// +default="ec-build"
	ttlShUser string,
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

	// Build and publish operator image using APKO/Melange
	m.BuildMetadata.OperatorImageRepo = "ttl.sh/" + ttlShUser + "/embedded-cluster-operator-image"
	_, err := m.PublishOperatorImage(ctx, src, m.BuildMetadata.OperatorImageRepo, m.BuildMetadata.Version, m.BuildMetadata.K0SMinorVersion, m.BuildMetadata.Arch)
	if err != nil {
		return nil, fmt.Errorf("publish operator image: %w", err)
	}

	// Publish operator Helm chart
	chartRemote := "oci://ttl.sh/" + ttlShUser
	err = m.PublishOperatorChart(ctx, src, m.BuildMetadata.Version, m.BuildMetadata.OperatorImageRepo, chartRemote)
	if err != nil {
		return nil, fmt.Errorf("publish operator chart: %w", err)
	}

	// Build and publish local-artifact-mirror image
	m.BuildMetadata.LAMImageRepo = "ttl.sh/" + ttlShUser + "/embedded-cluster-local-artifact-mirror"
	_, err = m.PublishLocalArtifactMirrorImage(ctx, src, m.BuildMetadata.LAMImageRepo, m.BuildMetadata.Version, m.BuildMetadata.K0SMinorVersion, m.BuildMetadata.Arch)
	if err != nil {
		return nil, fmt.Errorf("publish local-artifact-mirror image: %w", err)
	}

	// Get the image tags
	tag := strings.ReplaceAll(m.BuildMetadata.Version, "+", "-")
	m.BuildMetadata.OperatorImageTag = tag
	m.BuildMetadata.LAMImageTag = tag

	// Construct the chart URL (chart name is "embedded-cluster-operator" from Chart.yaml)
	m.BuildMetadata.OperatorChartURL = chartRemote + "/embedded-cluster-operator"

	return m, nil
}
