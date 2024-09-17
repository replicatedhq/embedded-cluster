package main

import (
	"context"

	"dagger/embedded-cluster/internal/dagger"
)

const (
	MelangeImageVersion = "latest"
)

type EmbeddedCluster struct {
	common
	melange
}

// Builds the operator package with Melange.
func (m *EmbeddedCluster) BuildOperatorPackage(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
	// Version to use for the package.
	ecVersion string,
	// Architectures to build for.
	// +default="amd64,arm64"
	arch string,
) *dagger.Directory {

	melangeFile := m.renderTemplate(
		src.Directory("operator/deploy"),
		map[string]string{
			"PACKAGE_VERSION": ecVersion,
		},
		"melange.tmpl.yaml",
		"melange.yaml",
	)

	dir := dag.Directory().
		WithDirectory("operator", src.Directory("operator"))

	build := m.melangeBuildGo(ctx,
		directoryWithCommonGoFiles(dir, src),
		melangeFile,
		arch,
		MelangeImageVersion,
	)

	return build.Directory("build")
}

// directoryWithCommonGoFiles sets up the filesystem with only what we need to build for improved
// caching.
func directoryWithCommonGoFiles(dir *dagger.Directory, src *dagger.Directory) *dagger.Directory {
	return dir.
		WithFile("common.mk", src.File("common.mk")).
		WithFile("go.mod", src.File("go.mod")).
		WithFile("go.sum", src.File("go.sum")).
		WithDirectory("pkg", src.Directory("pkg")).
		WithDirectory("kinds", src.Directory("kinds")).
		WithDirectory("utils", src.Directory("utils"))
}
