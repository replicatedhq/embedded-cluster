package main

import "dagger/embedded-cluster/internal/dagger"

const (
	APKOImageVersion    = "latest"
	MelangeImageVersion = "latest"
)

type EmbeddedCluster struct {
	common
	chainguard
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
