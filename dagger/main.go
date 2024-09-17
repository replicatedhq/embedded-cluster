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

	build := m.melangeBuildGo(ctx,
		func(c *dagger.Container) *dagger.Container {
			return c.WithFile("/workspace/common.mk", src.File("common.mk")).
				WithFile("/workspace/go.mod", src.File("go.mod")).
				WithFile("/workspace/go.sum", src.File("go.sum")).
				WithDirectory("/workspace/pkg", src.Directory("pkg")).
				WithDirectory("/workspace/kinds", src.Directory("kinds")).
				WithDirectory("/workspace/utils", src.Directory("utils")).
				WithDirectory("/workspace/operator", src.Directory("operator"))
		},
		melangeFile,
		arch,
		imageTag,
	)

	return build.Directory("build")
}
