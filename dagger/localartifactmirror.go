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
	// K0s minor version to build for.
	// +default=""
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.File {

	tag := strings.ReplaceAll(ecVersion, "+", "-")
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateLocalArtifactMirror(src, ecVersion, kzerosMinorVersion)

	pkgBuild := m.BuildLocalArtifactMirrorPackage(src, ecVersion, kzerosMinorVersion, arch)

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
	// K0s minor version to build for.
	// +default=""
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) (string, error) {

	tag := strings.ReplaceAll(ecVersion, "+", "-")
	image := fmt.Sprintf("%s:%s", repo, tag)

	apkoFile := m.apkoTemplateLocalArtifactMirror(src, ecVersion, kzerosMinorVersion)

	pkgBuild := m.BuildLocalArtifactMirrorPackage(src, ecVersion, kzerosMinorVersion, arch)

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

// Builds the local artifact mirror package with Melange.
func (m *EmbeddedCluster) BuildLocalArtifactMirrorPackage(
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Version to use for the package.
	ecVersion string,
	// K0s minor version to build for.
	kzerosMinorVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.Directory {

	melangeFile := m.melangeTemplateLocalArtifactMirror(src, ecVersion, kzerosMinorVersion)

	dir := dag.Directory().
		WithDirectory("local-artifact-mirror",
			src.Directory("local-artifact-mirror").
				WithoutDirectory("bin").
				WithoutDirectory("build").
				WithoutDirectory("cache"),
		).
		WithDirectory("cmd",
			src.Directory("cmd").
				WithoutDirectory("installer/goods/bins").
				WithNewFile("installer/goods/bins/.placeholder", ".placeholder").
				WithoutDirectory("installer/goods/internal/bins").
				WithNewFile("installer/goods/internal/bins/.placeholder", ".placeholder"),
		)

	build := m.chainguard.melangeBuildGo(
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
	k0sMinorVersion string,
) *dagger.File {
	vars := map[string]string{
		"PACKAGE_VERSION": ecVersion,
	}
	if k0sMinorVersion != "" {
		vars["K0S_MINOR_VERSION"] = k0sMinorVersion
	}
	return m.common.renderTemplate(
		src.Directory("local-artifact-mirror/deploy"),
		vars,
		"apko.tmpl.yaml",
		"apko.yaml",
	)
}

func (m *EmbeddedCluster) melangeTemplateLocalArtifactMirror(
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
		src.Directory("local-artifact-mirror/deploy"),
		vars,
		"melange.tmpl.yaml",
		"melange.yaml",
	)
}
