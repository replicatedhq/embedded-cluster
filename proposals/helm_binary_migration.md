# Helm Binary Migration Proposal

## Executive Summary

Replace the Helm Go SDK with direct helm binary execution for **all Embedded Cluster installs (V2 and V3)**. This approach aligns with KOTS' existing helm binary usage, reducing migration complexity and potential regressions when porting functionality from KOTS.

## Problem Statement

The current Helm Go SDK integration presents several challenges:
- **Migration Complexity**: Using the SDK instead of the binary adds complexity and potential for regressions when migrating from KOTS, which uses the helm binary directly.
- **Compatibility Issues**: SDK behavior may diverge from CLI behavior in edge cases.
- **Debugging Complexity**: SDK errors are harder to diagnose than CLI output.
- **Stability**: The Helm CLI interface seems to be more commonly used and robust than the SDK

## Proposed Solution

### Architecture Overview

This proposal replaces the Helm Go SDK with direct binary execution while maintaining the exact same API interface. The change is transparent to all consumers and only affects the internal implementation.

#### Current State (SDK-based)
```
App/Installer → pkg/helm/interface.go → pkg/helm/client.go → Helm Go SDK → Kubernetes API
```

#### Proposed State (Binary-based) 
```
App/Installer → pkg/helm/interface.go → pkg/helm/client.go → helm binary → Kubernetes API
```

### Implementation Architecture

**Application Layer (No Changes)**
• api/, cmd/embedded-cluster/, etc.
• All existing code continues to work unchanged

↓

**Helm Interface (No Changes)**
• pkg/helm/interface.go maintains same Client interface
• Same method signatures, return types, and error handling

↓

**Unified Binary Implementation:**
• pkg/helm/client.go (refactored to use helm binary)
• HelmClient struct (same name, different implementation)
• Command execution via helpers.RunCommand
• JSON output parsing with stdout/stderr capture
• Error handling and logging
• binaryExecutor interface (mockable for tests)
• Uses helm binary from cmd/installer/goods/materializer.go

### Migration Strategy
**Single-phase migration**: Refactor existing `pkg/helm/client.go` to use binary execution instead of Go SDK for **both V2 and V3** installs.

- Replace SDK calls with helm binary execution via helpers.RunCommand
- Maintain exact same public interface and behavior
- Helm binary availability handled by existing materializer functionality

### Key Components

#### 1. binaryExecutor Interface (Mockable)
```go
type binaryExecutor interface {
    // ExecuteCommand runs a command and returns stdout, stderr, and error
    ExecuteCommand(ctx context.Context, env map[string]string, bin string, args ...string) (stdout string, stderr string, err error)
}

type commandExecutor struct{}

func (c *commandExecutor) ExecuteCommand(ctx context.Context, env map[string]string, bin string, args ...string) (string, string, error) {
    var stdoutBuf, stderrBuf bytes.Buffer

    err := helpers.RunCommandWithOptions(helpers.RunCommandOptions{
        Context: ctx,
        Stdout:  &stdoutBuf,
        Stderr:  &stderrBuf,
        Env:     env,
    }, bin, args...)

    return stdoutBuf.String(), stderrBuf.String(), err
}
```

#### 2. HelmClient Structure (Refactored)
```go
type HelmClient struct {
    helmPath         string                                 // Path to helm binary
    executor         binaryExecutor                         // Mockable executor
    tmpdir           string                                 // Temporary directory for helm
    kversion         *semver.Version                        // Kubernetes version
    restClientGetter genericclioptions.RESTClientGetter     // REST client getter
    registryConfig   string                                 // Registry config path for OCI
    repositories     []*repo.Entry                          // Repository entries
    logFn            action.DebugLog                        // Debug logging function
    airgapPath       string                                 // Airgap path where charts are stored
}
```

## New Subagents / Commands

**No new subagents or commands will be created.** This proposal only changes the internal implementation of the existing Helm client.

## Database

**No database changes required.** This proposal only affects in-memory operations and command execution.

## Implementation plan

### Files to Create/Modify

#### New Files:
- `pkg/helm/binary_executor.go` - Executor interface and implementation (~100 lines)
- `pkg/helm/binary_executor_mock.go` - Generated mock for testing (~50 lines)
- `pkg/helm/output_parser.go` - Parse helm command outputs (~300 lines)
- `pkg/helm/output_parser_test.go` - Parser tests (~200 lines)

#### Modified Files:
- `pkg/helm/client.go` - Complete refactor from SDK to binary execution (~800 lines, replacing 613 existing)
- `pkg/helm/client_test.go` - Update tests to use mock executor (~300 lines modified)
- `pkg/helm/values_test.go` - Update for binary client (~50 lines modified)
- `pkg/helm/interface.go` - No changes (same interface)

#### Files Using Helm Client (No Changes Required):
- **70+ files** across codebase continue to work unchanged
- All addons, API managers, CLI commands, extensions maintain compatibility

### Function to Binary Command Mapping

| SDK Function | Helm Binary Command | Options Preserved | Output Parsing Required |
|--------------|-------------------|-------------------|------------------------|
| `Install()` | `helm install` | ✓ All | Release JSON |
| `Upgrade()` | `helm upgrade` | ✓ All including `--force` | Release JSON |
| `Uninstall()` | `helm uninstall` | ✓ `--wait`, `--no-hooks` | Success message |
| `ReleaseExists()` | `helm list` | `--namespace`, `--filter` | JSON list |
| `Render()` | `helm template` | ✓ All options | YAML manifests |
| `Pull()` | `helm pull` | `--version`, `--repo` | File path |
| `PullByRef()` | `helm pull` | `--version` for OCI | File path |
| `Push()` | `helm push` | OCI destination | Success message |
| `RegistryAuth()` | `helm registry login` | `--username`, `--password` | Success message |
| `AddRepo()` | `helm repo add` | `--force-update`, auth | Success message |
| `Latest()` | `helm search repo` | `--version ">0.0.0"` | Version string |
| `GetChartMetadata()` | `helm show chart` | Chart path | Chart.yaml parsing |

### Detailed Option Preservation

#### Install Options
```bash
helm install [NAME] [CHART] \
  --namespace <namespace> \
  --create-namespace \
  --wait \
  --wait-for-jobs \
  --timeout <duration> \
  --values <file> \
  --set key=value \
  --atomic=false \  # Explicitly false for install
  --replace \
  --output json
```

#### Upgrade Options
```bash
helm upgrade [NAME] [CHART] \
  --namespace <namespace> \
  --wait \
  --wait-for-jobs \
  --timeout <duration> \
  --values <file> \
  --set key=value \
  --atomic \
  --force \  # Critical: User noticed this was missing
  --output json
```

#### Uninstall Options
```bash
helm uninstall [NAME] \
  --namespace <namespace> \
  --wait \
  --timeout <duration> \
  --ignore-not-found
```

### Implementation

```go
// Example: Install implementation
func (c *HelmClient) Install(ctx context.Context, opts InstallOptions) (*release.Release, error) {
    args := []string{"install", opts.ReleaseName}

    // Handle chart source
    if c.airgapPath != "" {
        // Use chart from airgap path
    } else if !strings.HasPrefix(opts.ChartPath, "/") {
        // Pull chart with retries (includes oci:// prefix)
    } else {
        // Use local chart path
    }
    
    // Add all helm install flags: --namespace, --create-namespace, --wait, etc.
    // Add values file if provided
    // Add labels if provided
    
    // Execute helm command
    stdout, stderr, err := c.executor.ExecuteCommand(ctx, nil, c.helmPath, args...)
    
    // Parse release from JSON output
    return &release, nil
}

// Example: ReleaseExists implementation  
func (c *HelmClient) ReleaseExists(ctx context.Context, namespace, name string) (bool, error) {
    // Build: helm list --namespace X --filter "^name$" --output json
    // Execute command and parse JSON list
    // Check if release exists and is not uninstalled
    return exists, nil
}
```

### External Contracts

No changes to external APIs. The binary implementation maintains exact compatibility with existing interface.

## Testing

### Unit Tests
```go
// Using mockery-generated mock
func TestHelmClient_Install(t *testing.T) {
    mockExec := new(MockBinaryExecutor)
    client := &HelmClient{
        helmPath: "/usr/local/bin/helm",
        executor: mockExec,
    }
    
    mockExec.On("ExecuteCommand", 
        mock.Anything,  // context
        mock.Anything,  // env
        "/usr/local/bin/helm",
        "install", "myrelease", "/path/to/chart",
        "--namespace", "default",
        "--create-namespace",
        "--wait",
        "--wait-for-jobs",
        "--timeout", "5m0s",
        "--replace",
        "--output", "json",
    ).Return(testReleaseJSON, "", nil)
    
    release, err := client.Install(context.Background(), InstallOptions{
        ReleaseName: "myrelease",
        ChartPath:   "/path/to/chart",
        Namespace:   "default",
        Timeout:     5 * time.Minute,
    })
    
    require.NoError(t, err)
    assert.Equal(t, "myrelease", release.Name)
    mockExec.AssertExpectations(t)
}
```

### Integration Tests
- Execution with SDK and binary implementations
- Output comparison for all operations
- Airgap mode testing

### Test Data and Fixtures
- Sample chart archives
- Mock release JSON outputs
- Error response samples
- Repository index files

## Backward compatibility

### Full API Compatibility
- Exact same Client interface maintained
- All return types preserved
- No changes to function signatures

### Data Format Compatibility
- JSON output parsing for structured data
- YAML manifest compatibility for Render()
- Repository cache format unchanged

## Migrations

**Helm binary must be embedded in installer.** The existing materializer functionality in `cmd/installer/goods/materializer.go` will handle helm binary availability similar to other binaries.

### Required Changes:
1. **Embed helm binary** in the embedded-cluster installer binary
2. **Materialize helm binary** during installation to same directory we materialize other embedded binaries
3. **Enable binary client** for all installs (v2 and v3)
4. **Maintain exact same interface** for all consuming code

### Implementation:
- Verify helm binary is materialized during installation
- Replace all SDK calls with `helpers.RunCommand` execution
- Parse command outputs to maintain existing return types

## Trade-offs

### Optimizing For:
- **Maintainability**: Simpler codebase without SDK dependencies
- **Compatibility**: Guaranteed parity with helm CLI behavior
- **Debuggability**: Clear command output in logs

## Alternative solutions considered

### 1. Upgrade Helm SDK to Latest Version
- **Rejected**: Continues maintenance burden, doesn't solve core issues
- **Risk**: Breaking changes in SDK API

### 2. Fork Helm SDK
- **Rejected**: Massive maintenance burden
- **Risk**: Divergence from upstream

### 4. Hybrid Approach (SDK for some, binary for others)
- **Rejected**: Would require maintaining both SDK and binary implementations
- **Complexity**: 
  - Need to carefully track which functions use which implementation
  - More complex testing matrix to validate both paths
  - Increased cognitive load for developers to remember which path to use
  - Potential for subtle bugs when functions interact across implementations

## Research

### Prior Art in Codebase
- [Helm Binary Migration Research](./helm_binary_migration_research.md)
- `pkg/helpers/RunCommand` - Established pattern for command execution
- `pkg/helpers/firewalld/client.go` - Example of binary wrapper pattern
- Mock patterns in `pkg/helpers/mock.go`

### External References
- [Helm CLI Documentation](https://helm.sh/docs/helm/)
- [Kubernetes SIG-Apps Helm discussions](https://github.com/kubernetes/community/tree/master/sig-apps)
- [ArgoCD Helm Binary Integration](https://github.com/argoproj/argo-cd/tree/master/util/helm)
- [Flux Helm Controller](https://github.com/fluxcd/helm-controller) - Uses helm SDK but considering binary

### Prototypes and Learnings
- Spike: JSON output parsing - All commands support --output json
- Spike: Concurrent execution - No file lock issues with separate processes
- Test: Repository cache compatibility verified between SDK and binary

## Checkpoints (PR plan)

### PR 1: Foundation & Utilities
- `pkg/helm/binary_executor.go` - Interface and implementation
- Generate `pkg/helm/binary_executor_mock.go` using github.com/stretchr/testify/mock
- `pkg/helm/output_parser.go` - Parse JSON and YAML outputs from helm commands
- Unit tests for executor and parser components

### PR 2: Client Refactor
- Complete refactor of `pkg/helm/client.go` - replace SDK with binary execution
- All 13 interface methods implemented with binary commands
- Comprehensive error handling with stdout/stderr capture and logging
- Update `pkg/helm/client_test.go` to use mock executor
- Update `pkg/helm/values_test.go` for binary client
- Remove unused Helm Go SDK imports and dependencies

Each PR will include:
- Complete implementation for its scope
- Unit and integration tests
