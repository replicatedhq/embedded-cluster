# TLS CLI Configuration Proposal

## TL;DR (solution in one paragraph)

Enable TLS configuration during embedded-cluster CLI installation by unhiding the existing `--tls-cert` and `--tls-key` flags and passing them to the KOTS installer. This allows users to provide custom TLS certificates via command-line arguments, eliminating the manual browser-based TLS setup step that currently breaks automation workflows. The implementation leverages existing TLS infrastructure, maintains backward compatibility by keeping self-signed certificate generation as the default, and creates the necessary `kotsadm-tls` secret before KOTS installation to ensure the Admin Console starts with the correct TLS configuration.

## The problem

Users performing automated installations of embedded-cluster are forced to manually complete TLS configuration through the Admin Console web interface, breaking their automation workflows. This affects users with ephemeral test environments, CI/CD pipelines, and enterprise deployments that require consistent, fully automated installations.

**Evidence:**
- Pixee.ai reports this as a daily pain point for their automated deployments
- GitHub issue: https://github.com/replicated-collab/pixee-replicated/issues/81
- Users can configure everything else via CLI (ConfigValues, proxy settings) except TLS
- The manual step requires browser access and human intervention, incompatible with headless environments

**Impact:**
- Prevents fully automated installations in CI/CD pipelines
- Blocks ephemeral environment provisioning
- Increases deployment time and complexity
- Creates inconsistency between automated and manual installations

## Prototype / design

### High-Level Flow
```
CLI Installation with TLS
├── Parse TLS flags (--tls-cert, --tls-key)
├── Validate certificates
├── Create kotsadm-tls secret
├── Pass TLS config to KOTS
└── Admin Console starts with TLS configured

Data Flow:
User → CLI Flags → Installer → TLS Validation → Secret Creation → KOTS → Admin Console
```

### Key Components:
1. **CLI Interface**: Unhide `--tls-cert` and `--tls-key` flags for all installations
2. **Certificate Handling**: Load and validate certificates, support file paths and inline content
3. **Secret Management**: Create `kotsadm-tls` secret before KOTS installation
4. **KOTS Integration**: Pass TLS configuration to KOTS installer

### User Experience:
```bash
# Installation with custom certificates
embedded-cluster install \
  --license license.yaml \
  --tls-cert /path/to/cert.pem \
  --tls-key /path/to/key.pem \
  --assume-yes

# Installation with self-signed (default)
embedded-cluster install --license license.yaml
```

## New Subagents / Commands

No new subagents or commands will be created. This proposal only modifies existing installation commands.

## Database

**No database changes required.**

The TLS configuration is stored in Kubernetes secrets, not in any database tables.

## Implementation plan

### Files to modify:

1. **cmd/installer/cli/install.go**
   - Remove TLS flag hiding logic (lines 359-372)
   - Add TLS validation for non-V3 installs
   - Pass TLS config to `runKotsInstall()` function
   - Support inline certificate content via stdin/environment variables

2. **cmd/installer/kotscli/kotscli.go**
   - Add TLS fields to `InstallOptions` struct:
     ```go
     type InstallOptions struct {
         // existing fields...
         TLSCertBytes []byte
         TLSKeyBytes  []byte
         Hostname     string
     }
     ```
   - Create `kotsadm-tls` secret before KOTS install
   - Remove or conditionally apply `--exclude-admin-console` flag

3. **pkg/addons/adminconsole/install.go**
   - Export `createTLSSecret()` for use by KOTS installer
   - Ensure idempotent secret creation

### External contracts:
- **APIs consumed**: None
- **APIs emitted**: None
- **Events**: Standard Kubernetes secret creation events

### Toggle strategy:
- **Feature flag**: None required, uses existing flag presence
- **Rollout**: Immediate availability when flags are provided
- **Fallback**: Maintains current behavior when flags not provided

## Testing

### Unit tests:
- Certificate validation with valid/invalid certificates
- Secret creation with proper labels and data
- Flag parsing for file paths and inline content
- Backward compatibility when flags not provided

### Integration tests:
- Install with custom certificates and verify Admin Console TLS
- Install with self-signed and verify generation
- Install without TLS flags and verify backward compatibility
- Automated install with `--assume-yes` and TLS config

### E2E tests:
- Full installation flow with custom certificates
- Certificate rotation scenario
- Multi-node cluster with TLS configuration
- Restore from backup with TLS settings

### Test data:
- Valid certificate/key pairs for testing
- Expired certificates for error testing
- Mismatched certificate/key for validation testing
- Multi-SAN certificates for hostname testing

## Monitoring & alerting

### Metrics:
- `tls_configuration_method` (custom vs self-signed)
- `tls_certificate_expiry_days` 
- `tls_configuration_errors_total`

### Logs:
- Info: "Using custom TLS certificate for Admin Console"
- Warning: "No TLS certificate provided, generating self-signed certificate"
- Error: "Failed to validate TLS certificate: [error details]"

### Health checks:
- Verify `kotsadm-tls` secret exists and is valid
- Check certificate expiration in monitoring

### Alert thresholds:
- Certificate expiring in < 30 days (warning)
- Certificate expiring in < 7 days (critical)
- TLS configuration failures > 0 (critical)

## Backward compatibility

### API versioning:
- No API changes required
- CLI flags are additive and optional

### Data compatibility:
- Existing installations unaffected
- TLS secrets use same format as current Admin Console

### Migration windows:
- No migration required
- Existing automated scripts continue working without changes

### Compatibility matrix:
- Works with all supported KOTS versions
- Compatible with existing automation tools
- No breaking changes to existing workflows

## Migrations

**No special deployment handling required.**

The changes are backward compatible and don't require any migration steps. Existing installations continue to work as before, and new installations can optionally use the TLS flags.

## Trade-offs

### Optimizing for:
1. **Automation compatibility** - Eliminating manual steps
2. **Backward compatibility** - Not breaking existing workflows
3. **Simplicity** - Leveraging existing infrastructure

### Trade-offs made:
1. **Exposing more flags** vs **Cleaner CLI**
   - Chose to expose flags for automation needs
   
2. **Creating secret before KOTS** vs **Letting KOTS create it**
   - Chose pre-creation for better control and consistency
   
3. **Supporting inline content** vs **File paths only**
   - Chose to support both for flexibility in CI/CD environments

## Alternative solutions considered

### 1. Pass TLS via ConfigValues
- **Rejected because**: Would require application manifest changes and doesn't align with infrastructure configuration pattern

### 2. Create separate `configure-tls` command
- **Rejected because**: Adds complexity and additional step to automation

### 3. Use environment variables only
- **Rejected because**: Less discoverable and harder to validate

### 4. Modify KOTS to accept TLS flags directly
- **Rejected because**: Requires changes to KOTS CLI, longer implementation timeline

### 5. Keep V3-only implementation
- **Rejected because**: Doesn't solve immediate user need, V3 timeline uncertain

## Research

**Reference**: [TLS CLI Configuration Research](./tls_cli_configuration_research.md)

### Prior art in codebase:
- [TLS flag implementation (V3)](https://github.com/replicatedhq/embedded-cluster/blob/main/cmd/installer/cli/install.go#L353-L372)
- [Admin Console TLS secret creation](https://github.com/replicatedhq/embedded-cluster/blob/main/pkg/addons/adminconsole/install.go#L218-L264)
- [Proxy configuration pattern](https://github.com/replicatedhq/embedded-cluster/blob/main/cmd/installer/cli/install.go#L570-L581)
- [ConfigValues file support](https://github.com/replicatedhq/embedded-cluster/blob/main/cmd/installer/kotscli/kotscli.go#L82-L84)

### External references:
- [KOTS CLI documentation](https://docs.replicated.com/reference/kots-cli-install)
- [Kubernetes TLS secret format](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)
- [X.509 certificate validation in Go](https://pkg.go.dev/crypto/tls#LoadX509KeyPair)

### Prototypes:
- Tested TLS secret pre-creation with manual `kubectl create secret`
- Verified KOTS accepts existing `kotsadm-tls` secret
- Validated certificate loading from files and environment variables

## Checkpoints (PR plan)

### Single PR approach:
Given the focused scope and interdependent changes, this will be implemented as a single PR containing:

1. **Flag exposure**: Unhide TLS flags for all installations
2. **Validation logic**: Certificate validation for non-V3 installs  
3. **Secret creation**: Pre-create `kotsadm-tls` before KOTS install
4. **KOTS integration**: Pass TLS config to KOTS installer
5. **Tests**: Unit and integration tests for TLS configuration
6. **Documentation**: Update CLI help text and examples

The changes are tightly coupled and testing requires all components to work together, making a single PR the most appropriate approach.