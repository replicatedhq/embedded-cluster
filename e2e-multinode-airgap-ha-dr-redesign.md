# Redesigning `TestMultiNodeAirgapHADisasterRecovery`

## Objective

Reduce `TestMultiNodeAirgapHADisasterRecovery` from its observed 55–62 minute runtime to a **10-minute E2E target** while preserving meaningful coverage of the customer-critical path. Correctness, maintainability and fidelity to the customer workflow take priority over meeting an exact duration.

The current test is several tests composed serially. It builds a three-node installation, deploys an application, enables HA, creates a backup, destroys the cluster, restores it, validates it, imports an update, and performs an application/EC upgrade. The measured successful path spent approximately 2 minutes provisioning, 4 minutes acquiring and preparing bundles, 7 minutes installing, 8 minutes enabling HA, 3 minutes backing up, 4 minutes resetting, 13 minutes restoring/rejoining, and 7 minutes upgrading.

Approaching 10 minutes is possible only if the E2E test stops creating the state that it later restores. Bundle construction, backup production, detailed Velero behavior, resource selection, HA transformations, and the post-restore upgrade must move out of this E2E. These changes should be made because they create clearer test boundaries, not merely to satisfy a stopwatch.

## Scope of the new E2E

The new test should answer one question:

> Given an airgapped, restore-ready three-node environment, a valid release bundle, and a compatible HA backup, can the shipped `embedded-cluster restore` workflow recreate the cluster, join the remaining controllers, and recover the expected data?

It should retain only behavior that cannot be represented faithfully in kind:

1. Start three clean hosts with the bundle and backup fixture already staged.
2. Isolate the hosts from the public network.
3. Start the fixture S3 endpoint.
4. Run restore phase 1 on the first controller.
5. Join the other two controllers in parallel using restore join commands.
6. Run restore phase 2.
7. Assert that three controllers are ready, EC reports the expected version and HA state, the restored PVC contains a marker, and one application probe is healthy.

The test must not create an application release, download a Market-built bundle, perform an initial installation, enable HA, create its own backup, exercise reset warnings, install test dependencies at runtime, use Playwright, import an upgrade bundle, perform an upgrade, or collect support bundles after success. Failure diagnostics remain automatic and bounded.

The test name can remain unchanged because it still verifies multi-node, airgap, HA disaster recovery. Its contract becomes narrower and should be documented in the test itself.

## Runtime budget

| Phase | Target allocation |
|---|---:|
| Provision three CMX hosts concurrently | 2:00 |
| Stage/start local S3 fixture and enforce airgap | 0:30 |
| Restore phase 1 and first-controller readiness | 2:30 |
| Join two restore controllers in parallel | 2:00 |
| Restore phase 2 and component readiness | 2:00 |
| Data, HA, version and network assertions | 0:45 |
| Normal cleanup overhead | 0:15 |
| **Total target** | **10:00** |

These allocations are approximate and are not measurements. In particular, CMX provisioning and Kubernetes/Velero readiness have variable latency. The proposed design may exceed the 10-minute budget by up to approximately five minutes in its initial implementation. The first implementation should emit structured timestamps for every phase and report its runtime trend. If a phase is materially slower than expected, profile unnecessary waits, serialization and repeated work, but do not weaken product assertions or introduce brittle infrastructure merely to eliminate a small overrun.

CMX is the preferred backend for this E2E. The GitHub runner itself does not need to be airgapped; only the nodes or containers where EC is installed must be denied public network access. It is possible in principle to create isolated container networks on a GitHub runner while retaining runner connectivity for orchestration and logs.

The open question is whether a runner-hosted implementation can enforce that boundary reliably while also providing the host behavior this test requires: three independent nodes, privileged operations, systemd and k0s lifecycle, kernel and mount behavior, stable intra-cluster networking, reset/reboot semantics, and sufficient CPU, memory and disk. A container-based backend should replace CMX only after demonstrating those properties consistently. Until then, CMX provides the more faithful and predictable environment at a relatively small provisioning cost.

Provision all CMX hosts concurrently and use a prepared image containing `expect` and other immutable test dependencies. Stage the bundle and fixture before network isolation. Controller joins should run concurrently unless the product explicitly requires serialization.

For this test, “airgapped” means that after staging:

- the three nodes have no route to or permitted egress toward the public internet;
- cluster-internal traffic and traffic to the fixture S3 service remain available;
- public DNS resolution from the nodes is unavailable or unusable;
- the GitHub runner retains only the management path needed to execute commands and collect results;
- the management path cannot be used by workloads or cluster services as an egress gateway.

The test should assert the boundary immediately after isolation: a public HTTPS request and public DNS lookup from a node must fail, while node-to-node and fixture-S3 probes must succeed. CMX control-plane/API communication and the harness SSH channel are out-of-band test management, not part of the installed cluster's network. If CMX cannot prevent workloads from reaching the internet through that management channel, the isolation mechanism must be fixed before calling the test airgapped.

## Coverage moved to targeted integration tests

The existing kind integration harness already installs the real Velero add-on. Extend it with a local S3 service and small backup fixtures. Keep its tests independently runnable and parallelize suites where Docker resources allow.

| Behavior removed from the E2E | New coverage |
|---|---|
| Backup discovery and grouping of infra/application sub-backups | Existing CLI unit tests plus Kubernetes API integration fixtures |
| Legacy versus improved DR | Table-driven unit tests and one real-Velero test for each manifest shape |
| Labels selecting EC install, Admin Console, operator, application, registry and SeaweedFS resources | Rendered-manifest contract test and real Velero backup/restore fixture |
| Airgap, HA, service-IP and artifact-mirror annotations | API-level integration test covering serialization, validation and restore-plan consumption |
| rqlite conversion to one replica and removal of PVC selected-node annotation | Real Velero restore using the resource-modifier ConfigMap |
| Registry and SeaweedFS service-IP preservation | Real Velero restore followed by Service assertions |
| Persistent-volume backup and restore | Minimal PVC containing a marker file restored through the configured node agent/plugin |
| Vendor-provided improved-DR `Backup` and `Restore` specs | Fixture release processed into real Velero resources and restored in kind |
| Incomplete backup synchronization and restore resume states | Fake-client state-machine tests with deterministic phase transitions and failures |
| Reset HA warning and forced reset behavior | Installer integration tests using lightweight hosts; separate from DR |
| Application deployment and post-restore application upgrade | KOTS coverage plus the dedicated EC/KOTS boundary upgrade canary |
| Airgap update import | Local-bundle install/upgrade integration or dedicated E2E, not DR |

Create fixtures under `tests/integration/kind/velero/static/` with source manifests and expected restored forms. Generate opaque backup data through a checked-in fixture generator rather than editing it manually. Every fixture must record its schema version, Velero version, EC version constraints, input manifest digest, and generation command. CI should fail when generated output differs from committed metadata.

Restore-plan logic currently embedded in CLI functions should be extracted into a package with Kubernetes-client interfaces. The CLI and the integration tests must call the same implementation. Tests that merely reproduce restore selectors or resource modifiers independently would allow production and test behavior to drift.

## Local airgap bundle builder

The bundle used by this test should be built locally once per workflow and shared with every test that needs the same release. It must be structurally equivalent to the EC v2 bundle documented in `airgap-bundle-construction.md`; a tar concatenation or test-only approximation is insufficient.

### Shared implementation

Implement the construction engine as a public Go package in this repository, with a thin CLI wrapper. A possible boundary is:

```text
pkg/airgapbundle/
  plan.go          # resolve and validate immutable inputs
  build.go         # construct application and embedded-cluster content
  metadata.go      # channel-release and bundle metadata
  archive.go       # deterministic OCI/tar/gzip output
  verify.go        # structural and digest verification

cmd/ec-airgap-bundle/
  main.go          # build, verify and inspect commands
```

The package should accept explicit immutable inputs rather than calling Market directly:

- processed application release and discovered application images;
- generated channel-release metadata;
- license document supplied by the caller;
- EC version metadata and installer archive;
- extension and built-in charts;
- KOTS, operator and optional manager artifacts;
- resolved EC images and local artifact-mirror content;
- target architecture and output writer.

Network fetching, customer authentication, channel lookup, object-storage upload and queue orchestration remain adapters owned by the production airgap builder. Archive layout, release embedding, image/chart packaging, metadata generation and verification live in the shared package.

Because Embedded Cluster is public, the production airgap builder can import a tagged version of this module or execute the versioned CLI. Importing the package is preferred because it provides typed inputs and errors. The airgap builder must not retain a second implementation. Its tests should run shared golden vectors: given the same pinned inputs, the local CLI and production adapter must produce the same manifest, entry digests and embedded release metadata. Byte-for-byte equality is desirable once timestamps and archive ordering are deterministic.

### Determinism and security

The builder should:

- sort archive entries and normalize timestamps, ownership and file modes;
- address reusable inputs by digest and verify every downloaded object;
- emit a build manifest containing input and output digests;
- avoid putting credentials in the build manifest or cache key;
- separate the reusable release payload from customer-specific license material where the v2 format permits, while still producing the exact final outer archive;
- provide `verify` and `inspect` commands that never execute embedded binaries;
- generate an SBOM/provenance record for the builder and its resolved inputs.

### Workflow artifacts and reuse

Create one workflow job that builds the required **bundle set**. A bundle is release-specific, so install and upgrade releases still require separate final bundles; “reuse” means every consumer of a particular release uses the identical artifact rather than rebuilding or downloading it independently.

For the redesigned DR test, build only the install/restore bundle. It no longer needs an upgrade bundle. Publish the bundle with a key derived from:

```text
bundle-format + architecture + EC-version-metadata-digest
+ application-release-digest + channel-metadata-digest + license-profile-digest
```

Other airgap tests should download the workflow artifact onto the runner and copy it to the test node. Replace calls to `downloadAirgapBundleOnNode` with a helper that first resolves a declared workflow bundle and stages it. The helper must not silently fall back to Market; the separate release-gate canary owns that path.

Reusable caches should contain EC installer inputs, image blobs, charts and artifacts keyed by content digest. Final customer/test bundles should be workflow artifacts with normal retention rather than long-lived mutable caches. The build job should publish one manifest listing all bundles so consumers can verify the expected application label, EC version, architecture and SHA-256 before use.

## Backup fixture for the E2E

The E2E backup fixture should contain the smallest valid state accepted by the production restore command:

- EC installation/runtime configuration needed to reconstruct the cluster;
- Admin Console state required for restore orchestration;
- the operator and required extensions;
- airgap registry and SeaweedFS state required by the selected architecture;
- a minimal application workload and one PVC marker;
- improved-DR backup grouping and all required EC annotations.

Build the fixture using the same release bundle produced above and a dedicated generator workflow or local command. Fixture generation is not part of the E2E critical path. Check the non-secret fixture into Git LFS or publish it as an immutable versioned test artifact, depending on size and repository policy. A fixture is accepted only after a slower generation-validation job proves that it can be restored by its declared EC version. Do not regenerate it on every PR.

Treat fixtures like compatibility test data, not snapshots that are refreshed automatically when tests fail. Changes to restore schema, bundled Velero, k0s storage, or required metadata must intentionally update the generator and fixture. The PR updating a fixture should include its old/new manifest diff.

### Fixture production and storage

Do not use the GitHub Actions cache as the authoritative fixture store. Caches are mutable, evictable, scoped in ways that can produce surprising fallback behavior, and do not express compatibility. A cache may accelerate an immutable download, but a test must still resolve a fixture by digest and verify it.

Provide a `dr-fixture build` command and a manually dispatched/reusable producer workflow:

1. Resolve an exact EC commit/version, KOTS artifact set, bundle manifest and fixture schema.
2. Create a minimal three-controller installation in a producer environment.
3. Deploy the fixture application and PVC marker, enable HA, and create a real backup.
4. Stop writes and export the S3 prefix containing the logical backup and all sub-backups.
5. Normalize and compress the exported object tree.
6. Emit a manifest with the EC commit, KOTS source commit and artifact digests, k0s and Velero versions, bundle digest, backup names, fixture schema, creation command and payload SHA-256.
7. Validate the fixture by restoring it once before publishing.
8. Publish the payload and manifest as an immutable OCI artifact, preferably in a public GHCR test-fixtures repository, addressed by digest. A GitHub release asset is an acceptable initial implementation.

The blocking E2E downloads the declared digest, verifies it, unpacks it directly into the local MinIO data directory, and starts MinIO. It does not run a snapshot job first. The workflow cache key may be the fixture digest, but a cache miss simply downloads the immutable artifact.

Fixture production should run only when intentionally requested—for example when restore schema, the baseline release or fixture contents change. It is not a nightly job and must not silently replace an existing fixture tag. Pull requests changing the default fixture update a small declaration file containing the digest and compatibility metadata, making the change reviewable.

Use a released baseline fixture to test restoration with a candidate EC/KOTS build. This models the important customer operation: new software must restore backups produced by an older supported release. Building KOTS from a branch therefore does not normally require regenerating the fixture. A candidate-specific fixture is useful when developing the backup producer itself, but it is a separate producer/consumer compatibility test rather than part of the optimized E2E.

### Local execution and KOTS source selection

The same test command must support both CI and a developer workstation. Separate artifact preparation from test execution:

```text
make e2e-dr-assets \
  EC_REF=<local-worktree-or-commit> \
  KOTS_REF=main

make e2e-dr-run
```

`e2e-dr-assets` produces a local asset manifest containing the EC bundle, KOTS binaries/images and DR fixture digests. `e2e-dr-run` accepts only that manifest and does no compilation or registry discovery. It provisions CMX using the developer's normal CMX credentials, so “local” means that asset preparation and test control can run from a workstation; the airgapped nodes still run in CMX. CI publishes the same manifest as a workflow artifact, and a local run may build it locally or download it from a GitHub workflow.

`KOTS_REPOSITORY` does not need to be a user-facing parameter. Define it as `replicatedhq/kots` in the Makefile and keep `KOTS_REF` as the only KOTS source input. `KOTS_REF` may be a branch, tag or commit in that repository. Resolve it to a commit SHA before looking up or building artifacts, and record the SHA in provenance. Avoid mutable refs such as `main` in bundle metadata or cache keys.

KOTS currently builds `kotsadm` and `kots` through a Melange package and builds the `kotsadm`, migrations and kurl-proxy images from that package with APKO. Preserve that path for E2E artifacts; do not substitute `go build` plus ad hoc Dockerfiles because the test would no longer exercise the shipped filesystem, dependencies or image construction.

Avoid cross-compilation and emulation:

- For the current amd64 CMX E2E, build the Melange package and APKO images on an amd64 Linux runner with `--arch amd64`/`archs: amd64` only.
- An amd64 Linux developer can run the same native Melange/APKO commands locally.
- An arm64 workstation targeting amd64 CMX should request a native amd64 GitHub build for the selected KOTS repository/ref and download its artifact manifest. It should not build amd64 Go binaries locally or run the APKO build under QEMU.
- Add a native arm64 build only when an arm64 E2E consumer exists; do not make this test wait for a multi-architecture KOTS image index.

The preferred long-term interface is a reusable workflow in KOTS that accepts `source-ref`, a non-production version label and target architecture, then returns test-only APKs, OCI image layouts and a `kots.tar.gz` with digests. It should reuse KOTS's existing Melange spec, APKO configs and local pipeline override. The workflow must build from the resolved source commit without creating release tags or pushing production images.

Until that workflow exists, Embedded Cluster can provide a pinned wrapper that checks out the requested KOTS ref from `replicatedhq/kots` and invokes the KOTS-owned Melange/APKO configs. The wrapper should contain orchestration only; package and image definitions remain in KOTS.

The local airgap builder consumes the resulting OCI layouts and binary archives by digest. It must allow these KOTS inputs to override the KOTS artifacts referenced by normal EC version metadata and must write the override commit and digests into the test bundle manifest. Every KOTS image reference embedded in the release and every image stored in `images-amd64.tar` must resolve to the candidate artifact set; mixing candidate binaries with released images should fail verification.

### Commit-addressed KOTS artifact cache

Cache native KOTS build output by resolved commit SHA and target architecture. Asset preparation should first resolve `KOTS_REF` to `KOTS_COMMIT`, fetch a signed manifest for that commit, and build only on a cache miss.

Suggested durable keys are:

```text
s3://<test-artifacts>/kots/<commit>/linux-amd64/kots.tar.gz
s3://<test-artifacts>/kots/<commit>/linux-amd64/packages.tar.gz
s3://<test-artifacts>/kots/<commit>/linux-amd64/manifest.json

<durable-registry>/kotsadm:<last-release-version>-sha<commit>
<durable-registry>/kotsadm-migrations:<last-release-version>-sha<commit>
<durable-registry>/kurl-proxy:<last-release-version>-sha<commit>
```

The `sha` prefix is required before the commit value so the prerelease identifier cannot be interpreted as an all-numeric SemVer identifier, including a value with a leading zero. The tag has no architecture suffix: either publish only the platform needed by this test or publish a multi-platform image index under the same tag. Record the platforms present in the artifact manifest and fail before bundle construction if the required platform is absent.

GitHub Actions artifacts can be used for handoff within or between recent workflow runs, but their retention policy makes S3 preferable for a persistent binary cache. The manifest should contain the KOTS commit, last release version used in the tag, architecture/platform set, Melange package digests, image digests, binary archive digest and build provenance. Consumers resolve image tags once and record immutable OCI digests in the local bundle manifest; tags are lookup keys, not the integrity boundary.

Use an existing durable Docker Hub or GHCR repository if one is available, preferably under a test-specific namespace with an immutable-tag policy. Pushing commit tags into normal production image repositories is acceptable only if repository ownership and retention policy explicitly permit test artifacts. The build must reject an existing tag whose digest does not match its published manifest rather than overwrite it.

`ttl.sh` is useful for an ad hoc local build but is not suitable as the shared cache because its images expire. A developer may select it as an ephemeral publication transport for an immediate run, but CI and reproducible local runs should use the durable commit-addressed artifacts. Garbage collection for the durable cache can retain commits referenced by active EC branches, releases and DR fixtures, with an age-based policy for the rest.

Commit SHA is the primary cache identity because the Melange spec, APKO configs and source are all stored in KOTS. Include architecture and a cache-schema version in the manifest and physical key so the cache can be deliberately invalidated if external builder behavior or artifact layout changes without a KOTS source change.

## Proposed test flow

```text
build-local-bundles (once per workflow)
    |
    +-- install/restore bundle --> all matching airgap tests
    +-- upgrade bundle ---------> upgrade tests only

kind DR integration tests (parallel)
    |- real Velero backup/restore of labeled resources
    |- resource modifiers and service-IP preservation
    |- improved/legacy DR manifests and metadata
    `- PVC marker recovery

TestMultiNodeAirgapHADisasterRecovery
    |- provision three clean prepared CMX hosts concurrently
    |- stage verified install bundle and immutable backup fixture
    |- enforce airgap; start fixture S3
    |- restore phase 1
    |- join controllers 2 and 3 concurrently
    |- restore phase 2
    `- assert HA, version, PVC marker and application probe
```

## Acceptance criteria

The redesign is complete when:

1. The new E2E passes repeatedly and targets a p95 runtime at or below 10 minutes, measured from host setup through normal cleanup. An initial overrun of up to approximately five minutes is acceptable while measured bottlenecks are addressed; correctness is not traded for the target.
2. It performs no Market application-release creation or bundle download and no post-restore upgrade.
3. All removed assertions have named unit or integration-test replacements running as blocking pull-request checks.
4. Every airgap E2E declares and consumes a bundle from the shared workflow manifest.
5. The local builder and production airgap builder use the same public construction package and golden compatibility vectors.
6. One release-gate canary still validates application release creation, production airgap construction, Market download, install and restore using the shipped artifact.
7. Local and CI runs can select an exact `replicatedhq/kots` ref, reuse commit-addressed native Melange/APKO artifacts, and record their commit and digests in the bundle manifest.
8. The DR fixture is immutable and digest-addressed; an Actions cache is never its source of truth.
9. Automated probes prove that cluster nodes cannot reach public HTTPS or DNS after isolation while internal and fixture-S3 communication still work.
10. Test logs publish phase timings, bundle/fixture manifests and bounded diagnostics on failure.

The runtime goal must not be met by replacing product assertions with mocks, distorting production architecture, or introducing fragile test infrastructure. Runtime improves by preparing immutable inputs once, exercising Velero and Kubernetes behavior in targeted integration tests, and leaving only the real multi-host restore boundary in the E2E. The quality of those boundaries is the primary design criterion.

## Appendix A: Recommended implementation sequence

The implementation proceeds in this order. A later item starts only after the
preceding item is implemented and validated. Status is maintained in this
document so the remaining work is explicit.

1. **Completed — Produce and validate one immutable backup fixture.** Build
   a fixture from a released EC/KOTS baseline, inspect it for secrets, and prove
   that it restores into a clean three-node CMX environment.
2. **Completed — Add a restore-only CMX test using the existing bundle artifact.**
   Remove installation, backup creation, reset, Playwright application setup,
   bundle download, and post-restore upgrade from the DR test's critical path.
3. **Completed — Replace Playwright join-command retrieval with the product CLI.**
   Use `embedded-cluster join print-command` on the restored first controller.
4. **In progress — Join both restore controllers concurrently.** Generate two
   independent join commands before starting either join, then execute the two
   joins in parallel.
5. **Pending — Add direct restore assertions.** Verify three ready controllers,
   the expected EC version and HA state, the restored PVC marker, and one
   application health probe without Playwright.
6. **Pending — Add explicit airgap-boundary probes.** Verify that public HTTPS
   and public DNS fail while node-to-node and fixture-S3 traffic succeed.
7. **Pending — Collect and analyze phase timings.** Emit structured timings for
   provisioning, staging, isolation, both restore phases, controller joins,
   assertions, and cleanup; establish a measured p95 before further tuning.
8. **Pending — Formalize fixture production and move removed coverage.** Add the
   generator/validation workflow, immutable digest-addressed storage, fixture
   metadata checks, and the blocking unit/integration tests that own behavior
   removed from this E2E.
9. **Pending — Add a separate EC/KOTS boundary upgrade canary.** Keep application
   and post-restore upgrade coverage, optional candidate-KOTS builds, and
   commit-addressed KOTS artifact caching outside this DR test. This item is
   independent of, and does not block, the restore-focused redesign.
