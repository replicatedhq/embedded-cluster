package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/embedded-cluster/internal/dagger"
)

// Builds the local artifact mirror image with APKO.
func (m *EmbeddedCluster) BuildLocalArtifactMirrorImage(
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Repository to use for the image.
	// +default="replicated/embedded-cluster-local-artifact-mirror"
	repo string,
	// Version to use for the package.
	ecVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.File {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateLocalArtifactMirror(src, ecVersion)

	pkgBuild := m.BuildLocalArtifactMirrorPackage(src, ecVersion, arch)

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

// Builds and publishes the local artifact mirror image with APKO.
func (m *EmbeddedCluster) PublishLocalArtifactMirrorImage(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Repository to use for the image.
	// +default="replicated/embedded-cluster-local-artifact-mirror"
	repo string,
	// Version to use for the package.
	ecVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (string, error) {

	tag := strings.Replace(ecVersion, "+", "-", -1)
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateLocalArtifactMirror(src, ecVersion)

	pkgBuild := m.BuildLocalArtifactMirrorPackage(src, ecVersion, arch)

	dir := dag.Directory().
		WithFile("melange.rsa.pub", pkgBuild.File("melange.rsa.pub")).
		WithDirectory("packages", pkgBuild.Directory("packages"))

	if m.RegistryAuth != nil {
		dir = dir.WithDirectory(".docker", m.RegistryAuth)
	}

	publish := m.apkoPublish(
		dir,
		apkoFile,
		image,
		arch,
		APKOImageVersion,
	)

	return publish.Stdout(ctx)
}

// Builds the local artifact mirror package with Melange.
func (m *EmbeddedCluster) BuildLocalArtifactMirrorPackage(
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Version to use for the package.
	ecVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.Directory {

	melangeFile := m.melangeTemplateLocalArtifactMirror(src, ecVersion)

	dir := dag.Directory().
		WithDirectory("local-artifact-mirror", src.Directory("local-artifact-mirror")).
		WithDirectory("cmd", src.Directory("cmd"))

	build := m.melangeBuildGo(
		directoryWithCommonGoFiles(dir, src),
		melangeFile,
		arch,
		MelangeImageVersion,
	)

	return build.Directory("build")
}

func (m *EmbeddedCluster) apkoTemplateLocalArtifactMirror(
	src *dagger.Directory,
	ecVersion string,
) *dagger.File {
	return m.renderTemplate(
		src.Directory("local-artifact-mirror/deploy"),
		map[string]string{
			"PACKAGE_VERSION": ecVersion,
		},
		"apko.tmpl.yaml",
		"apko.yaml",
	)
}

func (m *EmbeddedCluster) melangeTemplateLocalArtifactMirror(
	src *dagger.Directory,
	ecVersion string,
) *dagger.File {
	return m.renderTemplate(
		src.Directory("local-artifact-mirror/deploy"),
		map[string]string{
			"PACKAGE_VERSION": ecVersion,
		},
		"melange.tmpl.yaml",
		"melange.yaml",
	)
}
