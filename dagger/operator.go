package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/embedded-cluster/internal/dagger"
)

// Builds the operator image with APKO.
func (m *EmbeddedCluster) BuildOperatorImage(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Repository to use for the image.
	// +default="replicated/embedded-cluster-operator-image"
	repo string,
	// Version to use for the package.
	ecVersion string,
	// K0s minor version to build for.
	// +default=""
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (*dagger.File, error) {

	tag := strings.ReplaceAll(ecVersion, "+", "-")
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile, err := localImageConfig(ctx, src, "embedded-cluster-operator", kzerosMinorVersion)
	if err != nil {
		return nil, err
	}

	pkgBuild, err := m.BuildOperatorPackage(ctx, src, ecVersion, kzerosMinorVersion, arch)
	if err != nil {
		return nil, err
	}

	dir := dag.Directory().
		WithFile("melange.rsa.pub", pkgBuild.File("melange.rsa.pub")).
		WithDirectory("packages", pkgBuild.Directory("packages"))

	build := m.chainguard.apkoBuild(
		dir,
		apkoFile,
		image,
		arch,
		APKOImageVersion,
	)

	return build.File("apko.tar"), nil
}

// Builds and publishes the operator image with APKO.
func (m *EmbeddedCluster) PublishOperatorImage(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Repository to use for the image.
	// +default="replicated/embedded-cluster-operator-image"
	repo string,
	// Version to use for the package.
	ecVersion string,
	// K0s minor version to build for.
	// +default=""
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (string, error) {

	tag := strings.ReplaceAll(ecVersion, "+", "-")
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile, err := localImageConfig(ctx, src, "embedded-cluster-operator", kzerosMinorVersion)
	if err != nil {
		return "", err
	}

	pkgBuild, err := m.BuildOperatorPackage(ctx, src, ecVersion, kzerosMinorVersion, arch)
	if err != nil {
		return "", err
	}

	dir := dag.Directory().
		WithFile("melange.rsa.pub", pkgBuild.File("melange.rsa.pub")).
		WithDirectory("packages", pkgBuild.Directory("packages"))

	if m.RegistryAuth != nil {
		dir = dir.WithDirectory(".docker", m.RegistryAuth)
	}

	publish := m.chainguard.apkoPublish(
		dir,
		apkoFile,
		image,
		arch,
		APKOImageVersion,
	)

	return publish.Stdout(ctx)
}

// Builds the operator package with Melange.
func (m *EmbeddedCluster) BuildOperatorPackage(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Version to use for the package.
	ecVersion string,
	// K0s minor version to build for.
	// +default=""
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (*dagger.Directory, error) {

	melangeFile, err := localPackageConfig(ctx, src, "embedded-cluster-operator", kzerosMinorVersion)
	if err != nil {
		return nil, err
	}

	build := m.chainguard.melangeBuildGo(
		src,
		melangeFile,
		ecVersion,
		arch,
		MelangeImageVersion,
	)

	return build.Directory("build"), nil
}
