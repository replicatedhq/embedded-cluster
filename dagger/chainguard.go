package main

import (
	"context"
	"fmt"
	"time"

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

	// Create cache volumes for improved build performance
	goCache := dag.CacheVolume("ec-melange-gocache")
	apkCache := dag.CacheVolume("ec-melange-apkcache")

	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/melange:%s", imageTag)).
		WithDirectory("/workspace", src).
		WithFile("/workspace/melange.yaml", melangeFile).
		WithFile("/workspace/melange.rsa", keygen.File("/workspace/melange.rsa")).
		WithEnvVariable("MELANGE_CACHE_DIR", "/cache/melange").
		WithEnvVariable("MELANGE_APK_CACHE_DIR", "/cache/apk").
		WithMountedCache("/cache/melange", goCache, dagger.ContainerWithMountedCacheOpts{
			Sharing: dagger.CacheSharingModeShared,
		}).
		WithMountedCache("/cache/apk", apkCache, dagger.ContainerWithMountedCacheOpts{
			Sharing: dagger.CacheSharingModeShared,
		}).
		WithWorkdir("/workspace").
		WithExec(
			[]string{
				"melange", "build", "melange.yaml",
				"--signing-key", "melange.rsa",
				"--cache-dir", "/cache/melange",
				"--apk-cache-dir", "/cache/apk",
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

	// Create cache volumes for APK package indexes and downloads
	apkoCache := dag.CacheVolume("ec-apko-cache")

	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/apko:%s", imageTag)).
		WithDirectory("/workspace", src).
		WithFile("/workspace/apko.yaml", apkoFile).
		WithMountedCache("/cache/apko", apkoCache, dagger.ContainerWithMountedCacheOpts{
			Sharing: dagger.CacheSharingModeShared,
		}).
		WithWorkdir("/workspace").
		WithExec(
			[]string{
				"apko", "build", "apko.yaml", image, "apko.tar",
				"--cache-dir", "/cache/apko",
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

	// Create cache volumes for APK package indexes and downloads
	apkoCache := dag.CacheVolume("ec-apko-cache")

	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/apko:%s", imageTag)).
		WithDirectory("/workspace", src).
		WithFile("/workspace/apko.yaml", apkoFile).
		WithMountedCache("/cache/apko", apkoCache, dagger.ContainerWithMountedCacheOpts{
			Sharing: dagger.CacheSharingModeShared,
		}).
		WithEnvVariable("DOCKER_CONFIG", "/workspace/.docker").
		WithWorkdir("/workspace").
		WithExec(
			[]string{
				"apko", "publish", "apko.yaml", image,
				"--cache-dir", "/cache/apko",
				"--arch", arch,
			},
		)

	return c
}

// retryPublish retries the publish operation up to maxRetries times with
// exponential backoff. The *dagger.Container pipeline is reused between
// attempts, so the expensive build is cached and only the registry push is
// retried.
func (m *chainguard) retryPublish(
	ctx context.Context,
	publish *dagger.Container,
	maxRetries int,
	initialBackoff time.Duration,
) (string, error) {
	var lastErr error
	backoff := initialBackoff
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			fmt.Printf("Publish attempt %d/%d failed, retrying in %v: %v\n", i, maxRetries, backoff, lastErr)
			time.Sleep(backoff)
			backoff *= 2
		}
		out, err := publish.Stdout(ctx)
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func (m *chainguard) apkoLogin(
	src *dagger.Directory,
	registryServer string,
	registryUsername string,
	registryPassword string,
	// +default="latest"
	imageTag string,
) *dagger.Container {
	c := dag.Container().
		From(fmt.Sprintf("cgr.dev/chainguard/apko:%s", imageTag)).
		WithDirectory("/workspace", src).
		WithEnvVariable("DOCKER_CONFIG", "/workspace/.docker").
		WithWorkdir("/workspace").
		WithExec([]string{
			"apko", "login", registryServer,
			"--username", registryUsername,
			"--password-stdin",
		}, dagger.ContainerWithExecOpts{
			Stdin: registryPassword,
		})

	return c
}
