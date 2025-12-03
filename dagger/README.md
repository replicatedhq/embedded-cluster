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

   The E2E framework uses 1Password for secret management. You need a service account token with access to the Vault.

   ```bash
   export OP_SERVICE_ACCOUNT_TOKEN="your-token-here"
   ```

   All Dagger commands must include the `with-one-password` configuration:
   ```bash
   dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN ...
   ```

##### Required Secrets

The following secrets are available in 1Password in the **"Developer Automation"** vault under the **"EC Dev"** item:

| Secret Field Name | Purpose |
|-------------------|---------|
| `CMX_REPLICATED_API_TOKEN` | CMX API access for VM provisioning |
| `CMX_SSH_PRIVATE_KEY` | SSH private key for accessing provisioned VMs |
| `ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID` | AWS S3 access key for artifact uploads |
| `ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY` | AWS S3 secret key for artifact uploads |
| `STAGING_REPLICATED_API_TOKEN` | Replicated API access for creating releases |

**Note:** The vault name defaults to "Developer Automation" and can be overridden via `--vault-name` parameter in the `with-one-password` call.

#### Quick Start

##### Building a Release

Before running E2E tests, you must build and release a version of embedded-cluster that can be tested against:

```bash
make e2e-v3-initial-release
```

This command:
- Builds the embedded-cluster binaries
- Uploads artifacts to S3
- Creates a release in the Replicated staging environment
- Generates an app version that can be used in E2E tests (e.g., `appver-dev-xpXCTO`)

The app version returned by this command should be used as the `--app-version` parameter in your E2E test commands.

##### Running a Complete E2E Test

Run a complete online headless installation test (provisions VM, installs, validates, and cleans up):

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  e-2-e-run-headless-online \
  --app-version=appver-dev-xpXCTO \
  --kube-version=1.33 \
  --license-file=./local-dev/ethan-dev-license.yaml
```

This will:
- Provision a fresh Ubuntu 22.04 VM (8GB RAM, 4 CPUs)
- Perform a headless CLI installation
- Validate the installation
- Clean up the VM automatically
- Return comprehensive test results

##### Running Test Steps Individually

For debugging or development, you can run each test step separately:

**1. Provision a Test VM:**

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  provision-cmx-vm --name="my-test-vm" string
```

This returns the VM ID which you'll use in subsequent steps.

**2. Install Embedded Cluster:**

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  with-cmx-vm --vm-id=YOUR_VM_ID \
  install-headless \
  --scenario=online \
  --app-version=appver-dev-xpXCTO \
  --license-file=./local-dev/ethan-dev-license.yaml \
  --config-values-file=./assets/config-values.yaml
```

**3. Validate Installation:**

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  with-cmx-vm --vm-id=YOUR_VM_ID \
  validate \
  --scenario=online \
  --expected-kube-version=1.33 \
  --expected-app-version=appver-dev-xpXCTO \
  string
```

**4. Cleanup (when done):**

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  with-cmx-vm --vm-id=YOUR_VM_ID \
  cleanup
```

#### Available Commands

Initializes the E2E test module. 1Password integration is configured via `with-one-password`.

##### VM Provisioning

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  provision-cmx-vm \
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
  provision-cmx-vm run-command --command="ls,-la,/tmp"
```

Commands are automatically executed with `sudo` and `PATH=$PATH:/usr/local/bin`.

##### Expose Port

Expose a port on the VM and get a public hostname:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  provision-cmx-vm expose-port --port=30000 --protocol="https"
```

**Parameters:**
- `port`: Port number to expose
- `protocol`: Protocol (default: "https")

##### Cleanup VM

Remove a provisioned VM:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  provision-cmx-vm cleanup
```

##### Install Headless

Perform a headless (CLI) installation of embedded-cluster:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  provision-cmx-vm install-headless \
  --scenario=online \
  --app-version=v1.0.0 \
  --license="..." \
  --license-id="..."
```

**Parameters:**
- `scenario`: Installation scenario ("online" or "airgap")
- `app-version`: App version to install
- `license`: License content
- `license-id`: License ID for downloading
- `config-file`: Optional config file content

##### Validate Installation

Validate an embedded-cluster installation after it completes:

```bash
dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
  provision-cmx-vm validate \
  --expected-k8s-version=1.31 \
  --expected-app-version=v1.0.0 \
  --airgap=false
```

**Parameters:**
- `expected-k8s-version`: Expected Kubernetes version (e.g., "1.31")
- `expected-app-version`: Expected app version (e.g., "v1.0.0")
- `airgap`: Whether to use airgap validation mode (default: false)

**Validation Checks:**

The validation performs comprehensive checks including:

1. **Kubernetes Cluster Health**
   - Verifies all nodes are running the expected k8s version
   - Checks kubelet version matches expected version on all nodes
   - Validates node readiness status

2. **Installation CRD Status**
   - Verifies Installation resource exists and is in "Installed" state
   - Confirms embedded-cluster operator successfully completed installation

3. **Application Deployment**
   - Waits for application's nginx pods to be Running
   - Verifies correct app version is deployed
   - Confirms no upgrade artifacts present

4. **Admin Console Components**
   - Confirms kotsadm pods are healthy
   - Confirms kotsadm API is healthy (kubectl kots get apps works)
   - Validates admin console branding configmap has DR label

5. **Data Directory Configuration**
   - Validates K0s data directory is configured correctly
   - Validates OpenEBS data directory is configured correctly
   - Validates Velero pod volume path is configured correctly
   - Verifies all components use expected base directory

6. **Pod and Job Health**
   - All non-Job pods are in Running/Completed/Succeeded state
   - All Running pods have ready containers
   - All Jobs have completed successfully

**Return Value:**

Returns a `ValidationResult` containing:
- `Success`: Overall validation status (bool)
- `ClusterHealth`: Cluster health check result
- `InstallationCRD`: Installation CRD check result
- `AppDeployment`: App deployment check result
- `AdminConsole`: Admin console check result
- `DataDirectories`: Data directories check result
- `PodsAndJobs`: Pod and job health check result

Each check result includes:
- `Passed`: Whether the check passed (bool)
- `ErrorMessage`: Error message if failed (string)
- `Details`: Additional context or details (string)

#### Test Scenarios

This framework supports the following E2E test scenarios:

##### Headless Installation Tests (PR 3)
- Online installation (headless CLI)
- Airgap installation (headless CLI)
- Comprehensive validation after installation

##### Future: Browser-Based Installation Tests (PR 5)
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
     provision-cmx-vm --name="dev-test"
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
