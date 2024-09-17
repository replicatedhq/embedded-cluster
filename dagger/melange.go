package main

import (
	"context"
	"fmt"

	"dagger/embedded-cluster/internal/dagger"
)

type melange struct{}

func (m *melange) melangeBuildGo(
	ctx context.Context,
	setupFilesystem func(c *dagger.Container) *dagger.Container,
	melangeFile *dagger.File,
	// +default="amd64,arm64"
	arch string,
	// +default="latest"
	imageTag string,
) *dagger.Container {

	keygen := m.melangeKeygen(ctx, imageTag)

	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/melange:%s", imageTag))

	c = setupFilesystem(c)

	c = c.WithFile("/workspace/melange.yaml", melangeFile).
		WithFile("/workspace/melange.rsa", keygen.File("/workspace/melange.rsa")).
		WithFile("/workspace/build/melange.rsa.pub", keygen.File("/workspace/melange.rsa.pub")).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithWorkdir("/workspace").
		WithExec(
			[]string{
				"melange", "build", "melange.yaml",
				"--signing-key", "melange.rsa",
				"--cache-dir", "/go/pkg/mod",
				"--arch", arch,
				"--out-dir", "build/packages/",
			},
			dagger.ContainerWithExecOpts{
				ExperimentalPrivilegedNesting: true,
				InsecureRootCapabilities:      true,
			},
		)

	return c
}

func (m *melange) melangeKeygen(
	ctx context.Context,
	// +default="latest"
	imageTag string,
) *dagger.Container {
	keygen := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/melange:%s", imageTag)).
		WithWorkdir("/workspace").
		WithExec([]string{"melange", "keygen"})

	return keygen
}
