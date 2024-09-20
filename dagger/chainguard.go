package main

import (
	"fmt"

	"dagger/embedded-cluster/internal/dagger"
)

type chainguard struct{}

func (m *chainguard) melangeBuildGo(
	src *dagger.Directory,
	melangeFile *dagger.File,
	// +default="amd64,arm64"
	arch string,
	// +default="latest"
	imageTag string,
) *dagger.Container {

	keygen := m.melangeKeygen(imageTag)

	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/melange:%s", imageTag)).
		WithDirectory("/workspace", src).
		WithFile("/workspace/melange.yaml", melangeFile).
		WithFile("/workspace/melange.rsa", keygen.File("/workspace/melange.rsa")).
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

	// for output
	c = c.WithFile("/workspace/build/melange.rsa.pub", keygen.File("/workspace/melange.rsa.pub"))

	return c
}

func (m *chainguard) melangeKeygen(
	// +default="latest"
	imageTag string,
) *dagger.Container {
	keygen := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/melange:%s", imageTag)).
		WithWorkdir("/workspace").
		WithExec([]string{"melange", "keygen"})

	return keygen
}

func (m *chainguard) apkoBuild(
	src *dagger.Directory,
	apkoFile *dagger.File,
	image string,
	// +default="amd64,arm64"
	arch string,
	// +default="latest"
	imageTag string,
) *dagger.Container {

	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/apko:%s", imageTag)).
		WithDirectory("/workspace", src).
		WithFile("/workspace/apko.yaml", apkoFile).
		WithWorkdir("/workspace").
		WithExec(
			[]string{
				"apko", "build", "apko.yaml", image, "apko.tar",
				"--cache-dir", "/go/pkg/mod",
				"--arch", arch,
			},
		)

	return c
}

func (m *chainguard) apkoPublish(
	src *dagger.Directory,
	apkoFile *dagger.File,
	image string,
	// +default="amd64,arm64"
	arch string,
	// +default="latest"
	imageTag string,
) *dagger.Container {

	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/apko:%s", imageTag)).
		WithDirectory("/workspace", src).
		WithFile("/workspace/apko.yaml", apkoFile).
		WithWorkdir("/workspace").
		WithExec(
			[]string{
				"apko", "publish", "apko.yaml", image,
				"--cache-dir", "/go/pkg/mod",
				"--arch", arch,
			},
		)

	return c
}
