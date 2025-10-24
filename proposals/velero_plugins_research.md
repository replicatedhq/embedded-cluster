# Velero Plugins Research

## Current Architecture

### Velero Addon Implementation

The Velero addon in Embedded Cluster is currently implemented as a Helm-based addon with the following key components:

1. **Installation Process** (`pkg/addons/velero/install.go`):
   - Creates prerequisites (namespace, credentials secret)
   - Generates Helm values from template
   - Installs using Helm client

2. **Helm Values Generation** (`pkg/addons/velero/values.go`):
   - Base values from `static/values.tpl.yaml`
   - Supports proxy configuration
   - Handles custom CA bundles
   - Allows value overrides via `UnsupportedOverrides.BuiltInExtensions`

3. **Current Plugin Support**:
   - AWS plugin is hardcoded as an initContainer in `static/values.tpl.yaml`
   - Plugin image: `velero-plugin-for-aws`
   - No mechanism for custom plugins

### Configuration Schema

The EC configuration (`kinds/apis/v1beta1/config_types.go`) uses:
- `UnsupportedOverrides.BuiltInExtensions[]` for addon value overrides
- Each `BuiltInExtension` has:
  - `Name`: Chart name (e.g., "velero")
  - `Values`: YAML-formatted Helm values

### Current Limitations

1. **No Custom Plugin Support**: Only the AWS plugin is included
2. **Hardcoded Plugin Configuration**: Plugin specified in static template
3. **No Plugin Validation**: No mechanism to verify plugin images
4. **Limited Extensibility**: Users cannot add database-specific backup tools

## Velero Plugin Architecture

### Plugin Distribution Model

Velero plugins are distributed as OCI container images that:
1. Contain the plugin binary
2. Run as initContainers in the Velero pod
3. Copy the binary to a shared volume
4. Are loaded by the Velero server at startup

### Plugin Types

Velero supports multiple plugin types:
- **BackupItemAction**: Executes custom logic for items during backup
- **RestoreItemAction**: Executes custom logic for items during restore
- **VolumeSnapshotter**: Implements volume snapshot operations
- **ObjectStore**: Implements object storage operations
- **DeleteItemAction**: Executes custom logic for item deletion

### Plugin Loading Mechanism

From the Helm chart values reference:
```yaml
initContainers:
  - name: velero-plugin-for-aws
    image: velero/velero-plugin-for-aws:v1.10.0
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins
```

The plugin container copies its binary to `/target` which is mounted as a shared volume accessible to the Velero server.

### Plugin Configuration

Plugins are configured via ConfigMaps with specific labels:
```yaml
configMaps:
  plugin-config:
    labels:
      velero.io/plugin-config: ""
      velero.io/plugin-name: "BackupItemAction"
    data:
      # Plugin-specific configuration
```

## Similar Patterns in EC

### Addon Value Overrides

The existing `UnsupportedOverrides.BuiltInExtensions` pattern allows users to override Helm values:
```yaml
unsupportedOverrides:
  builtInExtensions:
    - name: velero
      values: |
        key: value
```

### Image Management

EC already handles custom images through:
- Image replacement in templates
- Proxy registry domain configuration
- Image validation for airgap scenarios

## Security Considerations

### Current Security Model

1. **Velero Permissions**: Runs with cluster-admin equivalent permissions
2. **Plugin Execution Context**: Plugins run within Velero's security context
3. **Network Access**: Plugins inherit Velero's network policies

### Required Security Enhancements

1. **Image Verification**: Need to verify plugin image signatures
2. **Supply Chain Security**: Validate image provenance
3. **Runtime Isolation**: Consider additional sandboxing
4. **Audit Logging**: Track plugin operations

## Technical Requirements

### Configuration Changes Needed

1. **New Config Field**: Add plugin specification to EC config
2. **Helm Value Injection**: Pass plugins to Velero Helm chart
3. **Validation Logic**: Verify plugin image format and availability

### Implementation Approach

1. **Extend ConfigSpec**: Add `Extensions.Velero.Plugins[]` field
2. **Modify Values Generation**: Inject plugin initContainers
3. **Add Validation**: Check plugin image accessibility
4. **Support Multiple Plugins**: Allow array of plugin specifications

## Use Cases

### Primary Use Case: Database Backups

Users want to use specialized database backup tools:
- **PostgreSQL**: barman, pgbackrest
- **MySQL**: mysqldump integration
- **MongoDB**: mongodump integration

### Secondary Use Cases

1. **Custom Storage Providers**: Non-AWS S3-compatible storage
2. **Pre/Post Backup Hooks**: Custom application quiesce logic
3. **Data Transformation**: Encryption, compression during backup
4. **Compliance**: Audit logging, data residency controls

## Alternatives Analysis

### Alternative 1: Sidecar Containers

Instead of Velero plugins, use sidecar containers:
- **Pros**: Complete isolation, independent lifecycle
- **Cons**: No integration with Velero workflow, separate orchestration needed

### Alternative 2: Operator Pattern

Create separate operators for backup operations:
- **Pros**: Full control, Kubernetes-native
- **Cons**: Complex implementation, no Velero benefits

### Alternative 3: Job-Based Backups

Use Kubernetes Jobs/CronJobs:
- **Pros**: Simple, well-understood
- **Cons**: No unified backup management, limited restore capabilities

## Existing Proposals Review

### V3 Infrastructure Upgrade Proposal

Key patterns to follow:
- Comprehensive architecture diagrams
- Clear upgrade paths
- Detailed testing requirements
- Rollback procedures

### Strict Preflight Blocking Proposal

Security patterns to adopt:
- Validation at multiple stages
- Clear error messaging
- Feature flags for rollout

## Open Questions

1. **Plugin Registry**: Should EC maintain an approved plugin registry?
2. **Version Compatibility**: How to handle Velero/plugin version mismatches?
3. **Resource Limits**: Should plugins have resource constraints?
4. **Multi-tenancy**: How do plugins work with multiple applications?
5. **Testing Requirements**: What testing must vendors provide?

## Next Steps

1. Design configuration schema for plugin specification
2. Implement security validation framework
3. Create plugin development guide
4. Build example plugins for testing
5. Define support boundaries