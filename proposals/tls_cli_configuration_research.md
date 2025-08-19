# TLS CLI Configuration Research

## Current State Analysis

### TLS Infrastructure
The embedded-cluster codebase already has robust TLS handling infrastructure:

1. **TLS Flags** (`cmd/installer/cli/install.go`):
   - `--tls-cert` and `--tls-key` flags are defined (lines 353-354)
   - Hidden unless `ENABLE_V3=1` is set (lines 359-372)
   - Flag validation and loading logic exists (lines 625-649)
   - Support for both provided certificates and self-signed generation

2. **Admin Console TLS** (`pkg/addons/adminconsole/install.go`):
   - `createTLSSecret()` method creates `kotsadm-tls` secret (lines 218-264)
   - Secret contains TLS certificate, key, and hostname
   - Properly labeled for disaster recovery

3. **KOTS CLI Integration** (`cmd/installer/kotscli/kotscli.go`):
   - Currently uses `--exclude-admin-console` flag (line 75)
   - No TLS configuration passed to KOTS install

### Key Observations

1. **V3 Installer Separation**: TLS features are gated behind `ENABLE_V3` environment variable
2. **Existing TLS Loading**: Certificate loading and validation logic already exists
3. **Admin Console Bypass**: KOTS install explicitly excludes admin console configuration
4. **ConfigValues Support**: KOTS already supports `--config-values` flag for automation
5. **Proxy Configuration**: Proxy settings are already passed to KOTS during installation

### Gap Analysis

The main gap is that TLS configuration is collected but not passed to KOTS during installation:
- TLS certificates are loaded and validated in the installer
- Admin Console TLS secret is created 
- But KOTS install uses `--exclude-admin-console` and doesn't receive TLS config

## KOTS TLS Support Investigation

Based on the codebase analysis and external knowledge:

1. **KOTS TLS Flags**: KOTS CLI likely supports TLS configuration flags:
   - `--tls-skip-verify` for disabling TLS verification
   - Potentially `--tls-cert` and `--tls-key` for custom certificates
   - May require additional flags for hostname configuration

2. **Admin Console Configuration**: 
   - KOTS creates its own TLS configuration when not excluded
   - Need to understand how to pass TLS settings without excluding admin console
   - Or continue excluding but pass TLS config through other means

## Implementation Approach Options

### Option 1: Remove --exclude-admin-console and Pass TLS Config
- Remove `--exclude-admin-console` flag from KOTS install
- Pass TLS certificate and key to KOTS CLI
- Let KOTS handle Admin Console configuration with provided TLS

### Option 2: Continue Excluding Admin Console, Configure TLS Separately
- Keep `--exclude-admin-console` 
- Create TLS secret before KOTS install
- Configure KOTS to use existing TLS secret

### Option 3: Use KOTS Config Values
- Include TLS settings in ConfigValues file
- Pass through existing `--config-values` mechanism
- May require KOTS application manifest changes

## Technical Considerations

1. **Certificate Validation**: 
   - Existing validation in `tls.LoadX509KeyPair()` 
   - Need to ensure certificates are valid for intended hostnames
   - Support for IP SANs and DNS names

2. **Self-Signed vs Custom Certificates**:
   - Current logic prompts for self-signed if no certs provided
   - Need to maintain this UX for interactive installs
   - Silent handling for automated installs with `--assume-yes`

3. **Backward Compatibility**:
   - Must work with existing automation scripts
   - Should not break installs without TLS flags
   - Consider feature flag or gradual rollout

4. **Secret Management**:
   - TLS secrets need proper labeling for backup/restore
   - Ensure secrets are created in correct namespace
   - Handle secret updates for certificate rotation

## File Changes Required

1. **cmd/installer/cli/install.go**:
   - Unhide TLS flags for non-V3 installs
   - Pass TLS config to KOTS install function

2. **cmd/installer/kotscli/kotscli.go**:
   - Add TLS parameters to InstallOptions struct
   - Pass TLS flags to kubectl-kots command
   - Handle TLS secret creation timing

3. **pkg/addons/adminconsole/install.go**:
   - May need to adjust TLS secret creation timing
   - Ensure compatibility with KOTS TLS handling

## Testing Requirements

1. **Installation Scenarios**:
   - Install with custom certificates
   - Install with self-signed certificates
   - Install without TLS flags (backward compatibility)
   - Automated install with --assume-yes

2. **Certificate Types**:
   - Standard domain certificates
   - Wildcard certificates
   - IP-based certificates
   - Multi-SAN certificates

3. **Error Cases**:
   - Invalid certificate/key pairs
   - Expired certificates
   - Mismatched certificate and key
   - Missing certificate files

## Related Components

1. **Manager Experience**: V3 installer with different TLS handling
2. **Proxy Configuration**: Already passed to KOTS, similar pattern
3. **ConfigValues**: Existing automation mechanism
4. **Host CA Bundle**: CA certificate trust configuration
5. **Disaster Recovery**: TLS secrets need proper labeling

## Next Steps

1. Verify KOTS CLI TLS flag support
2. Test KOTS behavior with TLS configuration
3. Choose implementation approach based on KOTS capabilities
4. Implement minimal changes for backward compatibility
5. Add comprehensive test coverage
6. Update documentation and examples