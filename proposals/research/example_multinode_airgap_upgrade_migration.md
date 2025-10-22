# Example: Migrating TestMultiNodeAirgapUpgrade to Integration Tests

## Current E2E Test Overview

`TestMultiNodeAirgapUpgrade` (e2e/install_test.go:1226) currently does:
1. Spins up 2 CMX VMs
2. Downloads airgap bundles for old and new versions
3. Airgaps the cluster (cuts off internet)
4. Installs old EC version with k0s + all addons + extensions + app
5. Joins a worker node
6. Runs airgap update to load new bundle
7. Triggers upgrade via UI (entire upgrade flow: k0s autopilot, addons, extensions, app)
8. Validates post-upgrade state

**Total runtime:** 40-60 minutes
**Failure mode:** When it fails, could be any of the 8+ phases

## Breaking Down Into Integration Tests

We'll create 4 focused integration tests that mirror the actual upgrade phases. All tests use multi-node clusters and are labeled as airgap tests with internet access disabled.

### 1. Airgap Artifact Distribution Test
**Location:** `tests/integration/k0s/airgap_artifact_distribution_test.go`

**What it tests:** Distribution of airgap upgrade artifacts from Registry to LAM to all nodes

**Implementation details:**
- **Test type:** Integration test with real k0s
- **Cluster:** 3-node k0s cluster bootstrapped via k0s CLI
- **Mocks/fixtures:**
  - Pre-loaded test artifacts (k0s binaries and images) in Registry addon (in-cluster registry)
  - Use actual `local-artifact-mirror serve` binary (starts empty)
- **Setup:**
  - Bootstrap 3-node k0s cluster with old k0s version using k0s CLI
  - Install Registry addon and push test upgrade artifacts to it
  - Start `local-artifact-mirror serve` with empty data directory
  - Disable internet access (network policies or iptables)
- **Test:**
  - Call `upgrader.DistributeArtifacts()` from `pkg-new/upgrade/upgrader.go`
  - This creates Jobs that:
    1. Pull artifacts from in-cluster Registry
    2. Copy them to LAM's data directory on each node
    3. Make them available for k0s via LAM HTTP server
- **Assertions:**
  - Artifact distribution Jobs complete successfully on all nodes
  - Artifacts are copied to LAM data directory (`/var/lib/embedded-cluster/bin`, `/var/lib/embedded-cluster/images`, etc.)
  - K0s images are loaded into the cluster
  - No external network calls
- **Runtime:** 5-8 minutes

**Key insight:** This tests the artifact distribution flow: Registry → Jobs → LAM data directory → LAM HTTP server. Uses the actual `local-artifact-mirror serve` binary and actual distribution Jobs from production code.

### 2. Addons Airgap Upgrade Test
**Location:** `tests/integration/kind/addons/airgap_upgrade_test.go`

**What it tests:** Addon upgrades (registry, seaweedfs, ECO, admin console, velero, openebs) in airgap mode

**Implementation details:**
- **Test type:** Integration test with multi-node kind cluster
- **Cluster:** 3-node kind cluster (matches multi-node nature of original test)
- **Mocks/fixtures:**
  - Pre-loaded old addon images in kind
  - Pre-loaded new addon images in kind (simulates airgap)
  - No LAM needed
- **Setup:**
  - Create 3-node kind cluster
  - Load old versions of addon images into kind
  - Install old addons (registry, seaweedfs, ECO, admin console, velero, openebs)
  - Load new versions of addon images into kind
  - Apply network policies to block external access
  - Upgrade addons
- **Test:**
  - Call `upgrader.UpgradeAddons()` from `pkg-new/upgrade/upgrader.go`
  - This single call upgrades addons in sequence
  - Verify each addon upgrades successfully
- **Assertions:**
  - Addon versions match expected new versions
  - Addon pods restart and become ready
  - Data persistence verified for stateful addons (registry, seaweedfs)
  - No external network calls
- **Runtime:** 8-12 minutes

**Leverage existing infrastructure:**
- Use `tests/integration/kind/registry/ha_test.go` as template for multi-addon setup
- Reuse `tests/integration/util/kind.go` helpers

### 3. Extensions Airgap Upgrade Test
**Location:** `tests/integration/kind/extensions/airgap_upgrade_test.go`

**What it tests:** Extension upgrades in airgap mode

**Implementation details:**
- **Test type:** Integration test with multi-node kind cluster
- **Cluster:** 2-node kind cluster
- **Mocks/fixtures:**
  - Pre-loaded old extension images and Helm charts in kind
  - Pre-loaded new extension images and charts in kind
  - Test extensions (minimal but representative)
- **Setup:**
  - Create 2-node kind cluster
  - Load old extension images into kind
  - Install old versions of test extensions via Helm
  - Load new extension images and charts into kind
  - Apply network policies to block external access
  - Upgrade extensions
- **Test:**
  - Call `upgrader.UpgradeExtensions()` from `pkg-new/upgrade/upgrader.go`
  - This single call upgrades extensions
  - Verify Helm releases updated
- **Assertions:**
  - Extension Helm release versions updated
  - Extension pods running with new versions
  - Extensions functional post-upgrade
  - No external calls
- **Runtime:** 4-6 minutes

**Key insight:** Uses `upgrader.UpgradeExtensions()` to test the actual upgrade orchestration logic.

**New infrastructure needed:**
- Create `tests/integration/kind/extensions/` directory
- Add extension management utilities
- Create test extension fixtures (minimal Helm charts with version bumps)

### 4. K0s Autopilot Multi-Node Airgap Upgrade Test
**Location:** `tests/integration/k0s/autopilot_airgap_upgrade_test.go`

**What it tests:** K0s autopilot upgrade across multiple nodes in airgap mode

**Implementation details:**
- **Test type:** Integration test with real k0s
- **Cluster:** 3-node k0s cluster
- **Mocks/fixtures:**
  - Pre-loaded new k0s binary in LAM data directory
  - Pre-loaded new k0s images in LAM data directory
  - Use actual `local-artifact-mirror serve` binary
- **Setup:**
  - Bootstrap 3-node airgap k0s cluster with old version using k0s CLI
  - Start `local-artifact-mirror serve` with new k0s binary and images pre-loaded
  - Disable internet access (iptables rules)
- **Test:**
  - Call `upgrader.UpgradeK0s()` from `pkg-new/upgrade/upgrader.go`
  - This creates the autopilot Plan CR and monitors it
  - Wait for autopilot to upgrade the nodes
  - Verify upgrade orchestration
- **Assertions:**
  - Autopilot Plan reaches `Completed` state
  - Nodes upgraded to new k0s version (check via `kubectl get nodes`)
  - Cluster remains functional
  - No external calls
- **Runtime:** 10-15 minutes (k0s upgrades are inherently slow)

**Key insight:** Uses `upgrader.UpgradeK0s()` which encapsulates the autopilot plan creation and monitoring logic. The LAM file server (`local-artifact-mirror serve`) is needed here because k0s autopilot fetches binaries and images during upgrade.

**Note:** This is the slowest test because k0s upgrades can't be rushed. However, it's still 3-4x faster than the full E2E test.

**New infrastructure needed:**
- Create `tests/integration/k0s/` directory
- Add k0s cluster management utilities
- Add LAM setup utilities

## Key Implementation Details

### Using the Upgrader Interface

All tests leverage the `InfraUpgrader` interface from `pkg-new/upgrade/upgrader.go`:

```go
type InfraUpgrader interface {
    UpgradeK0s() error
    UpgradeAddons() error
    UpgradeExtensions() error
    DistributeArtifacts() error
    // ... other methods
}
```

This ensures we're testing the **actual production upgrade code paths**, not reimplementing upgrade logic in tests.

**Test 1 (Artifact Distribution):** Calls `upgrader.DistributeArtifacts()` which creates Jobs on all nodes to pull and cache images locally.

**Test 2 (Addons):** Calls `upgrader.UpgradeAddons()` which orchestrates upgrading all addons (registry, seaweedfs, ECO, admin console, velero, openebs) in the correct order with proper dependencies.

**Test 3 (Extensions):** Calls `upgrader.UpgradeExtensions()` which handles Helm-based extension upgrades.

**Test 4 (K0s Autopilot):** Calls `upgrader.UpgradeK0s()` which creates the autopilot Plan CR and monitors it to completion.

### Airgap Strategy

All 4 tests simulate airgap environments but with different approaches:

**K0s tests (artifact distribution, autopilot):**
- Use actual `local-artifact-mirror serve` binary as file server
- Pre-load test artifacts (k0s binaries, images) into LAM's data directory
- LAM serves files over HTTP to simulate airgap upgrade bundle being available
- Internet blocked via iptables rules or network policies

**Kind tests (addons, extensions):**
- Use `kind load docker-image` to pre-load old and new images into kind nodes
- This simulates having an airgap bundle extracted and loaded
- No LAM needed - images are already on nodes
- Internet blocked via Kubernetes NetworkPolicy resources

### Multi-Node

Since we're migrating `TestMultiNodeAirgapUpgrade`, all integration tests use multi-node setups:
- **Artifact distribution test:** 3-node k0s cluster (tests cross-node artifact distribution)
- **Addons test:** 3-node kind cluster (tests multi-node addon behavior, e.g., registry HA, seaweedfs scaling)
- **Extensions test:** 2-node kind cluster (tests extension deployment across nodes)
- **K0s autopilot test:** 3-node k0s cluster (tests autopilot coordinating upgrades across all nodes)

This ensures we catch multi-node specific issues:
- Cross-node network policies
- Node-to-node communication
- Autopilot orchestration across multiple nodes

## What We're NOT Testing in Integration Tests

These remain as 1-2 smoke tests in E2E:

1. **Full end-to-end flow:** The complete user journey from install through upgrade
2. **UI interactions:** Playwright tests for clicking "Deploy" button and UI validation
3. **Cross-component integration:** How all phases work together in sequence
4. **Initial installation:** Integration tests assume a cluster already exists

## Directory Structure

```
tests/integration/
├── k0s/
│   ├── airgap_artifact_distribution_test.go
│   ├── autopilot_airgap_upgrade_test.go
│   ├── util.go (k0s cluster management utilities)
│   └── lam.go (LAM setup for k0s tests)
├── kind/
│   ├── addons/
│   │   └── airgap_upgrade_test.go (all addons in one test)
│   ├── extensions/
│   │   └── airgap_upgrade_test.go
│   └── (existing subdirectories)
└── util/
    ├── airgap.go (airgap test utilities - network policies, image pre-loading)
    └── (existing files)
```

## Fixtures and Test Data

### Docker Images
- **Location:** loaded into the registry or kind at runtime
- **Contents:**
  - Old versions: registry:v2.0.0-old, seaweedfs:v3.0.0-old, eco:v1.0.0-old, etc.
  - New versions: registry:v2.1.0-new, seaweedfs:v3.1.0-new, eco:v1.1.0-new, etc.
  - K0s images for autopilot test
- **Usage:**
  - Kind tests: Use `kind load docker-image` to pre-load into nodes
  - K0s tests: Copy to LAM's data directory (e.g., `/var/lib/embedded-cluster/images/`) for LAM to serve

### K0s Binaries
- **Location:** downloaded at runtime
- **Usage:** Downloaded at runtime, used to bootstrap k0s clusters

### LAM Implementation (K0s tests only)
- **Approach:** Use the existing `cmd/local-artifact-mirror` binary as a file server
- **Pre-loaded artifacts:** Copy k0s binaries and images to LAM's data directory during test setup

**Original E2E test runtime:** 40-60 minutes

**Key differences:**
1. **Parallelization:** All integration tests can run concurrently
2. **Isolation:** Each test is independent
3. **Failure precision:** Know exactly which component failed
4. **Faster iteration:** Developers can run specific tests (e.g., just registry upgrade)


## Example: Addons Upgrade Test Implementation

Here's what the addons airgap upgrade test would look like conceptually:

```go
// tests/integration/kind/addons/airgap_upgrade_test.go

func TestAddons_AirgapUpgrade(t *testing.T) {
    ctx := context.Background()

    // 1. Setup 3-node kind cluster
    clusterName := util.GenerateClusterName(t)
    kubeconfig := util.SetupKindCluster(t, clusterName, &util.KindClusterOptions{
        NumControlPlaneNodes: 3,
    })

    kcli := util.CtrlClient(t, kubeconfig)
    mcli := util.MetadataClient(t, kubeconfig)
    hcli := util.HelmClient(t, kubeconfig)

    // 2. Load old addon images into kind (simulates airgap bundle loaded)
    loadImages(t, clusterName, []string{
        "registry:v2.0.0-old",
        "seaweedfs:v3.0.0-old",
        "eco:v1.0.0-old",
        "kotsadm:v1.0.0-old",
        "velero:v1.0.0-old",
        "openebs:v3.0.0-old",
    })

    // 3. Install old versions of ALL addons
    installAllAddonsOldVersion(t, kcli, mcli, hcli)

    // 4. Load new addon images into kind (simulates upgrade bundle loaded)
    loadImages(t, clusterName, []string{
        "registry:v2.1.0-new",
        "seaweedfs:v3.1.0-new",
        "eco:v1.1.0-new",
        "kotsadm:v1.1.0-new",
        "velero:v1.1.0-new",
        "openebs:v3.1.0-new",
    })

    // 5. Apply network policies to block external access
    util.ApplyAirgapNetworkPolicies(t, kubeconfig)

    // 6. Create upgrader and trigger upgrade
    upgrader := upgrade.NewInfraUpgrader(kcli, mcli, hcli, "v1.1.0-new")

    // THIS IS THE KEY - uses actual production upgrade orchestration
    err := upgrader.UpgradeAddons(ctx)
    require.NoError(t, err)

    // 7. Verify all addons upgraded
    assertAddonVersion(t, kubeconfig, "registry", "v2.1.0-new")
    assertAddonVersion(t, kubeconfig, "seaweedfs", "v3.1.0-new")
    assertAddonVersion(t, kubeconfig, "eco", "v1.1.0-new")
    assertAddonVersion(t, kubeconfig, "admin-console", "v1.1.0-new")
    assertAddonVersion(t, kubeconfig, "velero", "v1.1.0-new")
    assertAddonVersion(t, kubeconfig, "openebs", "v3.1.0-new")

    // 8. Verify no external calls
    assertNoExternalNetworkCalls(t, kubeconfig)

    // 9. Verify data persistence for stateful addons
    assertRegistryImagesAccessible(t, kubeconfig)
    assertSeaweedFSDataAccessible(t, kubeconfig)
}
```

**Key implementation details:**
- `loadImages`: Uses `kind load docker-image` to preload images into kind nodes
- `installAllAddonsOldVersion`: Calls addon `.Install()` methods for each addon with old versions
- `util.ApplyAirgapNetworkPolicies`: Creates NetworkPolicy resources to block external egress
- `upgrade.NewInfraUpgrader`: Creates the actual upgrader from `pkg-new/upgrade/upgrader.go`
- `upgrader.UpgradeAddons()`: **This is the production code path** - upgrades all addons in the correct order
- `assertAddonVersion`: Checks Helm release versions or pod image tags
- `assertNoExternalNetworkCalls`: Checks that no pods attempted external connections (can inspect network policy counters or pod logs)

This approach tests **real upgrade orchestration logic** with **all real addons** but in a **controlled, fast environment**. The upgrade ordering, dependencies, and interaction between addons are all tested because we use the actual `upgrader.UpgradeAddons()` method.

## Example: K0s Autopilot Multi-Node Airgap Upgrade Test Implementation

Here's what the k0s autopilot airgap upgrade test would look like conceptually:

```go
// tests/integration/k0s/autopilot_airgap_upgrade_test.go

func TestK0s_AutopilotAirgapUpgradeMultiNode(t *testing.T) {
    ctx := context.Background()

    // Setup 3-node k0s cluster with old version
    oldK0sVersion := "v1.28.0+k0s.0"
    newK0sVersion := "v1.29.0+k0s.0"

    clusterNodes := []string{"node1", "node2", "node3"}

    // Bootstrap k0s cluster using k0s CLI directly (not our installer)
    // This creates controller nodes without going through our install flow
    t.Logf("Bootstrapping 3-node k0s cluster with version %s", oldK0sVersion)
    k0sCluster := bootstrapK0sCluster(t, clusterNodes, oldK0sVersion)
    defer k0sCluster.Cleanup()

    kubeconfig := k0sCluster.GetKubeconfig()
    kcli := util.CtrlClient(t, kubeconfig)
    mcli := util.MetadataClient(t, kubeconfig)
    hcli := util.HelmClient(t, kubeconfig)

    // Verify cluster is running old k0s version
    assertK0sVersion(t, k0sCluster, oldK0sVersion)

    // Setup LAM with new k0s artifacts
    t.Logf("Setting up LAM with k0s %s artifacts", newK0sVersion)
    lamDataDir := t.TempDir()

    // Download new k0s binary and put in the LAM data directory
    prepareK0sBinaryInLAM(t, lamDataDir, newK0sVersion)

    // Start local-artifact-mirror serve
    lamURL := startLAM(t, lamDataDir)
    t.Logf("LAM serving at %s", lamURL)

    // Block internet access (simulate airgap)
    t.Logf("Blocking internet access")
    blockInternetAccess(t, k0sCluster)

    // Create Installation CR pointing to LAM
    t.Logf("Creating Installation CR")
    installation := &ecv1beta1.Installation{
        ObjectMeta: metav1.ObjectMeta{
            Name: "test-installation",
        },
        Spec: ecv1beta1.InstallationSpec{
            Config: &ecv1beta1.ConfigSpec{
                Version: newK0sVersion,
            },
            AirGap: true,
            LocalArtifactMirror: ecv1beta1.LocalArtifactMirror{
                Address: lamURL,
            },
        },
    }
    err := kcli.Create(ctx, installation)
    require.NoError(t, err)

    // Create upgrader and trigger k0s upgrade
    t.Logf("Creating upgrader")
    upgrader := upgrade.NewInfraUpgrader(kcli, mcli, hcli, newK0sVersion)

    // THIS IS THE KEY - uses actual production k0s autopilot upgrade logic
    t.Logf("Triggering k0s autopilot upgrade")
    err = upgrader.UpgradeK0s(ctx)
    require.NoError(t, err)

    // Verify all nodes upgraded to new version
    assertK0sVersion(t, k0sCluster, newK0sVersion)
    assertAllNodesReady(t, kubeconfig, 3)

    // Verify no external network calls
    assertNoExternalNetworkCalls(t, k0sCluster)
}
```

**Key implementation details:**

### bootstrapK0sCluster Implementation

```go
// tests/integration/k0s/cluster.go

import (
    "fmt"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/ory/dockertest/v3"
    "github.com/ory/dockertest/v3/docker"
)

type K0sCluster struct {
    nodes      []*K0sNode
    kubeconfig string
    pool       *dockertest.Pool
}

type K0sNode struct {
    name      string
    container *dockertest.Resource
    volume    string
}

func bootstrapK0sCluster(t *testing.T, nodeNames []string, k0sVersion string) *K0sCluster {
    pool, err := dockertest.NewPool("")
    require.NoError(t, err)

    cluster := &K0sCluster{
        nodes: make([]*K0sNode, len(nodeNames)),
        pool:  pool,
    }

    // Create VMs for all nodes
    for i, name := range nodeNames {
        t.Logf("Creating VM for node %s", name)
        node := createK0sVM(t, pool, name)
        cluster.nodes[i] = node
    }

    // Bootstrap first node as controller
    t.Logf("Installing k0s %s on %s", k0sVersion, nodeNames[0])
    installK0sController(t, cluster.nodes[0], k0sVersion)

    // Get kubeconfig from first node
    cluster.kubeconfig = getKubeconfig(t, cluster.nodes[0])

    // Generate join token
    joinToken := generateJoinToken(t, cluster.nodes[0])

    // Join remaining nodes
    for i := 1; i < len(cluster.nodes); i++ {
        t.Logf("Joining node %s to cluster", nodeNames[i])
        joinK0sController(t, cluster.nodes[i], k0sVersion, joinToken)
    }

    // Wait for all nodes to be ready
    waitForNodesReady(t, cluster, len(nodeNames))

    return cluster
}

func createK0sVM(t *testing.T, pool *dockertest.Pool, name string) *K0sNode {
    // Get distro image from env or use default
    distro := os.Getenv("EC_TEST_DISTRO")
    if distro == "" {
        distro = "debian-bookworm"
    }
    image := fmt.Sprintf("replicated/ec-distro:%s", distro)

    // Create volume for persistent storage
    volumeName := fmt.Sprintf("k0s-test-%s-%d", name, time.Now().Unix())

    // Run container with systemd
    container, err := pool.RunWithOptions(&dockertest.RunOptions{
        Name:       name,
        Repository: "replicated/ec-distro",
        Tag:        distro,
        Mounts: []string{
            fmt.Sprintf("%s:/var/lib/embedded-cluster", volumeName),
        },
        Privileged: true,
    }, func(config *docker.HostConfig) {
        config.RestartPolicy = docker.RestartPolicy{
            Name: "unless-stopped",
        }
    })
    require.NoError(t, err)

    // Wait for systemd to be ready
    err = pool.Retry(func() error {
        code, err := container.Exec([]string{"systemctl", "status"}, dockertest.ExecOptions{})
        if err != nil {
            return err
        }
        if code != 0 {
            return fmt.Errorf("systemd not ready")
        }
        return nil
    })
    require.NoError(t, err)

    return &K0sNode{
        name:      name,
        container: container,
        volume:    volumeName,
    }
}

func installK0sController(t *testing.T, node *K0sNode, version string) {
    // Download k0s binary
    downloadCmd := fmt.Sprintf(
        "curl -sSLf https://github.com/k0sproject/k0s/releases/download/%s/k0s -o /usr/local/bin/k0s && chmod +x /usr/local/bin/k0s",
        version,
    )
    execCommand(t, node, []string{"sh", "-c", downloadCmd})

    // Install k0s as controller
    execCommand(t, node, []string{"k0s", "install", "controller", "--enable-worker"})

    // Start k0s
    execCommand(t, node, []string{"systemctl", "start", "k0scontroller"})

    // Wait for k0s to be ready
    waitForK0sReady(t, node)
}

func generateJoinToken(t *testing.T, node *K0sNode) string {
    output := execCommandWithOutput(t, node, []string{"k0s", "token", "create", "--role=controller"})
    return strings.TrimSpace(output)
}

func joinK0sController(t *testing.T, node *K0sNode, version string, token string) {
    // Download k0s binary
    downloadCmd := fmt.Sprintf(
        "curl -sSLf https://github.com/k0sproject/k0s/releases/download/%s/k0s -o /usr/local/bin/k0s && chmod +x /usr/local/bin/k0s",
        version,
    )
    execCommand(t, node, []string{"sh", "-c", downloadCmd})

    // Write token to file
    tokenPath := "/tmp/k0s-token"
    execCommand(t, node, []string{"sh", "-c", fmt.Sprintf("echo '%s' > %s", token, tokenPath)})

    // Install k0s as controller with token
    execCommand(t, node, []string{"k0s", "install", "controller", "--enable-worker", "--token-file", tokenPath})

    // Start k0s
    execCommand(t, node, []string{"systemctl", "start", "k0scontroller"})

    // Wait for k0s to be ready
    waitForK0sReady(t, node)
}

func execCommand(t *testing.T, node *K0sNode, cmd []string) {
    code, err := node.container.Exec(cmd, dockertest.ExecOptions{})
    require.NoError(t, err)
    require.Equal(t, 0, code, "Command failed: %v", cmd)
}

func execCommandWithOutput(t *testing.T, node *K0sNode, cmd []string) string {
    var stdout bytes.Buffer
    code, err := node.container.Exec(cmd, dockertest.ExecOptions{
        StdOut: &stdout,
    })
    require.NoError(t, err)
    require.Equal(t, 0, code, "Command failed: %v", cmd)
    return stdout.String()
}

func getKubeconfig(t *testing.T, node *K0sNode) string {
    output := execCommandWithOutput(t, node, []string{"k0s", "kubeconfig", "admin"})

    // Write kubeconfig to temp file
    kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
    err := os.WriteFile(kubeconfigPath, []byte(output), 0600)
    require.NoError(t, err)

    return kubeconfigPath
}

func waitForK0sReady(t *testing.T, node *K0sNode) {
    for i := 0; i < 60; i++ {
        code, _ := node.container.Exec([]string{"k0s", "kubectl", "get", "nodes"}, dockertest.ExecOptions{})
        if code == 0 {
            return
        }
        time.Sleep(2 * time.Second)
    }
    t.Fatal("k0s failed to become ready")
}

func waitForNodesReady(t *testing.T, cluster *K0sCluster, expectedNodes int) {
    for i := 0; i < 60; i++ {
        code, err := cluster.nodes[0].container.Exec(
            []string{"k0s", "kubectl", "get", "nodes", "--no-headers"},
            dockertest.ExecOptions{},
        )
        if err == nil && code == 0 {
            // Count ready nodes
            var stdout bytes.Buffer
            cluster.nodes[0].container.Exec(
                []string{"k0s", "kubectl", "get", "nodes", "--no-headers"},
                dockertest.ExecOptions{StdOut: &stdout},
            )
            lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
            if len(lines) == expectedNodes {
                return
            }
        }
        time.Sleep(2 * time.Second)
    }
    t.Fatalf("Expected %d nodes to be ready", expectedNodes)
}

func (c *K0sCluster) Cleanup() {
    for _, node := range c.nodes {
        c.pool.Purge(node.container)
        // Remove volume
        c.pool.Client.RemoveVolume(node.volume)
    }
}

func (c *K0sCluster) GetKubeconfig() string {
    return c.kubeconfig
}
```

### Other Helper Functions

```go
// tests/integration/k0s/lam.go

func prepareK0sBinaryInLAM(t *testing.T, lamDataDir string, k0sVersion string) {
    // Create bin directory
    binDir := filepath.Join(lamDataDir, "bin")
    err := os.MkdirAll(binDir, 0755)
    require.NoError(t, err)

    // Download k0s binary
    k0sBinaryURL := fmt.Sprintf(
        "https://github.com/k0sproject/k0s/releases/download/%s/k0s",
        k0sVersion,
    )

    resp, err := http.Get(k0sBinaryURL)
    require.NoError(t, err)
    defer resp.Body.Close()

    // Write to LAM directory
    k0sPath := filepath.Join(binDir, "k0s")
    f, err := os.Create(k0sPath)
    require.NoError(t, err)
    defer f.Close()

    _, err = io.Copy(f, resp.Body)
    require.NoError(t, err)

    // Make executable
    err = os.Chmod(k0sPath, 0755)
    require.NoError(t, err)

    t.Logf("Downloaded k0s %s to %s", k0sVersion, k0sPath)
}

func startLAM(t *testing.T, dataDir string) string {
    // Find local-artifact-mirror binary
    lamBinary := os.Getenv("LAM_BINARY_PATH")
    if lamBinary == "" {
        // Default to built binary
        lamBinary = "../../../output/bin/local-artifact-mirror"
    }

    // Start LAM server
    port := 50000
    cmd := exec.Command(lamBinary, "serve", "--data-dir", dataDir, "--port", fmt.Sprintf("%d", port))

    err := cmd.Start()
    require.NoError(t, err)

    // Stop LAM on cleanup
    t.Cleanup(func() {
        if cmd.Process != nil {
            cmd.Process.Kill()
        }
    })

    // Wait for LAM to be ready
    lamURL := fmt.Sprintf("http://127.0.0.1:%d", port)
    for i := 0; i < 30; i++ {
        resp, err := http.Get(lamURL)
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            t.Logf("LAM ready at %s", lamURL)
            return lamURL
        }
        time.Sleep(1 * time.Second)
    }

    t.Fatal("LAM failed to start")
    return ""
}

func blockInternetAccess(t *testing.T, cluster *K0sCluster) {
    for _, node := range cluster.nodes {
        // Block all outbound traffic except:
        // 1. Cluster-internal IPs (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
        // 2. Localhost (127.0.0.0/8)
        // 3. Container network (Docker bridge network)

        rules := []string{
            // Allow localhost
            "iptables -A OUTPUT -d 127.0.0.0/8 -j ACCEPT",
            // Allow private networks (cluster-internal communication)
            "iptables -A OUTPUT -d 10.0.0.0/8 -j ACCEPT",
            "iptables -A OUTPUT -d 172.16.0.0/12 -j ACCEPT",
            "iptables -A OUTPUT -d 192.168.0.0/16 -j ACCEPT",
            // Allow established connections
            "iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT",
            // Drop everything else (blocks internet)
            "iptables -A OUTPUT -j DROP",
        }

        for _, rule := range rules {
            cmd := []string{"sh", "-c", rule}
            // Best effort - don't fail if iptables rules fail
            node.container.Exec(cmd, dockertest.ExecOptions{})
        }
        t.Logf("Blocked internet access on node %s (allowed cluster-internal only)", node.name)
    }
}
```

```go
// tests/integration/k0s/assertions.go

func assertK0sVersion(t *testing.T, cluster *K0sCluster, expectedVersion string) {
    for _, node := range cluster.nodes {
        output := execCommandWithOutput(t, node, []string{"k0s", "version"})
        require.Contains(t, output, expectedVersion, "Node %s has wrong k0s version", node.name)
    }
    t.Logf("All nodes running k0s %s", expectedVersion)
}

func assertAllNodesReady(t *testing.T, kubeconfig string, expectedCount int) {
    cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "get", "nodes", "--no-headers")
    output, err := cmd.CombinedOutput()
    require.NoError(t, err)

    lines := strings.Split(strings.TrimSpace(string(output)), "\n")
    require.Equal(t, expectedCount, len(lines), "Expected %d nodes, got %d", expectedCount, len(lines))

    // Verify all nodes are Ready
    for _, line := range lines {
        require.Contains(t, line, "Ready", "Node not ready: %s", line)
    }
    t.Logf("All %d nodes are Ready", expectedCount)
}

func assertNoExternalNetworkCalls(t *testing.T, cluster *K0sCluster) {
    // This is a simplified check - in practice you'd inspect logs or network metrics
    // For now, just verify iptables rules are in place
    for _, node := range cluster.nodes {
        output := execCommandWithOutput(t, node, []string{"iptables", "-L", "OUTPUT", "-n"})
        require.Contains(t, output, "DROP", "No blocking rules found on node %s", node.name)
    }
    t.Log("Verified airgap networking is active")
}
```

- `upgrade.NewInfraUpgrader`: Creates actual upgrader from `pkg-new/upgrade/upgrader.go`

- `upgrader.UpgradeK0s()`: **This is the production code path**
  - Creates autopilot Plan CR with new k0s version
  - Plan points to LAM for artifact fetching
  - Monitors Plan status until completion
  - This tests the actual upgrade orchestration used in production

- `waitForAutopilotPlanComplete`: Polls autopilot Plan CR
  - Checks `Plan.Status.State` until it reaches `Completed`
  - Fails if Plan reaches `IncompleteTargets` or times out
  - Logs progress during the upgrade

- `assertK0sVersion`: Verifies k0s version on all nodes
  - SSHs into each node and runs `k0s version`
  - Compares output to expected version

- `assertAllNodesReady`: Checks all nodes are Ready in Kubernetes
  - Uses `kubectl get nodes` to verify all 3 nodes are Ready
  - Ensures cluster is healthy post-upgrade

- `assertClusterFunctional`: Basic smoke test
  - Creates a test pod and verifies it runs
  - Tests kubectl connectivity
  - Verifies cluster API is responsive

- `assertNoExternalNetworkCalls`: Verifies airgap simulation worked
  - Checks iptables rules were respected
  - Optionally: inspect k0s logs to verify all artifacts came from LAM

- `assertLAMServedArtifacts`: Verifies LAM was actually used
  - Checks LAM access logs to confirm k0s binary and images were requested
  - Validates the upgrade pulled from LAM, not external sources

**Why this test is valuable:**

1. **Multi-node coordination**: Tests that autopilot upgrades all 3 nodes in the correct order
2. **Real autopilot**: Uses actual k0s autopilot, not a mock
3. **Real upgrade logic**: Uses production `upgrader.UpgradeK0s()` code path
4. **Airgap validation**: Proves upgrade works without internet access
5. **LAM integration**: Verifies LAM file server integration works correctly
6. **Fast isolation**: Takes 10-15 minutes vs 60+ minutes for full E2E

**What makes it different from the E2E test:**

- **No full install**: Bootstraps k0s directly via CLI (faster)
- **Minimal components**: Only k0s + ECO, no registry/seaweedfs/admin console
- **Focused scope**: Only tests k0s autopilot upgrade, nothing else
- **Faster**: ~10-15 minutes vs 60+ minutes
- **Better failure diagnosis**: If it fails, we know it's the autopilot upgrade specifically

This approach tests the **actual autopilot upgrade mechanism** with **real multi-node orchestration** but in a **controlled environment** that's 4x faster than the full E2E test.
