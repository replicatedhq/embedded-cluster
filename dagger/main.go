package main

import (
	"context"

	"dagger/embedded-cluster/internal/dagger"
)

// dagger call build-operator-package --ec-version test export --path build

type EmbeddedCluster struct {
	common
	melange
}

func (m *EmbeddedCluster) BuildOperatorPackage(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
	ecVersion string,
	// +default="amd64,arm64"
	arch string,
	// +default="latest"
	imageTag string,
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
		imageTag,
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
