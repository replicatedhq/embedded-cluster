# Design: NFTables Support for Linux Kernel 6.17+ (EC 2.3)

**Date**: 2026-05-11
**Status**: Approved

## Context & Problem

Starting with Linux kernel 6.17, `CONFIG_NETFILTER_XTABLES_LEGACY` defaults to disabled, removing the legacy `ip_tables` kernel module. Distributions shipping kernel 6.17+ (e.g., Amazon Linux 2023 with kernel 6.18, RHEL 10, Fedora) may not have `ip_tables` available as a loadable module or built-in.

EC currently fails on such hosts because:
1. **Preflight** requires `ip_tables` kernel module to be loadable
2. **Module loading** (`99-embedded-cluster.conf`) hardcodes `ip_tables`
3. **k0s defaults** `kubeProxy.Mode` to `"iptables"`, which crashes kube-proxy when `ip_tables` is absent

On hosts where `ip_tables` IS present (builtin or loadable), everything works as-is. This design adds a conditional path for hosts where `ip_tables` is absent but `nf_tables`+`nft_compat` are available.

## Key Constraints

- **Cannot bump Calico**: k0s 1.32–1.34 is pinned to Calico 3.29. Calico 3.31+ has native nftables dataplane, but EC cannot adopt it until k0s 1.35.
- **Calico Felix 3.29 auto-detects**: With `FELIX_IPTABLESBACKEND=Auto` (default), Calico uses the `iptables-nft` wrapper on nftables-only kernels via `nft_compat`. No Calico config changes needed.

## Design Overview

The solution is a **host capability detection + conditional k0s config** approach:

1. **Detect host iptables backend capability** at install/join time
2. **Update k0s config** to set `kubeProxy.Mode = "nftables"` when legacy iptables is unavailable
3. **Update preflights** to accept either `ip_tables` OR `nf_tables`+`nft_compat`
4. **Update module loading** to skip absent modules gracefully

## Components

### 1. Host Capability Detection

A new Go package or function detects whether the host supports legacy iptables:

```go
// pkg/helpers/kernel/kernel.go or similar

type IPTablesBackend string

const (
    BackendLegacy IPTablesBackend = "legacy"
    BackendNFT      IPTablesBackend = "nft"
    BackendUnknown  IPTablesBackend = "unknown"
)

// DetectIPTablesBackend determines if legacy iptables is available.
// Returns BackendLegacy if ip_tables module is available (loaded, loadable, or builtin).
// Returns BackendNFT if ip_tables is absent but nf_tables and nft_compat are available.
// Returns BackendUnknown if neither backend is detectable.
func DetectIPTablesBackend(ctx context.Context) (IPTablesBackend, error)
```

Detection logic:
1. Check `/proc/net/ip_tables_names` exists (indicates `ip_tables` is loaded or builtin)
2. If not, check if `ip_tables` module file exists under `/lib/modules/$(uname -r)/`
3. If `ip_tables` is absent, check if `nf_tables` module is available (loaded or loadable)
4. If `nf_tables` is available, check if `nft_compat` is available
5. Return `BackendNFT` if `nf_tables` + `nft_compat` are available

Alternative: k0s exposes `github.com/k0sproject/k0s/pkg/component/iptables.DetectHostIPTablesMode()` which detects based on existing iptables rules. However, on a fresh host with no Kubernetes installed, there may be no existing rules to inspect. The module-based check is more reliable for preflight/install-time decisions.

**Decision**: Implement a module-based detection in EC (using `modprobe --dry-run` or checking `/lib/modules/...`). Use `DetectHostIPTablesMode()` as a secondary/confirming signal if needed.

### 2. Conditional k0s Kube-Proxy Mode

`RenderK0sConfig()` is called both at **release build time** (in `pkg-new/metadata/metadata.go` for image listing) and at **install/join time** (in `pkg-new/k0s/k0s.go` and `cmd/installer/cli/join.go`). Host detection must NOT be inside `RenderK0sConfig()` because metadata gathering runs on the build machine, not the target host.

**Approach**: Add a new helper `ApplyHostK0sConfigOverrides()` that patches the already-generated config based on host detection. Call it at install/join time right before writing the config to disk.

**New helper in `pkg/config/config.go`:**
```go
// ApplyHostK0sConfigOverrides detects the host's netfilter backend capability
// and applies any host-specific overrides to the k0s config.
func ApplyHostK0sConfigOverrides(cfg *k0sv1beta1.ClusterConfig) error {
    backend, err := kernel.DetectIPTablesBackend(context.Background())
    if err != nil {
        logrus.WithError(err).Warn("Failed to detect iptables backend, defaulting to iptables")
        return nil
    }
    if backend == kernel.BackendNFT {
        logrus.Info("Host lacks legacy iptables, configuring kube-proxy for nftables mode")
        cfg.Spec.Network.KubeProxy.Mode = "nftables"
    }
    return nil
}
```

**Call sites to update:**

1. **`pkg-new/k0s/k0s.go:NewK0sConfig()`** — after generating base config, before applying unsupported overrides:
```go
func (k *K0s) NewK0sConfig(...) (*k0sv1beta1.ClusterConfig, error) {
    // ... existing config generation ...
    cfg := config.RenderK0sConfig(domains.ProxyRegistryDomain)
    // ... network overrides ...
    
    if err := config.ApplyHostK0sConfigOverrides(cfg); err != nil {
        return nil, fmt.Errorf("apply host k0s config overrides: %w", err)
    }
    
    if mutate != nil { ... }
    // ... rest of existing logic ...
}
```

2. **`cmd/installer/cli/join.go:applyNetworkConfiguration()`** — after generating config, before writing to disk:
```go
func applyNetworkConfiguration(...) error {
    clusterSpec := config.RenderK0sConfig(domains.ProxyRegistryDomain)
    // ... existing overrides ...
    
    if err := config.ApplyHostK0sConfigOverrides(clusterSpec); err != nil {
        return fmt.Errorf("apply host k0s config overrides: %w", err)
    }
    
    // ... marshal and write ...
}
```

**Backwards compatibility**: `RenderK0sConfig()` remains unchanged. On legacy-capable hosts, `ApplyHostK0sConfigOverrides` is a no-op (no override applied). `metadata.go` and `upgrade.go` continue to call `RenderK0sConfig()` without host detection.

### 3. Preflight Spec Update

The current preflight in `pkg-new/preflights/specs/host-preflight-common.yaml` has a `kernelModules` check that requires `ip_tables`:

```yaml
- kernelModules:
    checkName: "IP tables kernel module"
    outcomes:
      - pass:
          when: "rosetta == loaded"
          message: The kernel is likely linuxkit, skipping kernel module check
      - pass:
          when: "ip_tables == loaded,loadable"
          message: The 'ip_tables' kernel module is loaded or loadable
      - fail:
          when: ""
          message: The 'ip_tables' kernel module is not loaded or loadable
```

**Change**: Replace with a check that accepts either `ip_tables` OR `nf_tables`+`nft_compat`:

```yaml
- kernelModules:
    checkName: "IP tables or NFTables kernel module"
    outcomes:
      - pass:
          when: "rosetta == loaded"
          message: The kernel is likely linuxkit, skipping kernel module check
      - pass:
          when: "ip_tables == loaded,loadable"
          message: The 'ip_tables' kernel module is loaded or loadable (legacy iptables available)
      - pass:
          when: "nf_tables == loaded,loadable"
          message: The 'nf_tables' kernel module is loaded or loadable (nftables available)
      - fail:
          when: ""
          message: Neither 'ip_tables' (legacy iptables) nor 'nf_tables' (nftables) kernel module is available. At least one netfilter backend must be present.
```

**Note**: The `kernelModules` collector/analyzer in troubleshoot/preflight checks modules independently. We may need to restructure to use a custom `run` collector + `textAnalyze` if we need an "OR" across two different modules. Alternatively, we can check both modules and make both pass independently (i.e., two separate `kernelModules` checks, each with its own pass condition, and no fail condition for the nftables one). 

**Refined approach**: Use two separate `kernelModules` checks:

```yaml
- kernelModules:
    checkName: "Legacy IP tables kernel module"
    outcomes:
      - pass:
          when: "ip_tables == loaded,loadable"
          message: The 'ip_tables' kernel module is loaded or loadable
      - warn:
          when: ""
          message: The 'ip_tables' kernel module is not loaded or loadable. Will check for nftables alternative.

- kernelModules:
    checkName: "NFTables kernel module"
    outcomes:
      - pass:
          when: "nf_tables == loaded,loadable"
          message: The 'nf_tables' kernel module is loaded or loadable
      - warn:
          when: ""
          message: The 'nf_tables' kernel module is not loaded or loadable.

- textAnalyze:  # custom analyzer to ensure at least one passed
    checkName: "Netfilter backend available"
    fileName: host-collectors/run-host/kernel-modules.txt
    # This requires the kernelModules collector to also write raw output, or we use a run collector
```

Actually, troubleshoot's `kernelModules` analyzer is limited. A better approach is a **custom run collector** that checks modules via a script, then analyze the output:

```yaml
- run:
    collectorName: 'check-netfilter-backend'
    command: 'sh'
    args: ['-c', '
      if modprobe -n ip_tables 2>/dev/null || lsmod | grep -q "^ip_tables"; then
        echo "backend=legacy"
      elif modprobe -n nf_tables 2>/dev/null || lsmod | grep -q "^nf_tables"; then
        echo "backend=nft"
      else
        echo "backend=none"
      fi
    ']

analyzers:
- textAnalyze:
    checkName: "Netfilter backend"
    fileName: host-collectors/run-host/check-netfilter-backend.txt
    regex: 'backend=(legacy|nft)'
    outcomes:
      - pass:
          when: "true"
          message: "Host has a usable netfilter backend (legacy iptables or nftables)"
      - fail:
          when: "false"
          message: "No usable netfilter backend found. Neither ip_tables (legacy iptables) nor nf_tables (nftables) kernel module is available."
```

**Decision**: Use a custom `run` collector + `textAnalyze` for the netfilter backend check. Remove the existing `ip_tables`-only `kernelModules` check.

### 4. Module Loading Configuration

Current `pkg-new/hostutils/static/modules-load.d/99-embedded-cluster.conf`:
```
overlay
ip_tables
#ip6_tables
br_netfilter
nf_conntrack
```

**Problem**: `systemd-modules-load` (or `modprobe` when iterating manually) fails if a module in `modules-load.d` is absent, causing installation to error out.

**Changes**:

1. **Update the static config file** to include nftables modules:
```
overlay
ip_tables
nf_tables
nft_compat
#ip6_tables
br_netfilter
nf_conntrack
```

2. **Update the module loading logic** in `pkg-new/hostutils/system.go` (`ensureKernelModulesLoaded()`) to **gracefully skip** modules that don't exist:

```go
func ensureKernelModulesLoaded() (finalErr error) {
    modules := []string{"overlay", "ip_tables", "nf_tables", "nft_compat", "br_netfilter", "nf_conntrack"}
    for _, mod := range modules {
        // Check if module exists before attempting to load
        if err := checkModuleExists(mod); err != nil {
            logrus.Debugf("Module %s not available on this kernel, skipping", mod)
            continue
        }
        if err := loadModule(mod); err != nil {
            finalErr = multierr.Append(finalErr, fmt.Errorf("load module %s: %w", mod, err))
        }
    }
    return
}
```

`checkModuleExists` can use `modprobe -n <module>` (dry-run) or check `/lib/modules/$(uname -r)/modules.builtin` and the modules directory.

**`sysctl.d/99-embedded-cluster.conf`**: No changes needed. Sysctls like `net.bridge.bridge-nf-call-iptables = 1` control netfilter bridge processing and work correctly with both legacy iptables and nftables (via `nft_compat` or native nftables bridge support).

### 5. Calico Felix Configuration

**No change needed.**

The current `enableCalicoNetworkProvider()` only sets `FELIX_USAGEREPORTINGENABLED=false`. `FELIX_IPTABLESBACKEND` defaults to `Auto`, which:
- On legacy-capable hosts → uses legacy mode
- On nftables-only hosts → uses `iptables-nft` wrapper via `nft_compat`

This is the correct behavior for Calico 3.29. Do NOT set `FELIX_IPTABLESBACKEND=nft` (that would break older kernels).

### 6. Kube-Proxy Image

**No changes needed.** The upstream `kube-proxy` image (including the version proxied by EC) already contains the `nft` binary and required libraries. This was verified by inspecting the running kube-proxy container on the test host:
```
/proc/8363/root/usr/bin/nft: ELF 64-bit LSB executable
/proc/8363/root/usr/sbin/nft: ELF 64-bit LSB executable
```

The initial error observed (`"failed to run nft: Error: No such file or directory"`) was a **transient** failure during kube-proxy startup (likely the `nft` binary not yet being in the container filesystem during initialization). The pod eventually recovered and successfully started in nftables mode.

## Backwards Compatibility

| Scenario | Before Change | After Change |
|---|---|---|
| Legacy-capable host (ip_tables present) | Works | Works (zero change) |
| nftables-only host (ip_tables absent, nf_tables+nft_compat present) | Fails preflight + kube-proxy crash | Passes preflight + kube-proxy nftables mode |
| Host with neither backend | Fails preflight | Fails preflight (clearer message) |
| Mixed cluster (legacy control plane + nftables worker, or vice versa) | N/A | Not supported. First control plane node determines cluster-wide kube-proxy mode. All nodes must support the selected mode. |

## Key Files to Modify

1. `pkg/helpers/kernel/kernel.go` (NEW) — Host capability detection
2. `pkg/config/config.go` — `ApplyHostK0sConfigOverrides()` helper
3. `pkg-new/k0s/k0s.go` — Call `ApplyHostK0sConfigOverrides()` in `NewK0sConfig()`
4. `cmd/installer/cli/join.go` — Call `ApplyHostK0sConfigOverrides()` in `applyNetworkConfiguration()`
5. `pkg-new/preflights/specs/host-preflight-common.yaml` — Netfilter backend check
6. `pkg-new/hostutils/system.go` — Graceful module loading with `checkModuleExists()`
7. `pkg-new/hostutils/static/modules-load.d/99-embedded-cluster.conf` — Add `nf_tables` and `nft_compat`
8. `pkg-new/hostutils/interface.go` — Update interface if new methods added
9. `pkg-new/hostutils/mock.go` — Update mock

## Testing Plan

1. **Unit tests** for `DetectIPTablesBackend()` with mocked filesystem/module checks
2. **Dry-run tests** for preflight spec rendering (verify netfilter check renders correctly)
3. **E2E regression on legacy-capable host** (existing Docker/LXD tests — zero behavior change expected)
4. **Manual testing on nftables-only host** (e.g., AL2023 with kernel 6.18 where `ip_tables` is absent). Automated E2E is not feasible because CMX does not provide a VM image with this kernel profile.

## References

- [Calico v3.31 NFTables GA](https://www.tigera.io/blog/whats-new-in-calico-v3-31-ebpf-nftables-and-more/#calico-nftables) — For future reference when k0s 1.35/Calico 3.31 is adopted
- [Felix auto-detection logic](https://docs.tigera.io/calico/latest/reference/felix/configuration)
- [Kubernetes 1.35 kube-proxy nftables mode](https://kubernetes.io/blog/2025/02/28/nftables-kube-proxy/)
- [k0s iptables networking docs](https://docs.k0sproject.io/v1.28.7+k0s.0/networking/#iptables)
- k0s `pkg/component/iptables/iptables.go` — Host detection logic (in module cache)
- Shortcut story: [136108](https://app.shortcut.com/replicated/story/136108)
