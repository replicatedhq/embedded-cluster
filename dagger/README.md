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

### Build and Release

Dagger wrapper for building embedded-cluster artifacts and creating releases. Wraps existing build scripts for portability.

**Files:** `build.go`

#### Overview

The Build and Release module provides a portable, Dagger-based wrapper around the existing `scripts/build-and-release.sh` script. This preserves the battle-tested build logic while making it:

- **Portable**: Same build runs identically locally and in CI
- **Reproducible**: All dependencies are containerized
- **Secret-managed**: AWS and Replicated credentials via 1Password
- **Artifact-tracked**: Structured outputs with versioning

#### Quick Start

Build and release embedded-cluster artifacts with auto-detected versions:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release
```

With specific versions:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release \
  --ec-version="v1.2.3" \
  --app-version="appver-dev-abc123"
```

#### What It Does

The build-and-release function handles the complete build and release process:

1. **Build dependencies** (operator and local-artifact-mirror images)
2. **Build web UI** (React/TypeScript frontend)
3. **Build binary** (embedded-cluster CLI with embedded release)
4. **Upload to S3** (metadata.json with version information and artifact URLs)
5. **Create app release** (Replicated app channel release)

**Note:** Binary uploads (k0s, kots, operator) are currently skipped in Dagger due to Docker/crane/oras complexity.
For full binary uploads, run `scripts/ci-upload-binaries.sh` with `UPLOAD_BINARIES=1`.

#### Composable Build Steps

For faster iteration during development, you can run individual build steps instead of the full pipeline. Each step matches a script from `scripts/`:

##### Complete Standalone Workflow

Here's how to run the complete build pipeline using exported artifacts:

```bash
# Step 1: Build dependencies and export metadata
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-deps metadata export --path=./output/artifacts.json

# Step 2: Build binary using the metadata
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  build-metadata to-dir export --path=./output

# Step 3: Embed release using the binary directory
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  embed-release --build-dir=./output \
  binary export --path=./output/embedded-cluster.tgz
```

This approach allows you to:
- Run each step independently
- Iterate on individual steps without rebuilding everything
- Inspect intermediate artifacts between steps
- Resume from any step if a later step fails

##### 1. Build Dependencies (`build-deps`)

Builds and publishes operator and LAM images/charts (equivalent to `ci-build-deps.sh`):

```bash
# Build and export dependency metadata to ./output/deps.json
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-deps metadata export --path=./output/deps.json
```

Outputs:
- Operator image published to `ttl.sh/ec-build/embedded-cluster-operator-image:VERSION`
- Operator chart published to `oci://ttl.sh/ec-build/embedded-cluster-operator:VERSION`
- LAM image published to `ttl.sh/ec-build/embedded-cluster-local-artifact-mirror:VERSION`
- Metadata saved to `./output/deps.json` (JSON with version, image tags, chart URL)

##### 2. Build Binary (`build-bin`)

Builds web UI and binary without embedding release (equivalent to `ci-build-bin.sh`):

**Option A: Using deps metadata file (recommended):**

```bash
# First, export deps metadata
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-deps metadata export --path=./output/deps.json

# Then build binary using the metadata file
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  build-metadata to-dir export --path=./output
```

**Option B: Using individual parameters:**

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin \
    --ec-version="2.12.0+k8s-1.33-69-g4bba5" \
    --operator-image-tag="2.12.0-k8s-1.33-69-g4bba5" \
    --operator-image-repo="ttl.sh/ec-build/embedded-cluster-operator-image" \
    --lam-image-tag="2.12.0-k8s-1.33-69-g4bba5" \
    --operator-chart-url="oci://ttl.sh/ec-build/embedded-cluster-operator" \
  build-metadata to-dir export --path=./output
```

Outputs:
- Web UI assets at `./output/binary/web/`
- Unembedded binary at `./output/binary/bin/embedded-cluster`
- Metadata JSON at `./output/binary/metadata.yaml`

##### 3. Embed Release (`embed-release`)

Embeds KOTS release into binary (equivalent to `ci-embed-release.sh`):

**Option A: Using exported binary directory (standalone):**

```bash
# Assuming you already have ./output/binary from a previous build-bin run
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  embed-release --build-dir=./output \
  binary export --path=./output/embedded-cluster.tgz
```

**Option B: Chaining from build-bin:**

```bash
# Chain directly from build-bin
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  embed-release \
  binary export --path=./output/bin/embedded-cluster

# Export metadata JSON
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  embed-release \
  metadata export --path=./output/metadata.json

# Or export entire build directory
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  embed-release \
  build-metadata to-dir export --path=./output
```

Outputs:
- Embedded binary tarball accessible via `.binary` field
- Updated metadata accessible via `.metadata` field
- Full build directory accessible via `.build-dir` field

##### 4. Upload Binaries (`upload-bins`)

Uploads metadata to S3.

**Note:** This currently only uploads `metadata.json` due to Docker/crane/oras complexity in Dagger containers.
Binary uploads (k0s, kots, operator) are skipped. For full binary uploads, run `scripts/ci-upload-binaries.sh`
directly with `UPLOAD_BINARIES=1`.

**Option A: Using exported directory (standalone):**

```bash
# Assuming you already have ./output/binary from a previous build-bin run
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  upload-bins --build-dir=./output
```

**Option B: Chaining from embed-release:**

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  embed-release \
  upload-bins
```

Uploads:
- metadata.json (containing version information and artifact URLs)

##### 5. Create Release (`release-app`)

Creates Replicated app release (equivalent to `ci-release-app.sh`).

**Option A: Using exported directory (standalone):**

```bash
# Assuming you already have ./output/binary from a previous build-bin run
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  release-app --build-dir=./output
```

**Option B: Chaining from upload-bins:**

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  embed-release \
  upload-bins \
  release-app
```

Creates a new release in the Replicated app channel.

##### Iteration Examples

**Iterate on operator changes:**
```bash
# Just rebuild deps when operator code changes
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-deps metadata export --path=./output/deps.json

# View the published image tags
cat ./output/deps.json | jq .
```

**Iterate on web UI changes:**
```bash
# Rebuild binary with new web assets (reuses published deps from previous build)
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  build-dir export --path=./output

# Test the new binary
./output/binary/bin/embedded-cluster version
```

**Iterate on release YAML changes:**
```bash
# Skip binary rebuild and just re-embed with new release YAML
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  embed-release --build-dir=./output \
  binary export --path=./output/embedded-cluster.tgz
```

**Chain steps for quick iteration:**
```bash
# Chain build steps to embed release (uses Dagger caching when source unchanged)
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-bin --deps-metadata=./output/deps.json \
  embed-release \
  binary export --path=./output/embedded-cluster.tgz
```

#### Required Secrets

The following secrets are automatically fetched from 1Password (vault: "Developer Automation", item: "EC Dev"):

| Secret Field Name | Purpose |
|-------------------|---------|
| `ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID` | S3 access for artifact uploads |
| `ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY` | S3 secret key |
| `STAGING_REPLICATED_API_TOKEN` | Replicated API access for creating releases |

You can override these with command-line flags:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release \
  --aws-access-key-id=env:MY_AWS_KEY \
  --aws-secret-access-key=env:MY_AWS_SECRET \
  --replicated-apitoken=env:MY_REPLICATED_TOKEN
```

#### Advanced Options

##### Skip Release Creation

Build artifacts without creating a Replicated app release:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release --skip-release=true
```

##### Skip Metadata Upload

Build locally without uploading metadata to S3:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release --upload-binaries=false --skip-release=true
```

##### Custom Architecture

Build for arm64:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release --arch="arm64"
```

##### Use Chainguard Images

Build operator with Chainguard base images:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release --use-chainguard=true
```

#### Accessing Build Artifacts

After the build completes, you can access the artifacts:

```bash
# Get the version
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release version

# Export the binary
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release binary export --path=./embedded-cluster.tgz

# Export metadata
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release metadata export --path=./metadata.json

# Export entire build directory
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  build-and-release build-dir export --path=./output
```

#### Comparison with Direct Script Execution

| Feature | Direct Script | Dagger Wrapper |
|---------|--------------|----------------|
| Dependencies | Must install manually | Containerized, auto-installed |
| Secrets | Environment variables | 1Password integration |
| Portability | Linux-specific | Works on macOS, Linux, CI |
| Reproducibility | Depends on host | Always same container |
| CI Integration | Complex setup | Single command |

#### Troubleshooting

##### Docker Build Failures

**Problem**: `failed to build operator image`

**Solution**:
- Ensure Docker daemon is running
- Check disk space (builds require ~10GB)
- Verify network connectivity for pulling base images

##### AWS Upload Failures

**Problem**: `failed to upload to S3`

**Solution**:
- Verify AWS credentials are correct in 1Password
- Check S3 bucket exists and is accessible
- Verify IAM permissions allow PutObject

##### Version Detection Issues

**Problem**: Auto-detected version is not correct

**Solution**:
- Ensure you're in a git repository with tags
- Manually specify versions with `--ec-version` and `--app-version`
- Check git tags match the pattern `[0-9]*.[0-9]*.[0-9]*`

### E2E Testing (V3)

Portable E2E test framework for V3 installer with 1Password secret management and CMX VM provisioning.

**Files:** `e2e.go`, `cmx.go`, `onepassword.go`

#### Overview

The V3 E2E test framework provides:

- **Portable execution**: Same tests run identically locally and in CI
- **1Password integration**: Centralized secret management
- **CMX VM provisioning**: Isolated test environments using Replicated's CMX
- **Dagger-based orchestration**: Reproducible builds and tests

#### Architecture

```
┌────────────────────────────────────────────────────────┐
│                  Dagger CLI Interface                  │
├────────────────────────────────────────────────────────┤
│                                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │  E2E Module  │  │  1Password   │  │  Replicated  │  │
│  │              │  │   Module     │  │    Module    │  │
│  ├──────────────┤  ├──────────────┤  ├──────────────┤  │
│  │ - VM Mgmt    │  │ - Secrets    │  │ - CMX VMs    │  │
│  │ - Tests      │  │ - Creds      │  │ - Networking │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└────────────────────────────────────────────────────────┘
```

#### Prerequisites

##### Required Tools

1. **Dagger CLI** (v0.9+)
   ```bash
   curl -fsSL https://dl.dagger.io/dagger/install.sh | sh
   sudo mv ./bin/dagger /usr/local/bin/dagger
   ```

2. **1Password Service Account Token**

   The E2E framework uses 1Password for secret management. You need a service account token with access to the secrets.

   ```bash
   export OP_SERVICE_ACCOUNT_TOKEN="your-token-here"
   ```

   All Dagger commands must include the `with-one-password` configuration:
   ```bash
   dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN ...
   ```

##### Required Secrets

The following secrets must be available in 1Password in the **"Developer Automation"** vault under the **"EC Dev"** item:

| Secret Field Name | Purpose |
|-------------------|---------|
| `CMX_REPLICATED_API_TOKEN` | CMX API access for VM provisioning |
| `CMX_SSH_PRIVATE_KEY` | SSH private key for accessing provisioned VMs |

**Note:** The vault name defaults to "Developer Automation" and can be overridden via `--vault-name` parameter in the `with-one-password` call.

Additional secrets (for future PRs):
- `STAGING_REPLICATED_API_TOKEN` - Replicated API access for creating releases
- `STAGING_EMBEDDED_CLUSTER_UPLOAD_IAM_KEY_ID` - AWS S3 access key for artifact uploads
- `STAGING_EMBEDDED_CLUSTER_UPLOAD_IAM_SECRET` - AWS S3 secret key
- `GITHUB_TOKEN` - GitHub API access

#### Quick Start

##### 1. Provision a Test VM

Create a fresh CMX VM for testing:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  test-provision-vm --name="my-test-vm" string
```

This will:
- Create a Ubuntu 22.04 VM with default settings
- Wait for SSH to become available
- Discover the private IP address
- Return VM details (ID, name, network ID, IP address)

##### 2. Run Commands on VM

Execute commands on the provisioned VM:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  test-provision-vm run-command --command="ls,-la,/tmp"
```

##### 3. Cleanup

Remove the VM when done:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  test-provision-vm cleanup
```

#### Available Commands

Initializes the E2E test module. 1Password integration is configured via `with-one-password`.

##### VM Provisioning

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  test-provision-vm \
  --name="test-vm" \
  --distribution="ubuntu" \
  --version="22.04" \
  --instance-type="r1.medium" \
  --disk-size=50 \
  --wait="10m" \
  --ttl="2h"
```

**Parameters:**
- `name`: VM name (default: "ec-e2e-test")
- `distribution`: OS distribution (default: "ubuntu")
- `version`: Distribution version (default: "22.04")
- `instance-type`: Instance type (default: "r1.medium")
- `disk-size`: Disk size in GB (default: 50)
- `wait`: Wait timeout for VM to be ready (default: "10m")
- `ttl`: VM lifetime (default: "2h")

##### Run Commands

Execute commands on a provisioned VM:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  test-provision-vm run-command --command="ls,-la,/tmp"
```

Commands are automatically executed with `sudo` and `PATH=$PATH:/usr/local/bin`.

##### Expose Port

Expose a port on the VM and get a public hostname:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  test-provision-vm expose-port --port=30000 --protocol="https"
```

**Parameters:**
- `port`: Port number to expose
- `protocol`: Protocol (default: "https")

##### Cleanup VM

Remove a provisioned VM:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  test-provision-vm cleanup
```

#### Test Scenarios (PR 1 Foundation Only)

This is **PR 1: Foundation and Secret Management**. The following scenarios will be implemented in future PRs:

##### Future: PR 3 - Headless Installation Tests
- Online installation (headless CLI)
- Airgap installation (headless CLI)

##### Future: PR 5 - Browser-Based Installation Tests
- Online installation (Playwright UI)
- Airgap installation (Playwright UI)

#### Local Development

To develop E2E tests locally:

1. Set up 1Password access:
   ```bash
   export OP_SERVICE_ACCOUNT_TOKEN="your-token"
   ```

2. Test VM provisioning:
   ```bash
   dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
     test-provision-vm --name="dev-test"
   ```

3. Make changes to `dagger/e2e.go`

#### CI Integration

CI integration will be added in **PR 4: CI Integration + Documentation**.

The CI workflow will:
- Use GitHub Actions secrets for 1Password access
- Run E2E tests in parallel using isolated CMX VMs
- Collect test results and artifacts

#### Troubleshooting

##### 1Password Access Issues

**Problem**: `failed to access 1Password secrets`

**Solution**:
- Verify `OP_SERVICE_ACCOUNT_TOKEN` is set correctly
- Check 1Password service account has access to required secrets
- Validate secret names match exactly (case-sensitive)

##### CMX VM Provisioning Failures

**Problem**: `timed out waiting for ssh`

**Solution**:
- Check CMX API token is valid
- Verify network connectivity
- Increase `--wait` timeout if needed

##### SSH Connection Issues

**Problem**: SSH commands fail with connection errors

**Solution**:
- Wait for VM to fully initialize (may take 30s after "ready" status)
- Check SSH endpoint is accessible
- Verify SSH keys are configured correctly

#### Reference

##### External Modules Used

- **1Password Module**: Secret management
  - Repository: `github.com/replicatedhq/daggerverse/onepassword`
  - Functions: `FindSecret()`

- **Replicated Module**: CMX VM management
  - Repository: `github.com/replicatedhq/daggerverse/replicated`
  - Functions: `VMCreate`, `VMRemove`, `VMExposePort`

##### Related Documentation

- [Main embedded-cluster README](../README.md)
- [V3 E2E Tests Proposal](../proposals/v3_e2e_tests.md)
- [Dagger Documentation](https://docs.dagger.io/)
- [CMX Documentation](https://docs.replicated.com/vendor/testing-about)

##### Support

For questions or issues:
1. Check this README and troubleshooting section
2. Review the [V3 E2E Tests Proposal](../proposals/v3_e2e_tests.md)
3. Ask in #embedded-cluster Slack channel
