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
	k0SMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.File {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateOprator(src, ecVersion, k0SMinorVersion)

	pkgBuild := m.BuildOperatorPackage(src, ecVersion, k0SMinorVersion, arch)

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
	k0SMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (string, error) {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateOprator(src, ecVersion, k0SMinorVersion)

	pkgBuild := m.BuildOperatorPackage(src, ecVersion, k0SMinorVersion, arch)

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
	k0SMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.Directory {

	melangeFile := m.melangeTemplateOperator(src, ecVersion, k0SMinorVersion)

	build := m.chainguard.melangeBuildGo(
		directoryWithCommonFiles(dag.Directory(), src),
		melangeFile,
		arch,
		MelangeImageVersion,
	)

	return build.Directory("build")
}

func (m *EmbeddedCluster) apkoTemplateOprator(
	src *dagger.Directory,
	ecVersion string,
	k0SMinorVersion string,
) *dagger.File {
	vars := map[string]string{
		"PACKAGE_VERSION": ecVersion,
	}
	if k0SMinorVersion != "" {
		vars["K0S_MINOR_VERSION"] = k0SMinorVersion
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
	k0SMinorVersion string,
) *dagger.File {
	vars := map[string]string{
		"PACKAGE_VERSION": ecVersion,
	}
	if k0SMinorVersion != "" {
		vars["K0S_MINOR_VERSION"] = k0SMinorVersion
	}
	return m.common.renderTemplate(
		src.Directory("operator/deploy"),
		vars,
		"melange.tmpl.yaml",
		"melange.yaml",
	)
}
