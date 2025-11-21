package main

import (
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
	"strings"
)

type common struct {
}

func (m *common) renderTemplate(
	src *dagger.Directory,
	vars map[string]string,
	inFile string,
	outFile string,
) *dagger.File {
	c := ubuntuUtilsContainer().
		WithWorkdir("/workspace").
		WithDirectory("/workspace", src)

	keys := make([]string, 0, len(vars))
	for k, v := range vars {
		keys = append(keys, k)
		c = c.WithEnvVariable(k, v)
	}

	c = c.WithExec([]string{
		"sh",
		"-c",
		fmt.Sprintf("envsubst \"$(printf '${%%s} ' %s)\" < %s > %s", strings.Join(keys, " "), inFile, outFile),
	})

	return c.File(outFile)
}

// ubuntuUtilsContainer returns a container with the necessary tools for building.
func ubuntuUtilsContainer(opts ...dagger.ContainerOpts) *dagger.Container {
	return dag.Container(opts...).
		From("ubuntu:24.04").
		WithEnvVariable("TZ", "Etc/UTC").
		WithEnvVariable("DEBIAN_FRONTEND", "noninteractive").
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y",
			"build-essential", "cmake", "curl", "gettext", "git", "gzip", "jq", "libstdc++6", "make", "pkg-config", "tar", "unzip",
		})
}

// goBuildContainer returns a container with the necessary tools for building Go code.
func goBuildContainer() *dagger.Container {
	return dag.Container().
		From(fmt.Sprintf("golang:%s", GolangVersion))
}

// nodeBuildContainer returns a container with the necessary tools for building Node.js code.
func nodeBuildContainer() *dagger.Container {
	return dag.Container().
		From(fmt.Sprintf("node:%s-slim", NodeVersion))
}

// directoryWithCommonGoFiles sets up the filesystem with only what we need to build Go code for
// improved caching.
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
