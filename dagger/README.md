# Embedded Cluster Dagger Module

This directory contains Dagger modules for embedded-cluster development, build automation, and E2E testing.

## Development

Install dagger.

```bash
brew install dagger/tap/dagger
```

Run the `dagger develop` command to ensure that the SDK is installed, configured, and all its files re-generated.

```bash
dagger develop
```

## Modules

### Chainguard

Build local and CI container images using APKO and Melange. Stable release images are built and published by SecureBuild. Local builds adapt the recipes in `securebuild/package` and `securebuild/image`: they build the working source, preserve the requested binary version, and pin the image to locally signed head packages.

The release workflow waits for SecureBuild and then resolves Docker Hub images at
`replicated/embedded-cluster-operator-image:X.Y.Z-k8s1.N` and
`replicated/embedded-cluster-local-artifact-mirror:X.Y.Z-k8s1.N`. It checks both
architectures and pins the operator chart and installer to the published digests.
GitHub Actions no longer builds or publishes release images. SecureBuild's release
workflow supports stable versions; prerelease tags require images to have been
published separately under the same naming convention.

**Files:** `chainguard.go`, `securebuild.go`

### Local Artifact Mirror

Manage local artifact mirroring for airgap installations.

**Files:** `localartifactmirror.go`

### Operator

Build and publish the embedded-cluster operator.

**Files:** `operator.go`


