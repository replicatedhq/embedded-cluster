package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

const (
	APKOImageVersion    = "latest"
	MelangeImageVersion = "latest"

	NodeVersion = "22"
)

type EmbeddedCluster struct {
	// +private
	Source       *dagger.Directory
	RegistryAuth *dagger.Directory

	// 1Password operations
	OnePassword *OnePassword

	common     common
	chainguard chainguard
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
	c := m.chainguard.apkoLogin(dag.Directory(), server, username, plain, APKOImageVersion)
	m.RegistryAuth = c.Directory("/workspace/.docker")
	return m, nil
}

// directoryWithCommonGoFiles sets up the filesystem with only what we need to build for improved
// caching.
func directoryWithCommonGoFiles(dir *dagger.Directory, src *dagger.Directory) *dagger.Directory {
	return dir.
		WithFile("common.mk", src.File("common.mk")).
		WithFile("versions.mk", src.File("versions.mk")).
		WithFile("go.mod", src.File("go.mod")).
		WithFile("go.sum", src.File("go.sum")).
		WithDirectory("pkg", src.Directory("pkg")).
		WithDirectory("pkg-new", src.Directory("pkg-new")).
		WithDirectory("cmd/installer/goods", src.Directory("cmd/installer/goods")).
		WithDirectory("api", src.Directory("api")).
		WithDirectory("kinds", src.Directory("kinds")).
		WithDirectory("utils", src.Directory("utils"))
}
