package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

const (
	APKOImageVersion    = "latest"
	MelangeImageVersion = "latest"
)

type EmbeddedCluster struct {
	RegistryAuth *dagger.Directory

	common
	chainguard
}

func (m *EmbeddedCluster) WithRegistryLogin(
	ctx context.Context,
	server string,
	username string,
	password *dagger.Secret,
) (*EmbeddedCluster, error) {
	plain, err := password.Plaintext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get registry password from secret: %w", err)
	}
	c := m.apkoLogin(dag.Directory(), server, username, plain, APKOImageVersion)
	m.RegistryAuth = c.Directory("/workspace/.docker")
	return m, nil
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
