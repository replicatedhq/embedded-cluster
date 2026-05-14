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

Build Chainguard-based container images using APKO and Melange.

**Files:** `chainguard.go`, `common.go`

### Local Artifact Mirror

Manage local artifact mirroring for airgap installations.

**Files:** `localartifactmirror.go`

### Operator

Build and publish the embedded-cluster operator.

**Files:** `operator.go`


