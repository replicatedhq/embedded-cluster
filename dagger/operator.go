package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/embedded-cluster/internal/dagger"
)

// Builds the operator image with APKO.
func (m *EmbeddedCluster) BuildOperatorImage(
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Repository to use for the image.
	// +default="replicated/embedded-cluster-operator-image"
	repo string,
	// Version to use for the package.
	ecVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.File {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateOprator(src, ecVersion)

	pkgBuild := m.BuildOperatorPackage(src, ecVersion, arch)

	dir := dag.Directory().
		WithFile("melange.rsa.pub", pkgBuild.File("melange.rsa.pub")).
		WithDirectory("packages", pkgBuild.Directory("packages"))

	build := m.apkoBuild(
		dir,
		apkoFile,
		image,
		arch,
		APKOImageVersion,
	)

	return build.File("apko.tar")
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
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (string, error) {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateOprator(src, ecVersion)

	pkgBuild := m.BuildOperatorPackage(src, ecVersion, arch)

	dir := dag.Directory().
		WithFile("melange.rsa.pub", pkgBuild.File("melange.rsa.pub")).
		WithDirectory("packages", pkgBuild.Directory("packages"))

	build := m.apkoPublish(
		dir,
		apkoFile,
		image,
		arch,
		APKOImageVersion,
	)

	return build.Stdout(ctx)
}

// Builds the operator package with Melange.
func (m *EmbeddedCluster) BuildOperatorPackage(
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Version to use for the package.
	ecVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.Directory {

	melangeFile := m.melangeTemplateOperator(src, ecVersion)

	dir := dag.Directory().
		WithDirectory("operator", src.Directory("operator"))

	build := m.melangeBuildGo(
		directoryWithCommonGoFiles(dir, src),
		melangeFile,
		arch,
		MelangeImageVersion,
	)

	return build.Directory("build")
}

func (m *EmbeddedCluster) apkoTemplateOprator(
	src *dagger.Directory,
	ecVersion string,
) *dagger.File {
	return m.renderTemplate(
		src.Directory("operator/deploy"),
		map[string]string{
			"PACKAGE_VERSION": ecVersion,
		},
		"apko.tmpl.yaml",
		"apko.yaml",
	)
}

func (m *EmbeddedCluster) melangeTemplateOperator(
	src *dagger.Directory,
	ecVersion string,
) *dagger.File {
	return m.renderTemplate(
		src.Directory("operator/deploy"),
		map[string]string{
			"PACKAGE_VERSION": ecVersion,
		},
		"melange.tmpl.yaml",
		"melange.yaml",
	)
}
