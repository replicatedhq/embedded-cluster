package main

import (
	"context"
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// BuildWeb builds the web UI using official Node.js image
//
// This can be called standalone or used internally by the build process.
//
// Example:
//
//	dagger call build-web --src=./web export --path=./web/dist
func (m *EmbeddedCluster) BuildWeb(
	ctx context.Context,
	// Source directory to use for the build.
	// +defaultPath="/"
	src *dagger.Directory,
) *dagger.Directory {
	// Create cache volume for npm to avoid re-downloading packages
	npmCache := dag.CacheVolume("ec-npm-cache")

	// The web build needs api/docs as a sibling directory (../api/docs)
	// Create a directory structure with both web and api/docs
	buildContainer := nodeBuildContainer().
		WithMountedCache("/root/.npm", npmCache).
		WithDirectory("/workspace", src.Directory("web")).
		// Install dependencies (cached via npm cache)
		WithExec([]string{"npm", "ci"}).
		// Build production bundle
		WithExec([]string{"npm", "run", "build"})

	return buildContainer.Directory("/workspace/web/dist")
}

// nodeBuildContainer returns a container with the necessary tools for building Node.js code.
func nodeBuildContainer() *dagger.Container {
	return dag.Container().
		From(fmt.Sprintf("node:%s-slim", NodeVersion)).
		WithWorkdir("/workspace")
}
