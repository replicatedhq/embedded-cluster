package main

import (
	"context"
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
		}).
		// Install Replicated CLI
		WithExec([]string{"sh", "-c", "curl -fsSL https://raw.githubusercontent.com/replicatedhq/replicated/main/install.sh | bash"}).
		// Install Helm
		WithExec([]string{"sh", "-c", "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"}).
		// Install crane (needed to extract kots binary from kotsadm image)
		WithExec([]string{"sh", "-c", "curl -fsSL https://github.com/google/go-containerregistry/releases/latest/download/go-containerregistry_Linux_x86_64.tar.gz | tar -xzf - -C /usr/local/bin crane"}).
		// Install AWS CLI (only tool needed for metadata upload)
		WithExec([]string{"sh", "-c", "curl -fsSL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o /tmp/awscliv2.zip"}).
		WithExec([]string{"sh", "-c", "cd /tmp && unzip -q awscliv2.zip && ./aws/install"}).
		// Set the working directory to /workspace
		WithWorkdir("/workspace").
		// Configure Git to allow unsafe directories
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "/workspace"})

}

// goBuildContainer returns a container with the necessary tools for building Go code.
func goBuildContainer() *dagger.Container {
	return dag.Container().
		From(fmt.Sprintf("golang:%s", GolangVersion)).
		WithWorkdir("/workspace")
}

// nodeBuildContainer returns a container with the necessary tools for building Node.js code.
func nodeBuildContainer() *dagger.Container {
	return dag.Container().
		From(fmt.Sprintf("node:%s-slim", NodeVersion)).
		WithWorkdir("/workspace")
}

// directoryWithCommonFiles sets up the filesystem with only what we need to build code for
// improved caching.
func directoryWithCommonFiles(dir *dagger.Directory, src *dagger.Directory) *dagger.Directory {
	dir = dir.
		WithDirectory("api", src.Directory("api")).
		WithDirectory("cmd", src.Directory("cmd")).
		WithDirectory("e2e", src.Directory("e2e")).
		WithDirectory("kinds", src.Directory("kinds")).
		WithDirectory("local-artifact-mirror", src.Directory("local-artifact-mirror")).
		WithDirectory("operator", src.Directory("operator")).
		WithDirectory("pkg", src.Directory("pkg")).
		WithDirectory("pkg-new", src.Directory("pkg-new")).
		WithDirectory("scripts", src.Directory("scripts")).
		WithDirectory("utils", src.Directory("utils")).
		WithDirectory("web", src.Directory("web")).
		WithFile(".gitignore", src.File(".gitignore")).
		WithFile(".golangci.yml", src.File(".golangci.yml")).
		WithFile("common.mk", src.File("common.mk")).
		WithFile("go.mod", src.File("go.mod")).
		WithFile("go.sum", src.File("go.sum")).
		WithFile("Makefile", src.File("Makefile")).
		WithFile("versions.mk", src.File("versions.mk"))

	// Exclude directories and files that aren't needed to speed up syncing
	dir = dir.WithoutDirectory("cmd/installer/goods/bins")
	dir = dir.WithoutDirectory("cmd/installer/goods/images")
	dir = dir.WithoutDirectory("cmd/installer/goods/internal/bins")
	dir = dir.WithoutDirectory("e2e/playwright/node_modules")
	dir = dir.WithoutDirectory("e2e/playwright/playwright-report")
	dir = dir.WithoutDirectory("e2e/playwright/test-results")
	dir = dir.WithoutDirectory("kinds/bin")
	dir = dir.WithoutDirectory("local-artifact-mirror/bin")
	dir = dir.WithoutDirectory("local-artifact-mirror/build")
	dir = dir.WithoutDirectory("operator/bin")
	dir = dir.WithoutDirectory("operator/build")
	dir = dir.WithoutDirectory("web/node_modules")

	// Add placeholder files to ensure the directories exist
	dir = dir.WithFile("cmd/installer/goods/bins/.placeholder", src.File("cmd/installer/goods/bins/.placeholder"))
	dir = dir.WithFile("cmd/installer/goods/internal/bins/.placeholder", src.File("cmd/installer/goods/internal/bins/.placeholder"))

	return dir
}

// ShowDirectorySizes shows the sizes of directories after applying gitignore exclusions.
// This is useful for debugging and verifying that exclusions are working correctly.
//
// Example:
//
//	dagger call show-directory-sizes
func (m *EmbeddedCluster) ShowDirectorySizes(
	ctx context.Context,
	// Source directory to analyze (defaults to current directory)
	// +defaultPath="/"
	// +optional
	src *dagger.Directory,
) (string, error) {
	// Create a directory with common files (which applies gitignore exclusions)
	dir := directoryWithCommonFiles(dag.Directory(), src)

	// Run du -sh on all top-level directories
	output, err := ubuntuUtilsContainer().
		WithDirectory("/workspace", dir).
		WithWorkdir("/workspace").
		WithExec([]string{"sh", "-c", "du -sh * .git 2>/dev/null | sort -h"}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to calculate directory sizes: %w", err)
	}

	return output, nil
}
