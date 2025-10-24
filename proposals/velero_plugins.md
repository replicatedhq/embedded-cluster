# Velero Plugins Support in Embedded Cluster

## TL;DR

We're enabling vendors to extend Velero's backup and restore capabilities in Embedded Cluster by supporting custom Velero plugins packaged as OCI images. This allows integration of specialized backup tools like PostgreSQL's barman or pgbackrest directly into the EC disaster recovery workflow. Plugins are specified in the EC configuration, validated for security, and injected into the Velero deployment as initContainers following Velero's standard plugin architecture. The implementation maintains strict security boundaries through image verification, signature validation, and supply chain attestation while providing vendors the flexibility to implement database-specific backup strategies that integrate seamlessly with EC's existing backup infrastructure.

## The problem

Embedded Cluster currently provides disaster recovery through Velero with a fixed AWS plugin for object storage. However, many production applications require specialized backup strategies for their databases that go beyond generic volume snapshots:

1. **Database-specific requirements**: Production databases need application-consistent backups using native tools (pg_basebackup, mysqldump, mongodump) rather than volume snapshots
2. **Limited extensibility**: The current hardcoded plugin configuration prevents vendors from adding custom backup logic
3. **Integration gaps**: External backup systems cannot hook into EC's backup/restore workflow, forcing vendors to implement separate disaster recovery mechanisms
4. **Compliance needs**: Regulated industries require specific backup encryption, audit trails, and data handling that generic solutions cannot provide

Without plugin support, vendors must choose between EC's built-in disaster recovery (which may not meet their requirements) or implementing completely separate backup systems (losing EC's unified management experience).

## Prototype / design

```
┌─────────────────────────────────────────────────────────────────┐
│                    EC Configuration (YAML)                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ extensions:                                             │    │
│  │   velero:                                               │    │
│  │     plugins:                                            │    │
│  │     - image: myvendor/velero-postgresql:v1.0.0          │    │
│  │       imagePullSecret: my-registry-secret               │    │
│  │     - image: myvendor/velero-mongodb:v2.1.0             │    │
│  └─────────────────────────────────────────────────────────┘    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Validation Layer                              │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  • Image format validation (OCI compliance)              │    │
│  │  • Signature verification (cosign/notary)                │    │
│  │  • Registry accessibility check                          │    │
│  │  • Vulnerability scanning (optional)                     │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│              Velero Helm Values Generation                       │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  initContainers:                                         │    │
│  │    - name: velero-plugin-for-aws                         │    │
│  │      image: velero/velero-plugin-for-aws:v1.10.0         │    │
│  │      volumeMounts:                                       │    │
│  │        - mountPath: /target                              │    │
│  │          name: plugins                                   │    │
│  │    - name: velero-postgresql                             │    │
│  │      image: myvendor/velero-postgresql:v1.0.0            │    │
│  │      imagePullPolicy: IfNotPresent                       │    │
│  │      volumeMounts:                                       │    │
│  │        - mountPath: /target                              │    │
│  │          name: plugins                                   │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Velero Deployment                             │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  Pod Spec:                                               │    │
│  │    initContainers: [aws-plugin, pg-plugin, mongo-plugin] │    │
│  │    containers: [velero-server]                           │    │
│  │    volumes:                                              │    │
│  │      - name: plugins                                     │    │
│  │        emptyDir: {}                                      │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  Plugin Loading:                                         │    │
│  │    1. InitContainers copy binaries to /plugins           │    │
│  │    2. Velero scans /plugins on startup                   │    │
│  │    3. Plugins register their capabilities                │    │
│  │    4. Velero invokes plugins during backup/restore       │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### Plugin Interface

Vendors implement Velero's standard plugin interfaces:

```go
// BackupItemAction for custom backup logic
type BackupItemAction interface {
    Execute(item runtime.Object, backup *velerov1.Backup) (runtime.Object, []ResourceIdentifier, error)
}

// RestoreItemAction for custom restore logic
type RestoreItemAction interface {
    Execute(input *RestoreItemActionExecuteInput) (*RestoreItemActionExecuteOutput, error)
}
```

## New Subagents / Commands

No new subagents or commands will be created. Plugin functionality will be integrated into the existing Velero addon installation and upgrade flows.

## Database

### Schema Changes

No database schema changes are required. Plugin configuration is stored in the EC configuration CRD which already exists in the cluster.

### Configuration Storage

Plugin specifications will be stored in the existing Config CRD:

```yaml
apiVersion: v1beta1
kind: Config
metadata:
  name: embedded-cluster-config
spec:
  extensions:
    velero:
      plugins:
        - image: vendor/plugin:tag
          imagePullSecret: secret-name
```

## Implementation plan

### Files/Services to Modify

1. **Configuration Schema** (`kinds/apis/v1beta1/config_types.go`):
   ```go
   // Add to Extensions struct
   type Extensions struct {
       Velero VeleroExtensions `json:"velero,omitempty"`
   }

   type VeleroExtensions struct {
       Plugins []VeleroPlugin `json:"plugins,omitempty"`
   }

   type VeleroPlugin struct {
       Image           string `json:"image"`
       ImagePullSecret string `json:"imagePullSecret,omitempty"`
   }
   ```

2. **Velero Values Generation** (`pkg/addons/velero/values.go`):
   ```go
   func (v *Velero) GenerateHelmValues(...) {
       // Existing code...

       // Add plugin initContainers
       initContainers := getDefaultInitContainers()
       for _, plugin := range config.Spec.Extensions.Velero.Plugins {
           initContainers = append(initContainers, generatePluginContainer(plugin))
       }
       copiedValues["initContainers"] = initContainers
   }
   ```

3. **Plugin Validation** (`pkg/addons/velero/validation.go` - new file):
   ```go
   func ValidatePlugin(plugin VeleroPlugin) error {
       // Verify OCI image format
       if !isValidOCIImage(plugin.Image) {
           return fmt.Errorf("invalid OCI image format")
       }

       // Verify image signatures if signing is configured
       if err := verifyImageSignature(plugin.Image); err != nil {
           return fmt.Errorf("image signature verification failed: %w", err)
       }

       return nil
   }
   ```

4. **Preflight Checks** (`pkg/preflights/velero.go` - new file):
   ```go
   func CheckPluginAccessibility(plugins []VeleroPlugin) error {
       for _, plugin := range plugins {
           // Check registry accessibility
           // Verify image can be pulled
           // Check for required secrets
       }
   }
   ```

### API Endpoints

No new API endpoints required. Existing configuration endpoints handle the extended schema.

### Feature Toggles

Plugin support will be controlled by:
- **Alpha**: Disabled by default, enabled via `VELERO_PLUGINS_ENABLED=true` environment variable
- **Beta**: Enabled by default with validation, can be disabled via config
- **GA**: Always enabled

### External Contracts

**Plugin Image Requirements**:
- Must be valid OCI images
- Must follow Velero plugin structure (binary at `/plugins/` in container)
- Should include signatures for verification
- Must be accessible from cluster nodes

**Plugin Behavior Contract**:
- Must not modify cluster resources outside backup/restore context
- Must handle errors gracefully
- Must support Velero's timeout and cancellation signals
- Must be idempotent

## Testing

### Unit Tests

1. **Configuration Validation**:
   - Valid/invalid image formats
   - Missing required fields
   - Duplicate plugin detection

2. **Helm Values Generation**:
   - Single plugin injection
   - Multiple plugins
   - Plugin with imagePullSecret
   - Override scenarios

### Integration Tests

1. **Plugin Loading**:
   - Deploy Velero with custom plugin
   - Verify plugin binary copied to shared volume
   - Confirm Velero recognizes plugin

2. **Backup/Restore Operations**:
   - Create backup with custom plugin
   - Verify plugin execution in logs
   - Restore with plugin-modified items

### E2E Tests

1. **Full Workflow**:
   - Configure EC with PostgreSQL plugin
   - Deploy application with PostgreSQL
   - Trigger backup
   - Verify barman/pgbackrest execution
   - Restore to new cluster
   - Validate data integrity

### Load Tests

1. **Multiple Plugins**:
   - Test with 10+ plugins configured
   - Measure startup time impact
   - Monitor memory usage

## Backward compatibility

- Existing clusters without plugins continue working unchanged
- Default AWS plugin remains if no custom plugins specified
- Plugin configuration is additive - doesn't affect existing values
- Upgrades preserve plugin configuration

### API Versioning

The Config CRD uses v1beta1. When moving to v1:
- Plugin configuration will be promoted to stable
- Migration will be automatic via conversion webhook

## Migrations

No migrations required for existing clusters. Plugin support is opt-in and doesn't affect existing configurations.

For clusters adding plugins:
1. Update Config CRD with plugin specification
2. Trigger Velero addon upgrade
3. Verify plugins loaded successfully

## Trade-offs

**Optimizing for**: Vendor flexibility and ecosystem compatibility

**Trade-offs accepted**:
1. **Security complexity**: Accepting third-party code increases attack surface
   - *Mitigation*: Mandatory image signing and verification
2. **Support burden**: EC team cannot debug vendor plugins
   - *Mitigation*: Clear support boundaries in documentation
3. **Startup overhead**: Each plugin adds ~5-10 seconds to Velero startup
   - *Mitigation*: Recommend minimal plugin sets

**Trade-offs rejected**:
1. **Custom plugin format**: Could have created EC-specific plugin format
   - *Rejected because*: Breaks ecosystem compatibility
2. **Built-in plugin marketplace**: Could have provided curated plugins
   - *Rejected because*: Maintenance burden, limits innovation

## Alternative solutions considered

### 1. Sidecar Container Approach
Deploy backup tools as separate pods/sidecars rather than Velero plugins.

**Pros**:
- Complete isolation from Velero
- Independent lifecycle management
- No Velero version dependencies

**Cons**:
- No integration with EC backup workflow
- Requires separate orchestration
- Inconsistent user experience

**Rejected because**: Loses unified disaster recovery experience that Velero provides.

### 2. Operator-Based Backup
Create separate Kubernetes operators for each database type.

**Pros**:
- Full control over backup logic
- Native Kubernetes patterns
- Rich CRD-based configuration

**Cons**:
- Massive implementation effort
- No leverage of Velero ecosystem
- Fragmented backup management

**Rejected because**: Recreates what Velero already provides well.

### 3. Job-Based Backup Hooks
Use Kubernetes Jobs triggered by backup events.

**Pros**:
- Simple implementation
- Well-understood pattern
- No plugin complexity

**Cons**:
- Limited integration points
- No restore-time customization
- Difficult error handling

**Rejected because**: Insufficient for complex backup/restore scenarios.

## Research

See [Velero Plugins Research](./velero_plugins_research.md) for detailed analysis of:
- Current EC Velero implementation
- Velero plugin architecture
- Security considerations
- Alternative approaches

### Prior Art in Codebase

1. **UnsupportedOverrides Pattern**: Already allows Helm value overrides for addons
2. **Image Management**: Existing patterns for image validation and replacement
3. **Addon Upgrade Flow**: Established patterns for upgrading Helm-based addons

### External References

1. [Velero Plugin Development Guide](https://velero.io/docs/v1.14/custom-plugins/)
2. [Velero Plugin Examples](https://github.com/vmware-tanzu/velero-plugin-example)
3. [OCI Image Specification](https://github.com/opencontainers/image-spec)
4. [Cosign Image Signing](https://docs.sigstore.dev/cosign/overview/)

## Checkpoints (PR plan)

### PR 1: Configuration Schema and Validation
- Add `Extensions.Velero.Plugins` to ConfigSpec
- Implement plugin validation logic
- Add unit tests for validation
- Update CRD manifests

### PR 2: Helm Values Integration
- Modify `GenerateHelmValues` to inject plugins
- Add plugin container generation logic
- Update values template handling
- Add integration tests

### PR 3: Preflight and Security Checks
- Implement plugin accessibility checks
- Add image signature verification
- Create security validation framework
- Add preflight tests

### PR 4: Documentation and Examples
- Vendor-facing plugin development guide
- Security best practices documentation
- Example PostgreSQL plugin
- Testing guidelines

### PR 5: E2E Testing and Rollout
- Comprehensive E2E test suite
- Feature flag implementation
- Monitoring and metrics
- Production readiness checklist