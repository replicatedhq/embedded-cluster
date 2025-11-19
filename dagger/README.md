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

The following secrets must be available in 1Password in the **"Developer Automation"** vault under the **"EC CI"** item:

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
