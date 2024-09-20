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
	c := dag.Container().
		From("ubuntu:latest").
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "gettext"}).
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
