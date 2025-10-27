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

Tests must be parallelizable. All tests should be possible to run concurrently, given sufficient hardware. No test should conflict with other tests.

---

## Option 1: Phase-Based Integration Tests

### Strategy

Break install, upgrade, and restore into their constituent phases (k0s, addons, extensions, app, etc.). Each phase gets its own test. Use kind clusters instead of k0s wherever possible since k0s takes a long time to bootstrap. Only use k0s when we're actually testing k0s-specific behavior.

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

Restore is already broken into phases in `/cmd/installer/cli/restore.go`, so we can test each phase independently without doing a full install first.

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

**Prerequisite:** Modify the `kubectl kots backup` command to work without kotsadm API, rqlite, or an app installed. It should be able to back up only what's available in the cluster (individual components like registry, seaweedfs, etc.) without requiring the full KOTS stack.

### Join Tests

Test node join functionality independently from install:
- Install k0s, openebs, and kots only (no app, no other addons or extensions)
- Get join command via kots CLI or UI
- Join nodes to the cluster
- Verify the join succeeds and the HA prompt is shown
- Do not validate the actual HA migration (already covered by separate integration tests using kind)

### Benefits

**Speed:** Most tests run on kind instead of waiting for k0s. Tests run in minutes instead of hours. Tests can run in parallel since they're isolated.

**Reliability:** When a test fails, we know exactly which phase broke. Reduced flakiness from targeting specific components instead of testing everything together.

**Focused failures:** No more hunting through 70 minutes of logs trying to figure out if it was k0s, an addon, or the app. Each test targets one specific phase.

**Faster iteration:** Developers can run specific tests for what they're working on. Changing addon upgrade logic? Run the addon upgrade tests. Don't need to wait for k0s to install.

**Better CI:** We can parallelize across many more jobs. Instead of 2-3 long-running tests, we have 20+ fast tests running concurrently.

**Easier debugging:** Each test is self-contained. If it fails, the scope is small enough to reproduce and debug quickly.

### Migration Path

1. Start with one example from each category (install, upgrade, restore, join)
2. Keep existing E2E tests running while new tests are added
3. Once new tests prove stable, reduce the number of full E2E tests to 1-2 basic smoke tests
4. Document patterns so the team can add new tests easily

---

## Option 2: Integration Point Testing with Mocked Dependencies

### Strategy

Test integration points rather than full flows. For each external dependency (k0s autopilot, Velero, Helm), verify that we interact with it correctly by mocking its behavior. We create the expected Kubernetes resources with correct specs, then automatically update their status to simulate how the external system would behave. This allows us to test our handling of success, failure, timeout, and edge cases without actually running k0s, Velero, or Helm.

### Install Tests

Test that we correctly generate and apply installation resources:

**K0s Installation:**
- Test that we generate the correct k0s config (YAML validation)
- Test that we call k0s install with correct parameters
- Don't actually install k0s, just verify we're calling it correctly

**Addon Installation:**
- Test that we generate correct Helm values for each addon
- Test that we create Helm releases with correct parameters
- Mock Helm responses to test error handling
- Test different addon configurations and validate the generated values

**Extension Installation:**
- Test that we parse extension configs correctly
- Test that we generate correct Helm values for extensions
- Test that we track extension state correctly
- Mock Helm to simulate installation success/failure

**App Installation:**
- Test that we generate correct KOTS resources
- Test that we configure the app correctly based on config values
- Mock KOTS API responses

### Upgrade Tests

Test that we correctly orchestrate upgrades and handle responses:

**K0s Autopilot Upgrade:**
- Test that we create the autopilot plan CR with correct specs (version, nodes, etc.)
- Have the test automatically update the plan status to simulate:
  - In-progress state
  - Successful completion
  - Failure with error message
  - Timeout scenario
- Verify our code correctly waits for and handles each state
- Don't actually run k0s or wait for autopilot

**Artifact Distribution:**
- Test that we create Kubernetes jobs with correct specs (image, env vars, volumes, etc.)
- Mock job status updates to simulate completion/failure
- Test that we correctly monitor job status
- Test retry logic when jobs fail

**Addon Upgrades:**
- Test that we generate correct Helm upgrade values
- Test that we call Helm upgrade with correct parameters
- Mock Helm to simulate upgrade success/failure
- Test rollback logic when upgrades fail

**Extension Upgrades:**
- Test extension upgrade logic (install, upgrade, uninstall, noop)
- Test that we mark extensions with correct status
- Mock Helm responses for each scenario

**App Upgrades:**
- Test that we trigger app upgrades correctly
- Test that we monitor app upgrade status
- Mock KOTS API responses

### Restore/DR Tests

Test that we correctly create backup/restore resources and handle Velero's responses:

**Backup Creation:**
- Test that we create Velero backup CRs with correct specs
- Test that we include/exclude correct resources
- Mock Velero's backup status updates (in-progress, completed, failed)
- Test that we handle backup failures correctly

**Restore Execution:**
- Test that we create Velero restore CRs with correct specs
- Test that we configure resource modifiers correctly for our use cases
- Mock Velero's restore status updates:
  - In-progress
  - Completed successfully
  - Failed with errors
  - Partially successful with warnings
- Test that we handle each restore phase correctly
- Test that we detect and report restore failures

**Component-Specific Restore:**
- Test restore logic for each component (ECO, admin console, registry, seaweedfs, extensions, app)
- Verify correct restore ordering and dependencies
- Mock Velero to test edge cases (missing backups, corrupted data, etc.)

### Join Tests

Test node join token generation and validation:

**Join Token Generation:**
- Test that we generate correct join commands with proper tokens
- Test that we include correct flags and configuration
- Test HA vs non-HA join command differences

**Join Flow:**
- Test that we detect when nodes join successfully
- Test that we detect HA eligibility and show the prompt
- Mock node status updates to simulate join success/failure
- Don't actually join nodes, just test the orchestration logic

### Benefits

**Speed:** Tests run in seconds. No clusters needed, no waiting for operations to complete. Just verify resource creation and mock responses.

**Reliability:** No flakiness from external systems. Tests are completely deterministic since we control all dependencies. A test failure means our code is wrong, not that k0s or Velero had a bad day.

**Focused failures:** Each test validates one integration point. When it fails, you know exactly which API call or resource spec is wrong.

**Faster iteration:** Tests run in your IDE in seconds. You can run thousands of tests in the time it takes to bootstrap one k0s cluster.

**Better coverage:** Easy to test edge cases and error conditions that would be hard or impossible to trigger in real clusters:
- What happens when autopilot times out?
- What happens when Velero restore partially succeeds?
- What happens when Helm upgrade fails halfway through?
- What happens when jobs get deleted unexpectedly?

**Better CI:** Tens of thousands of fast unit tests are better than a handful of slow E2E tests. We can test more scenarios more quickly.

### Migration Path

1. Start with one example from each category:
   - K0s autopilot upgrade with mocked plan status
   - Velero backup with mocked status updates
   - Addon installation with mocked Helm responses
2. Keep existing E2E tests running while new tests are added
3. Once integration tests prove reliable, reduce E2E tests to 1-2 basic smoke tests
4. Document patterns for mocking each external system
5. Create test utilities for common mocking scenarios

---

## Option 3: Dryrun Testing

### Strategy

Test operations in "dryrun mode" where interactions with external dependencies are intercepted and recorded instead of actually invoked. This allows writing small, focused tests for any specific functionality without needing real clusters or external systems.

You can test any piece of functionality in isolation. Want to test OpenEBS data directory configuration? Write a test for that. Want to test custom port handling? Write a test for that. Want to test Helm values for a specific scenario? Write a test for that.

The dryrun framework intercepts interactions with external dependencies:
- Helm operations (install, upgrade, rollback)
- Kubernetes API calls
- Shell commands
- File system operations
- Environment variables
- Reporting metrics

### What You Can Test

Write small, targeted tests for any functionality:

**Installation pieces:**
- Helm values for any addon in any configuration
- k0s configuration with different flags
- Environment variable setup
- Command ordering and parameters
- Port configuration (admin console, LAM, etc.)
- TLS certificate setup
- Data directory configuration
- Metrics reporting
- Preflight collector configuration

**Upgrade pieces:**
- Individual addon upgrades with specific configurations
- Helm upgrade values for different scenarios
- Pre-upgrade cleanup operations
- Configuration changes between versions
- Airgap vs online mode differences
- HA vs single-node differences

**Any specific functionality:**
- How a flag affects Helm values
- How config values propagate to addons
- How proxy settings are applied
- How custom domains are configured
- Registry domain rewriting
- Node port selection

Each test is small and focused on one specific thing. No need to test entire install or upgrade flows - just test the piece you care about.

### Benefits

**Speed:** Tests run in seconds. No clusters, no waiting for real operations.

**No infrastructure needed:** Tests don't need k0s, kind, or any cluster. They run anywhere without special setup.

**Deterministic:** No flakiness from external systems. Tests are completely reproducible.

**Real code paths:** Unlike pure unit tests, dryrun tests exercise the actual code paths. They just don't make actual system changes.

**Granular testing:** Write as many small tests as you want. Each test can focus on one specific piece of functionality. Want to verify 10 different Helm value configurations? Write 10 small tests.

**Easy to write:** Initialize dryrun, call the function you want to test, validate the outputs. Simple pattern, quick to implement.

**Comprehensive validation:** Can validate Helm values, commands, environment variables, metrics, Kubernetes resources - whatever the code produces.

**Fast iteration:** Tests run in seconds. Write a test, run it, verify it, move on.

**Good coverage:** Can test edge cases that would be hard to reproduce with real clusters:
- Different configuration combinations
- Various flags and options
- Airgap vs online modes
- HA vs single-node scenarios
- Custom data directories and ports
- Error conditions and invalid inputs

### Limitations

**Not a real cluster:** Dryrun tests don't catch issues that only manifest when actually running operations:
- Helm chart incompatibilities
- Kubernetes version issues
- Resource constraints
- Timing issues
- Network problems
- Actual cluster behavior

**Limited scope:** Can only test that we're interacting with external dependencies correctly with correct parameters, not that the operations actually work.

### Migration Path

1. Infrastructure already exists in `pkg/dryrun/` - ready to use
2. Test patterns exist in `tests/dryrun/` - copy and adapt
3. Start adding small, focused tests for any functionality you want to validate
4. Don't worry about testing entire flows - just test the pieces you care about
5. Keep real integration/E2E tests for actual cluster validation
6. Use dryrun tests for fast feedback on every PR

### Existing Tests

Current dryrun tests in `tests/dryrun/`:
- `install_test.go` - Various install configurations (default, custom data dir, custom ports, TLS, etc.)
- `install_http_proxy_test.go` - Install with HTTP proxy configuration
- `install_prompts_test.go` - Install with interactive prompts
- `update_test.go` - Update command with airgap bundles
- `upgrade_test.go` - Individual addon upgrades (OpenEBS, etc.)
- `join_test.go` - Join command validations

---

## Comparison

| Aspect | Option 1 | Option 2 | Option 3 |
|--------|----------|----------|----------|
| **Speed** | Minutes per test | Seconds per test | Seconds per test |
| **Real clusters needed** | Yes (kind or k0s) | No | No |
| **Flakiness risk** | Low (isolated phases) | None (fully controlled) | None (fully controlled) |
| **Confidence level** | Higher (components work) | Lower (only contracts) | Medium (real code paths, mocked systems) |
| **Edge case testing** | Difficult | Easy | Easy |
| **Setup complexity** | Medium (need kind/k0s) | Low (just mock data) | Low (dryrun framework exists) |
| **Maintenance** | Medium | Low (keep mocks aligned) | Low (framework already exists) |
| **CI time** | Minutes (parallelized) | Seconds | Seconds |
| **Developer experience** | Need cluster access | Run in IDE | Run in IDE |
| **Tests real code paths** | Yes | Partial (unit level) | Yes (full command flow) |
| **Infrastructure exists** | Need to build | Need to build | Already exists |

## Recommendation

Use **Option 3 (Dryrun Testing) + minimal E2E tests** as the primary testing strategy.

### Practical Migration Strategy

**Convert most E2E tests to dryrun tests:**

Most E2E tests (like those in `e2e/install_test.go`) can be converted to dryrun tests. The value comes from testing **configuration correctness** - which dryrun tests excel at.

**What dryrun tests give us (85-90% confidence):**
- Configuration correctness (Helm values, k0s config, environment variables)
- Flag handling and option processing
- Command generation and ordering
- Resource specification (what gets created with what values)
- Metrics and telemetry reporting
- Installation/upgrade orchestration logic
- Edge cases and error handling for configuration combinations

**Keep 5-6 E2E tests for actual cluster behavior (10-15% additional confidence):**

1. **Basic single-node install** → Verify cluster actually works
2. **Multi-node install** → Verify nodes join and HA works
3. **Airgap install** → Verify airgap bundle handling
4. **Upgrade path** → Verify upgrade from previous version works
5. **Disaster recovery** → Verify backup/restore works
6. **Proxy install** → Verify proxy configuration works

**Why this works:**

Most bugs are in **how we configure things**, not in the external systems themselves (k0s, Helm, Kubernetes generally work). Dryrun tests catch configuration bugs extremely well and run in seconds instead of hours.

**Risk mitigation:**
1. Run E2E tests on every release candidate (not every PR)
2. Use dryrun tests for fast feedback on PRs (seconds vs hours)
3. Keep E2E tests focused on happy paths that actually exercise the cluster
4. Add more dryrun tests whenever you find a configuration bug

**Prioritization:**
1. Start with Option 3 (dryrun tests) - infrastructure exists, easy to write, immediate value
2. Convert existing E2E tests to dryrun tests where they test configuration
3. Keep 4-5 E2E tests for actual cluster validation

This gives us fast feedback on every PR (dryrun tests in seconds) and high confidence in actual cluster behavior (minimal E2E tests), without spending hours on CI for every commit.

**Future considerations:**

Options 1 and 2 can be added later if needed:
- **Option 2** for integration contract testing if we need more sophisticated error handling tests
- **Option 1** for phase-based testing if we need more granular real cluster validation
