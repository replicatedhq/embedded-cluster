package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

const (
	APKOImageVersion    = "latest"
	MelangeImageVersion = "latest"

	GolangVersion = "1.25"
	NodeVersion   = "22"
)

type EmbeddedCluster struct {
	RegistryAuth *dagger.Directory

	// 1Password operations
	OnePassword *OnePassword

	// Build metadata
	BuildMetadata *BuildMetadata

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
