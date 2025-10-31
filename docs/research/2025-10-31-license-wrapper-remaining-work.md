---
date: 2025-10-31T15:10:58+0000
researcher: Claude
git_commit: 82b4725d2b19d89b599d17f2c52a0d7248a5f3b5
branch: feature/crdant/supports-license-v1beta2
repository: replicatedhq/embedded-cluster
topic: "Remaining LicenseWrapper and EntitlementFieldWrapper migration work after merge from main"
tags: [research, license, licensewrapper, v1beta2, migration, codebase-audit]
status: complete
last_updated: 2025-10-31
last_updated_by: Claude
---

# Research: Remaining LicenseWrapper and EntitlementFieldWrapper Migration Work

**Date**: 2025-10-31T15:10:58+0000
**Researcher**: Claude
**Git Commit**: 82b4725d2b19d89b599d17f2c52a0d7248a5f3b5
**Branch**: feature/crdant/supports-license-v1beta2
**Repository**: replicatedhq/embedded-cluster

## Research Question

After merging code from main into the feature/crdant/supports-license-v1beta2 branch (which implements v1beta2 license support), are there additional places in the codebase where we need to use `LicenseWrapper` and `EntitlementFieldWrapper` that weren't covered in the original implementation plan?

## Summary

**Yes, there is additional work needed.** The comprehensive codebase audit found:

1. **3 production files** with remaining `kotsv1beta1.License` type usage that need conversion
2. **4 files** with **9 instances** of direct `.Spec.*` field access that should use wrapper methods
3. **2 files** with `EntitlementField` direct value access that should use wrapper abstractions
4. **1 critical merge conflict** with duplicate field definitions that must be resolved

### Critical Issues

1. **Merge Conflict in `api/internal/managers/app/config/manager.go`** (lines 37-38):
   - Duplicate `license` field definitions - both old and new versions present
   - Must remove old `*kotsv1beta1.License` field and keep only `LicenseWrapper`

2. **Production code accessing entitlement values directly** in `cmd/installer/cli/install.go`:
   - Lines 818-831 directly access `.StrVal` on entitlement values
   - Should use wrapper abstraction methods

## Detailed Findings

### 1. Remaining `kotsv1beta1.License` Type References (Non-Test Files)

#### 1.1 `cmd/installer/cli/upgrade.go:52`

**Issue**: Struct field using old license type

```go
type upgradeConfig struct {
    license *kotsv1beta1.License  // Line 52 - Should be licensewrapper.LicenseWrapper
}
```

**Impact**: Medium - Upgrade command needs to support both v1beta1 and v1beta2 licenses

**Fix Required**:
```go
type upgradeConfig struct {
    license licensewrapper.LicenseWrapper
}
```

**Related Code**: Line 129 also has direct field access (see section 2.2)

---

#### 1.2 `api/internal/managers/app/config/manager.go:37-38` ⚠️ CRITICAL

**Issue**: **DUPLICATE FIELD DEFINITIONS** - Merge conflict not resolved

```go
type appConfigManager struct {
    license                  licensewrapper.LicenseWrapper  // Line 37 - NEW
    license                    *kotsv1beta1.License         // Line 38 - OLD (DUPLICATE)
}
```

**Impact**: HIGH - This will cause compilation errors or undefined behavior

**Fix Required**: Remove line 38 entirely:
```go
type appConfigManager struct {
    license                  licensewrapper.LicenseWrapper  // Keep only this
}
```

---

#### 1.3 `api/internal/managers/linux/infra/upgrade.go:116,265`

**Issue**: Local variable declarations using old license type

```go
// Line 116
license := &kotsv1beta1.License{}

// Line 265
license := &kotsv1beta1.License{}
```

**Impact**: Medium - Upgrade operations may fail with v1beta2 licenses

**Fix Required**: Parse license from bytes using wrapper:
```go
license, err := licensewrapper.LoadLicenseFromBytes(m.license)
if err != nil {
    return fmt.Errorf("failed to parse license: %w", err)
}
```

**Related Code**: Lines 142, 143, 290 also have direct field access (see section 2.3)

---

### 2. Direct `.Spec.*` Field Access (Should Use Wrapper Methods)

#### 2.1 `cmd/installer/cli/install.go:139`

**Issue**: Direct access to `.Spec.AppSlug`

```go
// Line 139
installCfg.license.GetLicenseIO(), installCfg.clusterID, installCfg.license.Spec.AppSlug,
```

**Context**: Creating metrics reporter for installation

**Fix Required**:
```go
installCfg.license.GetLicenseIO(), installCfg.clusterID, installCfg.license.GetAppSlug(),
```

---

#### 2.2 `cmd/installer/cli/upgrade.go:129`

**Issue**: Direct access to `.Spec.LicenseID` and `.Spec.AppSlug`

```go
// Line 129
upgradeConfig.license.Spec.LicenseID, upgradeConfig.clusterID, upgradeConfig.license.Spec.AppSlug,
```

**Context**: Creating metrics reporter for upgrade

**Fix Required**:
```go
upgradeConfig.license.GetLicenseID(), upgradeConfig.clusterID, upgradeConfig.license.GetAppSlug(),
```

---

#### 2.3 `api/pkg/template/license.go:151,156,157`

**Issue**: Direct access to channel-related fields

```go
// Line 151
for _, channel := range e.license.Spec.Channels {

// Line 156
if e.license.Spec.ChannelID == e.releaseData.ChannelRelease.ChannelID {

// Line 157
return e.license.Spec.ChannelName, nil
```

**Context**: Resolving channel name for template functions

**Fix Required**:
```go
// Line 151
for _, channel := range e.license.GetChannels() {

// Line 156
if e.license.GetChannelID() == e.releaseData.ChannelRelease.ChannelID {

// Line 157
return e.license.GetChannelName(), nil
```

**Note**: Verify that `GetChannels()` method exists in LicenseWrapper. If not, it needs to be added to kotskinds.

---

#### 2.4 `api/internal/managers/linux/infra/upgrade.go:142,143,290`

**Issue**: Direct access to multiple spec fields

```go
// Line 142
IsDisasterRecoverySupported: license.Spec.IsDisasterRecoverySupported,

// Line 143
IsMultiNodeEnabled: license.Spec.IsEmbeddedClusterMultiNodeEnabled,

// Line 290
return m.upgrader.DistributeArtifacts(ctx, in, localArtifactMirrorImage, license.Spec.LicenseID, appSlug, channelID, appVersion)
```

**Context**: Setting LicenseInfo in Installation spec and distributing artifacts

**Fix Required**:
```go
// Line 142
IsDisasterRecoverySupported: license.IsDisasterRecoverySupported(),

// Line 143
IsMultiNodeEnabled: license.IsEmbeddedClusterMultiNodeEnabled(),

// Line 290
return m.upgrader.DistributeArtifacts(ctx, in, localArtifactMirrorImage, license.GetLicenseID(), appSlug, channelID, appVersion)
```

---

### 3. EntitlementField Direct Value Access

#### 3.1 `cmd/installer/cli/install.go:818-831` ⚠️ PRODUCTION CODE

**Issue**: Directly accessing `.StrVal` field on entitlement value

```go
entitlements := license.GetEntitlements()
if expiresAt, ok := entitlements["expires_at"]; ok {
    expiresAtValue := expiresAt.GetValue()
    if expiresAtValue.StrVal != "" {  // Line 821: Direct .StrVal access
        // read the expiration date, and check it against the current date
        expiration, err := time.Parse(time.RFC3339, expiresAtValue.StrVal)  // Line 823: Direct .StrVal access
        if err != nil {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("parse expiration date: %w", err)
        }
        if time.Now().After(expiration) {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("license expired on %s, please provide a valid license", expiration)
        }
    }
}
```

**Context**: License expiration validation during installation

**Impact**: Medium - Works for v1beta1, may break with v1beta2 entitlement structure

**Fix Required**: Use wrapper's abstraction method:
```go
entitlements := license.GetEntitlements()
if expiresAt, ok := entitlements["expires_at"]; ok {
    expiresAtValue := expiresAt.GetValue()
    valueInterface := expiresAtValue.Value()  // Use abstraction
    if expiresAtStr, ok := valueInterface.(string); ok && expiresAtStr != "" {
        expiration, err := time.Parse(time.RFC3339, expiresAtStr)
        if err != nil {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("parse expiration date: %w", err)
        }
        if time.Now().After(expiration) {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("license expired on %s, please provide a valid license", expiration)
        }
    }
}
```

**Alternative Fix** (if GetStringValue method exists):
```go
entitlements := license.GetEntitlements()
if expiresAt, ok := entitlements["expires_at"]; ok {
    if expiresAtStr := expiresAt.GetStringValue(); expiresAtStr != "" {
        expiration, err := time.Parse(time.RFC3339, expiresAtStr)
        if err != nil {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("parse expiration date: %w", err)
        }
        if time.Now().After(expiration) {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("license expired on %s, please provide a valid license", expiration)
        }
    }
}
```

---

#### 3.2 `api/pkg/template/license_test.go:50-69`

**Issue**: Test fixture using raw `kotsv1beta1.EntitlementField` type

```go
Entitlements: map[string]kotsv1beta1.EntitlementField{
    "maxNodes": {
        Value: kotsv1beta1.EntitlementValue{
            Type:   kotsv1beta1.String,
            StrVal: "10",
        },
    },
    // ... more entitlements
},
```

**Context**: Test fixtures

**Impact**: Low - Test code, but should follow best practices

**Fix Required**: Update tests to use LicenseWrapper constructors or test fixtures from testdata/

---

### 4. Summary by File

| Priority | File | Lines | Issue Type | Impact |
|----------|------|-------|------------|--------|
| 🔴 HIGH | `api/internal/managers/app/config/manager.go` | 37-38 | Duplicate field definition (merge conflict) | Compilation error |
| 🟠 MEDIUM | `cmd/installer/cli/install.go` | 818-831 | Direct entitlement value access (.StrVal) | v1beta2 compatibility risk |
| 🟠 MEDIUM | `cmd/installer/cli/upgrade.go` | 52, 129 | Old license type + direct field access | v1beta2 support missing |
| 🟠 MEDIUM | `api/internal/managers/linux/infra/upgrade.go` | 116, 142, 143, 265, 290 | Old license type + direct field access | v1beta2 support missing |
| 🟡 LOW | `cmd/installer/cli/install.go` | 139 | Direct field access (.Spec.AppSlug) | Minor - should use wrapper |
| 🟡 LOW | `api/pkg/template/license.go` | 151, 156, 157 | Direct field access (channel fields) | Minor - should use wrapper |
| 🟡 LOW | `api/pkg/template/license_test.go` | 50-69 | Test using raw EntitlementField | Best practice - tests should use wrapper |

---

## Code References

### Production Files Needing Updates

- `cmd/installer/cli/install.go:139` - Metrics reporter creation
- `cmd/installer/cli/install.go:818-831` - License expiration validation
- `cmd/installer/cli/upgrade.go:52` - upgradeConfig struct
- `cmd/installer/cli/upgrade.go:129` - Metrics reporter creation
- `api/pkg/template/license.go:151-157` - Channel name resolution
- `api/internal/managers/app/config/manager.go:37-38` - Duplicate field (CRITICAL)
- `api/internal/managers/linux/infra/upgrade.go:116` - License initialization
- `api/internal/managers/linux/infra/upgrade.go:142-143` - LicenseInfo population
- `api/internal/managers/linux/infra/upgrade.go:265` - License initialization
- `api/internal/managers/linux/infra/upgrade.go:290` - DistributeArtifacts call

### Test Files (Lower Priority)

- `cmd/installer/cli/install_test.go:252,259` - Test license creation
- `cmd/installer/cli/release_test.go:88` - Test license creation
- `api/pkg/template/execute_test.go:22,24` - Test helper functions
- `api/pkg/template/license_test.go:19,26,50-69,144,186,229,274,320,366,412,458,504` - Multiple test cases
- `api/internal/managers/linux/infra/install_test.go:61` - Test license creation
- `pkg/kubeutils/installation_test.go:213,254,289` - LicenseWrapper.V1 initialization

---

## Architecture Insights

### Migration Pattern Observed

The codebase is transitioning through three stages:

1. **Raw bytes** (`[]byte`) - Used for storage and transport
2. **Parsed wrapper** (`licensewrapper.LicenseWrapper`) - Used for business logic
3. **Accessor methods** (`.GetAppSlug()`, `.IsEmbeddedClusterDownloadEnabled()`) - Used for field access

### Where the Migration is Incomplete

1. **Upgrade command path** (`cmd/installer/cli/upgrade.go`) - Still uses old license type
2. **Infrastructure upgrade manager** (`api/internal/managers/linux/infra/upgrade.go`) - Still creates old license types
3. **Config manager** (`api/internal/managers/app/config/manager.go`) - Has conflicting definitions
4. **Direct field access scattered** - In 4 production files (9 instances total)

### Wrapper Method Availability

All the following wrapper methods are available and should be used:

| Method | Replaces |
|--------|----------|
| `GetAppSlug()` | `.Spec.AppSlug` |
| `GetLicenseID()` | `.Spec.LicenseID` |
| `GetCustomerName()` | `.Spec.CustomerName` |
| `GetChannelID()` | `.Spec.ChannelID` |
| `GetChannelName()` | `.Spec.ChannelName` |
| `IsEmbeddedClusterDownloadEnabled()` | `.Spec.IsEmbeddedClusterDownloadEnabled` |
| `IsEmbeddedClusterMultiNodeEnabled()` | `.Spec.IsEmbeddedClusterMultiNodeEnabled` |
| `IsDisasterRecoverySupported()` | `.Spec.IsDisasterRecoverySupported` |
| `GetChannels()` | `.Spec.Channels` (verify this exists) |

**Note**: If `GetChannels()` is not available in LicenseWrapper, it needs to be added to kotskinds.

---

## Test Coverage Status

### Test Fixtures

✅ **Complete** - Test fixtures exist for:
- `pkg/helpers/testdata/license-v1beta1.yaml` - v1beta1 license
- `pkg/helpers/testdata/license-v1beta2.yaml` - v1beta2 license
- `pkg/helpers/testdata/license-invalid-version.yaml` - Invalid version
- Multiple v1beta2 edge case fixtures

### Test Files

✅ **Complete** - Tests exist for:
- `pkg/helpers/parse_test.go` - ParseLicense with both versions
- `api/pkg/template/license_test.go` - Template functions with both versions
- Integration tests in multiple locations

⚠️ **Incomplete** - Tests need updating for:
- `cmd/installer/cli/upgrade.go` - Upgrade command with v1beta2 licenses
- `api/internal/managers/linux/infra/upgrade.go` - Upgrade manager with v1beta2

---

## Recommended Action Plan

### Phase 1: Critical Fixes (Must Do Before Merge)

1. **Resolve merge conflict** in `api/internal/managers/app/config/manager.go:37-38`
   - Remove duplicate `license *kotsv1beta1.License` field (line 38)
   - Keep only `license licensewrapper.LicenseWrapper` (line 37)
   - Verify all callers use wrapper methods

2. **Fix entitlement value access** in `cmd/installer/cli/install.go:818-831`
   - Replace direct `.StrVal` access with wrapper abstraction
   - Add error handling for type assertions

### Phase 2: Upgrade Command Support (High Priority)

3. **Update upgrade command** in `cmd/installer/cli/upgrade.go`
   - Change `upgradeConfig.license` type to `licensewrapper.LicenseWrapper` (line 52)
   - Replace direct field access (line 129) with wrapper methods
   - Add tests for upgrade with v1beta2 licenses

4. **Update upgrade manager** in `api/internal/managers/linux/infra/upgrade.go`
   - Replace `&kotsv1beta1.License{}` with wrapper parsing (lines 116, 265)
   - Replace direct field access with wrapper methods (lines 142, 143, 290)
   - Add tests for upgrade operations with v1beta2 licenses

### Phase 3: Minor Cleanup (Lower Priority)

5. **Fix remaining direct field access**:
   - `cmd/installer/cli/install.go:139` - Use `GetAppSlug()`
   - `api/pkg/template/license.go:151,156,157` - Use wrapper methods for channels

6. **Update test code** to use best practices:
   - `api/pkg/template/license_test.go:50-69` - Use LicenseWrapper fixtures
   - Other test files - Migrate to wrapper-based license creation

### Phase 4: Verification

7. **Run comprehensive tests**:
   ```bash
   go test ./... -v
   ```

8. **Manual verification**:
   - Test install with v1beta1 license
   - Test install with v1beta2 license
   - Test upgrade with v1beta1 license (NEW)
   - Test upgrade with v1beta2 license (NEW)
   - Verify license expiration validation works

---

## Open Questions

1. **Does `GetChannels()` method exist in LicenseWrapper?**
   - Used in `api/pkg/template/license.go:151`
   - If not, needs to be added to kotskinds repository

2. **Should test files use wrapper fixtures or raw license construction?**
   - Current practice is mixed
   - Recommend standardizing on wrapper-based fixtures

3. **Is there a helper method for typed entitlement access?**
   - E.g., `GetStringValue()`, `GetIntValue()`, `GetBoolValue()`
   - Would simplify the expiration validation code

---

## Related Research

- [docs/research/2025-10-29-license-v1beta2-support.md](2025-10-29-license-v1beta2-support.md) - Original v1beta2 support research
- Original implementation plan (provided in user's query)

---

## Conclusion

The merge from main introduced new code (particularly in upgrade paths) that was not covered in the original implementation plan. The most critical issue is a merge conflict with duplicate field definitions that must be resolved immediately. Additionally, the upgrade command and upgrade manager need to be updated to use LicenseWrapper for full v1beta2 support.

**Estimated effort**:
- Phase 1 (Critical): 2-3 hours
- Phase 2 (Upgrade support): 4-5 hours
- Phase 3 (Cleanup): 2-3 hours
- Phase 4 (Testing): 2-3 hours
- **Total**: 10-14 hours

**Risk level**: Medium - The duplicate field definition is a critical issue, but the fixes are straightforward and well-understood.
