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
	// K0s minor version to build for.
	// +default=""
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.File {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateOprator(src, ecVersion, kzerosMinorVersion)

	pkgBuild := m.BuildOperatorPackage(src, ecVersion, kzerosMinorVersion, arch)

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
	// K0s minor version to build for.
	// +default=""
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (string, error) {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateOprator(src, ecVersion, kzerosMinorVersion)

	pkgBuild := m.BuildOperatorPackage(src, ecVersion, kzerosMinorVersion, arch)

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
) *dagger.Directory {

	melangeFile := m.melangeTemplateOperator(src, ecVersion, kzerosMinorVersion)

	dir := dag.Directory().
		WithDirectory("operator", src.Directory("operator"))

	build := m.chainguard.melangeBuildGo(
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
	k0sMinorVersion string,
) *dagger.File {
	vars := map[string]string{
		"PACKAGE_VERSION": ecVersion,
	}
	if k0sMinorVersion != "" {
		vars["K0S_MINOR_VERSION"] = k0sMinorVersion
	}
	return m.common.renderTemplate(
		src.Directory("operator/deploy"),
		vars,
		"apko.tmpl.yaml",
		"apko.yaml",
	)
}

func (m *EmbeddedCluster) melangeTemplateOperator(
	src *dagger.Directory,
	ecVersion string,
	k0sMinorVersion string,
) *dagger.File {
	vars := map[string]string{
		"PACKAGE_VERSION": ecVersion,
	}
	if k0sMinorVersion != "" {
		vars["K0S_MINOR_VERSION"] = k0sMinorVersion
	}
	return m.common.renderTemplate(
		src.Directory("operator/deploy"),
		vars,
		"melange.tmpl.yaml",
		"melange.yaml",
	)
}
