# E2E Test Performance & Stability Improvements

## Problem

Our E2E tests are slow and brittle. A single test like `TestMultiNodeAirgapHADisasterRecovery` spins up a 3-node cluster, does a full airgap install (k0s, addons, extensions, app), joins 2 additional controller nodes in HA mode, creates a backup, resets all 3 nodes, then restores everything. These tests take 70+ minutes and when something fails, we waste a lot of time re-running the entire suite.

Because these tests are so long-running, they're also incredibly flaky. The longer a test runs, the more chances it has to hit a transient network issue, timeout, or race condition. When a test fails after 60 minutes, we don't know if it's a real bug or just bad luck. We end up re-running the same test multiple times hoping it passes, which wastes even more CI time.

The same issue exists for airgap upgrade tests like `TestMultiNodeAirgapUpgrade` - they download airgap bundles, install an old version (k0s, addons, extensions, app), join worker nodes, run the airgap update, then go through the entire upgrade path (upgrade service, k0s autopilot, addons, extensions, app).

## Proposal

Break the monolithic E2E tests into focused integration tests that target individual phases. Instead of testing the entire system end-to-end, we test each component independently with fast, isolated setups.

### Core Principles

Tests must be focused and targeted. Each test should validate a specific phase rather than testing multiple phases at once. This makes failures easier to diagnose and tests faster to run.

Tests must be isolated and not depend on each other. Each test should set up its own environment, test a specific phase, and clean up. As much as possible, tests should not rely on the output or state of another test.

### Strategy

Install, upgrade, and restore are each broken down into multiple phases (k0s, addons, extensions, app, etc.). Each of these phases gets its own isolated test. Use kind clusters instead of k0s wherever possible since k0s takes a long time to bootstrap. Only use k0s when we're actually testing k0s-specific behavior.

### Install Tests

Replace single multi-phase install tests with:
- Test k0s installation standalone
- Test addons on pre-existing kind cluster (single or multi-node)
- Test extensions on pre-existing kind cluster (single or multi-node)
- Test app installation on pre-existing kind cluster
- ... other phases
- Keep one or two full e2e tests for basic smoke testing

### Upgrade Tests

Replace single multi-phase upgrade tests with:

**In KOTS (replicatedhq/kots):**
- Test upgrade service: from when user clicks "Deploy" until the operator upgrade command gets executed. That's it. Don't test what happens after the operator binary runs.

**In Operator:**
- Test artifact distribution: use kind cluster, install registry if airgap, run distribution job, verify success, done
- Test k0s autopilot upgrade (single-node): install old k0s using k0s CLI, run autopilot upgrade, verify success, done
- Test k0s autopilot upgrade (multi-node): bootstrap old multi-node k0s cluster using k0s CLI directly (skip our join flow), run autopilot upgrade, verify all nodes upgrade successfully. That's what we care about - not whether our join logic works, just that k0s autopilot upgrades all nodes correctly.
- Test addon upgrades: use kind cluster (single or multi-node), install old addons, upgrade them, done
- Test extension upgrades: use kind cluster (single or multi-node), install old extensions, upgrade them, done
- Test app upgrade: use kind cluster, install old app version, upgrade app, done

### Restore/DR Tests

Leverage the fact that restore is already broken into phases in `/cmd/installer/cli/restore.go`:
- `runRestoreECInstall()` - k0s restore
- `runRestoreAdminConsole()` - admin console restore
- `runRestoreSeaweedFS()` - seaweedfs restore
- `runRestoreRegistry()` - registry restore
- `runRestoreExtensions()` - extensions restore
- `runRestoreApp()` - app restore
- ... other phases

**Prerequisite:** Modify the `kubectl kots backup` command to work without kotsadm API, rqlite, or an app installed. It should be able to back up only what's available in the cluster (individual components like registry, seaweedfs, etc.) without requiring the full KOTS stack.

**Key insight:** k0s isn't backed up or restored - we always set it up from scratch during restore. So we don't need a backup to test k0s restore functionality.

**Test pattern for each component:**
1. Set up a kind cluster (single or multi-node) with Velero
2. Install only the component being tested (e.g., registry, seaweedfs, extensions, ECO, etc.)
3. Create a backup using `kubectl kots backup`
4. Uninstall the component and delete its data
5. Restore from backup
6. Verify the component is restored correctly

**Specific tests:**
- K0s single-node restore: install k0s from scratch, verify success, done
- K0s multi-node restore: install k0s from scratch as single-node with velero, openebs, and kots only, get join command via kots CLI or UI, join nodes, verify, done
- ECO restore: use kind cluster + velero, install ECO, backup, uninstall, restore, verify
- Admin Console restore: use kind cluster + velero + openebs, install admin console, backup, uninstall, restore, verify
- Registry & SeaweedFS restore: use kind cluster + velero + openebs, install registry and seaweedfs, backup, uninstall, restore, verify
- Extensions restore: use kind cluster + velero, install extensions, backup, uninstall, restore, verify
- App restore: use kind cluster + velero + openebs, install kots and app, backup, uninstall, restore app only, verify
- ... other phases

**Note:** Keep one or two full e2e tests for basic smoke testing.

### Join Tests

Test node join functionality independently from install:
- Install k0s, openebs, and kots only (no app, no other addons or extensions)
- Get join command via kots CLI or UI
- Join nodes to the cluster
- Verify the join succeeds and the HA prompt is shown
- Do not validate the actual HA migration (already covered by separate integration tests using kind)

## Benefits

**Speed:** Most tests run on kind instead of waiting minutes for k0s. Tests can run in parallel since they're isolated.

**Reliability:** When a test fails, we know exactly which phase broke. No more hunting through 70 minutes of logs trying to figure out if it was k0s, an addon, the app, etc...

**Faster iteration:** Developers can run specific tests for what they're working on. Changing addon upgrade logic? Run the addon upgrade tests. Don't need to wait for k0s to install.

**Better CI:** We can parallelize across many more jobs. Instead of 2-3 long-running tests, we have 20+ fast tests running concurrently.

**Easier debugging:** Each test is self-contained. If it fails, the scope is small enough to reproduce and debug quickly.

## Migration Path

1. Start with one example from each category (install, upgrade, restore)
2. Keep existing E2E tests running while new tests are added
3. Once new tests prove stable, reduce the number of full E2E tests to 1-2 basic smoke tests
4. Document patterns so the team can add new tests easily