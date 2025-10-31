# Complete LicenseWrapper Migration Implementation Plan

## Overview

Complete the LicenseWrapper migration for v1beta2 license support by updating upgrade paths and fixing remaining direct field access patterns that were introduced during the merge from main. This work ensures full v1beta1 and v1beta2 license compatibility across all code paths, including upgrade scenarios.

## Current State Analysis

After merging code from main into the `feature/crdant/supports-license-v1beta2` branch, a comprehensive audit identified remaining work:

### What Exists Now:
- ✅ Core parsing infrastructure uses `LicenseWrapper` (`pkg/helpers/parse.go`)
- ✅ Template engine fully migrated to `LicenseWrapper` (`api/pkg/template/`)
- ✅ Install command path fully migrated (mostly - see exceptions below)
- ✅ Test fixtures exist for both v1beta1 and v1beta2 licenses
- ✅ Config manager merge conflict has been resolved

### What's Missing:
- ❌ Upgrade command still uses old `*kotsv1beta1.License` type
- ❌ Upgrade infrastructure manager creates old license types
- ❌ 9 instances of direct `.Spec.*` field access across 4 files
- ❌ 1 critical instance of direct `.StrVal` entitlement value access
- ❌ Tests for upgrade scenarios with v1beta2 licenses

### Key Discoveries:

**File: `cmd/installer/cli/upgrade.go:52`**
```go
type upgradeConfig struct {
    license *kotsv1beta1.License  // Should be licensewrapper.LicenseWrapper
}
```
- Impact: Upgrade command cannot handle v1beta2 licenses
- Also has direct field access at line 129

**File: `api/internal/managers/linux/infra/upgrade.go:116,265`**
```go
license := &kotsv1beta1.License{}
if err := kyaml.Unmarshal(m.license, license); err != nil {
    return nil, fmt.Errorf("parse license: %w", err)
}
```
- Impact: Upgrade operations will fail with v1beta2 licenses
- Also has direct field access at lines 142, 143, 290

**File: `cmd/installer/cli/install.go:821,823`**
```go
if expiresAtValue.StrVal != "" {
    expiration, err := time.Parse(time.RFC3339, expiresAtValue.StrVal)
}
```
- Impact: Direct entitlement value access bypasses v1beta2 abstraction

**Wrapper Methods Available:**
- ✅ `GetAppSlug()` - replaces `.Spec.AppSlug`
- ✅ `GetLicenseID()` - replaces `.Spec.LicenseID`
- ✅ `GetChannelID()` - replaces `.Spec.ChannelID`
- ✅ `GetChannelName()` - replaces `.Spec.ChannelName`
- ✅ `GetChannels()` - replaces `.Spec.Channels`
- ✅ `IsDisasterRecoverySupported()` - replaces `.Spec.IsDisasterRecoverySupported`
- ✅ `IsEmbeddedClusterMultiNodeEnabled()` - replaces `.Spec.IsEmbeddedClusterMultiNodeEnabled`
- ✅ `Value()` on EntitlementValue - replaces direct `.StrVal`/`.IntVal`/`.BoolVal` access

## Desired End State

After this implementation is complete:

1. **All production code uses `LicenseWrapper`** - No `*kotsv1beta1.License` types in production structs or variables
2. **All field access uses wrapper methods** - No direct `.Spec.*` access patterns
3. **All entitlement access uses abstractions** - No direct `.StrVal`/`.IntVal`/`.BoolVal` access
4. **Upgrade scenarios work with both license versions** - Full test coverage for v1beta1 and v1beta2 upgrades
5. **Compilation succeeds** - `go build ./...` completes without errors
6. **All tests pass** - `go test ./...` succeeds including new upgrade tests

### Verification:
```bash
# No old license types in production code
! grep -r "\*kotsv1beta1.License" cmd/ pkg/ api/ --include="*.go" --exclude="*_test.go"

# No direct Spec access in production code
! grep -r "\.Spec\." cmd/ pkg/ api/ --include="*.go" --exclude="*_test.go" | grep -i license

# Build succeeds
go build ./...

# All tests pass
go test ./... -v

# Upgrade works with v1beta2
# Manual test: perform upgrade with v1beta2 license file
```

## What We're NOT Doing

- ❌ Not updating test files to use wrapper (test code can still use raw types for now)
- ❌ Not adding new v1beta2 features beyond parallel support
- ❌ Not deprecating v1beta1 license support
- ❌ Not changing the template function syntax or public APIs
- ❌ Not modifying the LicenseWrapper implementation in kotskinds
- ❌ Not changing license validation logic (only refactoring to use wrapper methods)

## Implementation Approach

**Strategy:** Incremental, file-by-file migration following the same pattern used in the original v1beta2 implementation:
1. Update struct field types from `*kotsv1beta1.License` to `licensewrapper.LicenseWrapper`
2. Replace license parsing from raw types to wrapper methods
3. Replace all direct `.Spec.*` field access with wrapper getter methods
4. Replace direct entitlement value access with abstraction methods
5. Add tests for both v1beta1 and v1beta2 scenarios
6. Verify compilation and test passage

**Risk Mitigation:**
- Each phase is independently testable
- Changes are localized to specific files
- Wrapper methods provide identical behavior for both versions
- Existing install path already validated through prior work

---

## Phase 1: Upgrade Command Migration

### Overview
Update the upgrade command to use `LicenseWrapper` instead of `*kotsv1beta1.License`, enabling support for both v1beta1 and v1beta2 licenses in upgrade scenarios.

### Changes Required:

#### 1.1 Update upgradeConfig Struct Type

**File**: `cmd/installer/cli/upgrade.go:52`

**Current Code:**
```go
type upgradeConfig struct {
    passwordHash         []byte
    tlsConfig            apitypes.TLSConfig
    tlsCert              tls.Certificate
    license              *kotsv1beta1.License  // Line 52
    licenseBytes         []byte
    // ... other fields
}
```

**Changes:**
```go
import (
    "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

type upgradeConfig struct {
    passwordHash         []byte
    tlsConfig            apitypes.TLSConfig
    tlsCert              tls.Certificate
    license              licensewrapper.LicenseWrapper  // Changed type
    licenseBytes         []byte
    // ... other fields
}
```

#### 1.2 Update License Parsing in prepareUpgrade()

**File**: `cmd/installer/cli/upgrade.go` (find the function that parses the license)

**Current Pattern** (need to locate exact line):
```go
license := &kotsv1beta1.License{}
if err := kyaml.Unmarshal(licenseBytes, license); err != nil {
    return nil, fmt.Errorf("unmarshal license: %w", err)
}
config.license = license
```

**Changes:**
```go
license, err := licensewrapper.LoadLicenseFromBytes(licenseBytes)
if err != nil {
    return nil, fmt.Errorf("parse license: %w", err)
}
config.license = license
```

#### 1.3 Fix Direct Field Access in Metrics Reporter

**File**: `cmd/installer/cli/upgrade.go:129`

**Current Code:**
```go
// Line 129
upgradeConfig.license.Spec.LicenseID, upgradeConfig.clusterID, upgradeConfig.license.Spec.AppSlug,
```

**Changes:**
```go
upgradeConfig.license.GetLicenseID(), upgradeConfig.clusterID, upgradeConfig.license.GetAppSlug(),
```

#### 1.4 Update All Other References

Search the file for any other uses of `upgradeConfig.license` and update them:

```bash
# Find all references
grep -n "upgradeConfig.license" cmd/installer/cli/upgrade.go
```

**Pattern to replace:**
- `.Spec.AppSlug` → `.GetAppSlug()`
- `.Spec.LicenseID` → `.GetLicenseID()`
- `.Spec.CustomerName` → `.GetCustomerName()`
- Any other `.Spec.*` → Corresponding wrapper method

### Success Criteria:

#### Automated Verification:
- [ ] File compiles without errors: `go build ./cmd/installer/cli/`
- [ ] No direct `.Spec` access on license in upgrade.go: `! grep "\.license\.Spec\." cmd/installer/cli/upgrade.go`
- [ ] No `*kotsv1beta1.License` type in upgradeConfig: `! grep "\*kotsv1beta1.License" cmd/installer/cli/upgrade.go`
- [ ] Existing upgrade tests still pass: `go test ./cmd/installer/cli -v -run TestUpgrade`

#### Manual Verification:
- [ ] Upgrade command accepts v1beta1 license file and completes successfully
- [ ] Upgrade command accepts v1beta2 license file and completes successfully
- [ ] Metrics are reported correctly with license ID and app slug
- [ ] Invalid license files are rejected with appropriate error messages

---

## Phase 2: Upgrade Manager Migration

### Overview
Update the infrastructure upgrade manager to parse licenses using `LicenseWrapper` and replace all direct field access with wrapper methods.

### Changes Required:

#### 2.1 Fix License Parsing in newInstallationObj()

**File**: `api/internal/managers/linux/infra/upgrade.go:116-119`

**Current Code:**
```go
// Line 116
license := &kotsv1beta1.License{}
if err := kyaml.Unmarshal(m.license, license); err != nil {
    return nil, fmt.Errorf("parse license: %w", err)
}
```

**Changes:**
```go
import (
    "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

license, err := licensewrapper.LoadLicenseFromBytes(m.license)
if err != nil {
    return nil, fmt.Errorf("parse license: %w", err)
}
```

#### 2.2 Fix Direct Field Access in LicenseInfo Population

**File**: `api/internal/managers/linux/infra/upgrade.go:141-144`

**Current Code:**
```go
in.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
    IsDisasterRecoverySupported: license.Spec.IsDisasterRecoverySupported,  // Line 142
    IsMultiNodeEnabled:          license.Spec.IsEmbeddedClusterMultiNodeEnabled,  // Line 143
}
```

**Changes:**
```go
in.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
    IsDisasterRecoverySupported: license.IsDisasterRecoverySupported(),
    IsMultiNodeEnabled:          license.IsEmbeddedClusterMultiNodeEnabled(),
}
```

#### 2.3 Fix Second License Parsing Instance

**File**: `api/internal/managers/linux/infra/upgrade.go:265` (in a different function)

**Current Code:**
```go
// Line 265
license := &kotsv1beta1.License{}
if err := kyaml.Unmarshal(m.license, license); err != nil {
    return nil, fmt.Errorf("parse license: %w", err)
}
```

**Changes:**
```go
license, err := licensewrapper.LoadLicenseFromBytes(m.license)
if err != nil {
    return nil, fmt.Errorf("parse license: %w", err)
}
```

#### 2.4 Fix Direct Field Access in DistributeArtifacts Call

**File**: `api/internal/managers/linux/infra/upgrade.go:290`

**Current Code:**
```go
// Line 290
return m.upgrader.DistributeArtifacts(ctx, in, localArtifactMirrorImage, license.Spec.LicenseID, appSlug, channelID, appVersion)
```

**Changes:**
```go
return m.upgrader.DistributeArtifacts(ctx, in, localArtifactMirrorImage, license.GetLicenseID(), appSlug, channelID, appVersion)
```

#### 2.5 Verify No Other Direct Access

Search for any remaining direct access patterns:

```bash
# Find all license.Spec references
grep -n "license\.Spec\." api/internal/managers/linux/infra/upgrade.go
```

### Success Criteria:

#### Automated Verification:
- [ ] File compiles without errors: `go build ./api/internal/managers/linux/infra/`
- [ ] No direct `.Spec` access in upgrade.go: `! grep "license\.Spec\." api/internal/managers/linux/infra/upgrade.go`
- [ ] No `&kotsv1beta1.License{}` initializations: `! grep "&kotsv1beta1.License{}" api/internal/managers/linux/infra/upgrade.go`
- [ ] Existing manager tests pass: `go test ./api/internal/managers/linux/infra -v`

#### Manual Verification:
- [ ] Upgrade manager processes v1beta1 licenses correctly
- [ ] Upgrade manager processes v1beta2 licenses correctly
- [ ] Installation objects have correct LicenseInfo populated
- [ ] Artifact distribution includes correct license ID

---

## Phase 3: Entitlement Access Refactoring

### Overview
Fix the direct entitlement value access in the license expiration check to use the abstraction method `Value()` instead of directly accessing `.StrVal`.

### Changes Required:

#### 3.1 Refactor License Expiration Validation

**File**: `cmd/installer/cli/install.go:818-831`

**Current Code:**
```go
entitlements := license.GetEntitlements()
if expiresAt, ok := entitlements["expires_at"]; ok {
    expiresAtValue := expiresAt.GetValue()
    if expiresAtValue.StrVal != "" {  // Line 821: Direct access
        // read the expiration date, and check it against the current date
        expiration, err := time.Parse(time.RFC3339, expiresAtValue.StrVal)  // Line 823: Direct access
        if err != nil {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("parse expiration date: %w", err)
        }
        if time.Now().After(expiration) {
            return licensewrapper.LicenseWrapper{}, fmt.Errorf("license expired on %s, please provide a valid license", expiration)
        }
    }
}
```

**Changes:**
```go
entitlements := license.GetEntitlements()
if expiresAt, ok := entitlements["expires_at"]; ok {
    expiresAtValue := expiresAt.GetValue()
    valueInterface := expiresAtValue.Value()  // Use abstraction method
    if expiresAtStr, ok := valueInterface.(string); ok && expiresAtStr != "" {
        // read the expiration date, and check it against the current date
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

**Explanation:**
- `Value()` returns `interface{}` containing the actual value
- Type assertion safely extracts the string value
- Works correctly for both v1beta1 and v1beta2 entitlement structures
- Handles nil/empty cases gracefully with the `ok` check

#### 3.2 Add Test for Expired License

**File**: `cmd/installer/cli/install_test.go`

**Add New Test:**
```go
func TestGetLicenseFromFilepath_ExpiredLicense(t *testing.T) {
    // Create a license with expires_at entitlement set to past date
    tmpfile, err := os.CreateTemp("", "license-expired-*.yaml")
    require.NoError(t, err)
    defer os.Remove(tmpfile.Name())

    // Create v1beta2 license with expired date
    expiredDate := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
    licenseData := fmt.Sprintf(`apiVersion: kots.io/v1beta2
kind: License
metadata:
  name: test-license
spec:
  appSlug: embedded-cluster-test
  licenseID: test-license-expired
  licenseType: dev
  customerName: Test Customer
  endpoint: https://replicated.app
  isEmbeddedClusterDownloadEnabled: true
  entitlements:
    expires_at:
      title: Expiration Date
      value:
        type: String
        strVal: %s
`, expiredDate)

    _, err = tmpfile.Write([]byte(licenseData))
    require.NoError(t, err)
    tmpfile.Close()

    _, err = getLicenseFromFilepath(tmpfile.Name())
    require.Error(t, err)
    assert.Contains(t, err.Error(), "license expired")
}

func TestGetLicenseFromFilepath_ValidExpiration(t *testing.T) {
    // Create a license with expires_at set to future date
    tmpfile, err := os.CreateTemp("", "license-valid-*.yaml")
    require.NoError(t, err)
    defer os.Remove(tmpfile.Name())

    futureDate := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
    licenseData := fmt.Sprintf(`apiVersion: kots.io/v1beta2
kind: License
metadata:
  name: test-license
spec:
  appSlug: embedded-cluster-test
  licenseID: test-license-valid
  licenseType: dev
  customerName: Test Customer
  endpoint: https://replicated.app
  isEmbeddedClusterDownloadEnabled: true
  entitlements:
    expires_at:
      title: Expiration Date
      value:
        type: String
        strVal: %s
`, futureDate)

    _, err = tmpfile.Write([]byte(licenseData))
    require.NoError(t, err)
    tmpfile.Close()

    license, err := getLicenseFromFilepath(tmpfile.Name())
    require.NoError(t, err)
    assert.Equal(t, "test-license-valid", license.GetLicenseID())
}
```

### Success Criteria:

#### Automated Verification:
- [ ] No direct `.StrVal` access in install.go: `! grep "\.StrVal" cmd/installer/cli/install.go | grep -v "// "`
- [ ] No direct `.IntVal` access in install.go: `! grep "\.IntVal" cmd/installer/cli/install.go | grep -v "// "`
- [ ] No direct `.BoolVal` access in install.go: `! grep "\.BoolVal" cmd/installer/cli/install.go | grep -v "// "`
- [ ] New tests pass: `go test ./cmd/installer/cli -v -run TestGetLicenseFromFilepath_Expired`
- [ ] New tests pass: `go test ./cmd/installer/cli -v -run TestGetLicenseFromFilepath_ValidExpiration`
- [ ] All install CLI tests pass: `go test ./cmd/installer/cli -v`

#### Manual Verification:
- [ ] License with future expires_at date is accepted
- [ ] License with past expires_at date is rejected with clear error message
- [ ] License without expires_at entitlement is accepted
- [ ] Works correctly with both v1beta1 and v1beta2 licenses

---

## Phase 4: Minor Cleanups

### Overview
Fix remaining low-priority direct field access patterns in install.go and template/license.go to complete the migration.

### Changes Required:

#### 4.1 Fix Metrics Reporter in Install Command

**File**: `cmd/installer/cli/install.go:139`

**Current Code:**
```go
// Line 139
installCfg.license.GetLicenseIO(), installCfg.clusterID, installCfg.license.Spec.AppSlug,
```

**Changes:**
```go
installCfg.license.GetLicenseIO(), installCfg.clusterID, installCfg.license.GetAppSlug(),
```

#### 4.2 Fix Channel Name Resolution in Template Engine

**File**: `api/pkg/template/license.go:151-157`

**Current Code:**
```go
// Line 151
for _, channel := range e.license.Spec.Channels {
    if channel.ID == e.releaseData.ChannelRelease.ChannelID {
        return channel.Name, nil
    }
}
// Line 156
if e.license.Spec.ChannelID == e.releaseData.ChannelRelease.ChannelID {
    // Line 157
    return e.license.Spec.ChannelName, nil
}
```

**Changes:**
```go
for _, channel := range e.license.GetChannels() {
    if channel.ID == e.releaseData.ChannelRelease.ChannelID {
        return channel.Name, nil
    }
}
if e.license.GetChannelID() == e.releaseData.ChannelRelease.ChannelID {
    return e.license.GetChannelName(), nil
}
```

#### 4.3 Verify Complete Migration

Run comprehensive search to ensure no remaining direct access:

```bash
# Search all production Go files
grep -r "\.Spec\." cmd/ api/ pkg/ --include="*.go" --exclude="*_test.go" | grep -i license

# Should return no results except for comments
```

### Success Criteria:

#### Automated Verification:
- [ ] No direct `.Spec` access in production code: `! grep -r "\.Spec\." cmd/ api/ pkg/ --include="*.go" --exclude="*_test.go" | grep license | grep -v "//"`
- [ ] Install command compiles: `go build ./cmd/installer/cli/`
- [ ] Template engine compiles: `go build ./api/pkg/template/`
- [ ] All template tests pass: `go test ./api/pkg/template -v`
- [ ] All CLI tests pass: `go test ./cmd/installer/cli -v`

#### Manual Verification:
- [ ] Channel name resolution works in templates with both license versions
- [ ] Metrics reporting includes correct app slug
- [ ] No behavioral changes observed in install flow

---

## Phase 5: Comprehensive Testing

### Overview
Verify the complete migration through automated tests and manual validation with both v1beta1 and v1beta2 licenses across install and upgrade scenarios.

### Testing Activities:

#### 5.1 Run Full Test Suite

```bash
# Run all tests with verbose output
go test ./... -v

# Run tests with race detector
go test ./... -race

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

#### 5.2 Verify Build Success

```bash
# Build all binaries
go build ./...

# Build specific commands
go build ./cmd/installer
go build ./cmd/operator
```

#### 5.3 Static Analysis

```bash
# Run linter
golangci-lint run

# Check for direct license field access
grep -r "\.Spec\." cmd/ api/ pkg/ --include="*.go" --exclude="*_test.go" | grep license

# Should only find commented examples or no results
```

### Success Criteria:

#### Automated Verification:
- [ ] All tests pass: `go test ./... -v`
- [ ] No race conditions: `go test ./... -race`
- [ ] Build succeeds: `go build ./...`
- [ ] No linter errors: `golangci-lint run`
- [ ] Code coverage maintained or improved: `go test ./... -coverprofile=coverage.out`
- [ ] No direct `.Spec` access: `! grep -r "\.Spec\." cmd/ api/ pkg/ --include="*.go" --exclude="*_test.go" | grep license | grep -v "//"`
- [ ] No old license types: `! grep -r "\*kotsv1beta1.License" cmd/ api/ pkg/ --include="*.go" --exclude="*_test.go"`

#### Manual Verification:

**Install Scenarios:**
- [ ] Install with v1beta1 license succeeds
- [ ] Install with v1beta2 license succeeds
- [ ] Install with expired license (expires_at entitlement) is rejected
- [ ] Install with license missing embedded cluster enablement is rejected
- [ ] Metrics are reported with correct license ID and app slug
- [ ] Template functions access license fields correctly

**Upgrade Scenarios:**
- [ ] Upgrade with v1beta1 license succeeds
- [ ] Upgrade with v1beta2 license succeeds
- [ ] Upgrade preserves license information in Installation CRD
- [ ] Upgrade metrics include correct license details
- [ ] Upgraded cluster functions normally after upgrade

**Template Rendering:**
- [ ] License template functions work with v1beta1 licenses
- [ ] License template functions work with v1beta2 licenses
- [ ] Channel name resolution works correctly
- [ ] Entitlement values accessible in templates
- [ ] Docker config generation includes license credentials

**Error Handling:**
- [ ] Invalid license version (e.g., v1beta3) is rejected with clear error
- [ ] Malformed license YAML produces helpful error message
- [ ] Missing required fields produce specific error messages

---

## Testing Strategy

### Unit Tests

**Existing Tests to Verify:**
- `pkg/helpers/parse_test.go` - License parsing with both versions
- `api/pkg/template/license_test.go` - Template functions
- `cmd/installer/cli/install_test.go` - Install command validation

**New Tests to Add:**
- License expiration validation (v1beta1 and v1beta2)
- Upgrade command with v1beta2 licenses
- Upgrade manager with v1beta2 licenses

**Key Edge Cases:**
- Empty/nil license
- License without expires_at entitlement
- License with invalid expires_at format
- License without embedded cluster enablement
- License with missing app slug
- Invalid license version

### Integration Tests

**Scenarios to Test:**
1. **Fresh install with v1beta1 license**
   - Verify installation succeeds
   - Check metrics reporting
   - Validate template rendering

2. **Fresh install with v1beta2 license**
   - Verify installation succeeds
   - Check metrics reporting
   - Validate template rendering
   - Verify entitlements accessible

3. **Upgrade existing installation with v1beta1 license**
   - Verify upgrade succeeds
   - Check Installation CRD update
   - Validate license info preservation

4. **Upgrade existing installation with v1beta2 license**
   - Verify upgrade succeeds
   - Check Installation CRD update
   - Validate license info preservation

### Manual Testing Steps

1. **Prepare Test Licenses:**
   ```bash
   # Create v1beta1 license file
   cat > license-v1.yaml <<EOF
   apiVersion: kots.io/v1beta1
   kind: License
   # ... complete license content
   EOF

   # Create v1beta2 license file
   cat > license-v2.yaml <<EOF
   apiVersion: kots.io/v1beta2
   kind: License
   # ... complete license content with entitlements
   EOF
   ```

2. **Test Install Flow:**
   ```bash
   # Install with v1beta1
   ./embedded-cluster install --license license-v1.yaml

   # Clean up and install with v1beta2
   ./embedded-cluster install --license license-v2.yaml
   ```

3. **Test Upgrade Flow:**
   ```bash
   # Perform upgrade with v1beta1
   ./embedded-cluster upgrade --license license-v1.yaml

   # Test upgrade with v1beta2
   ./embedded-cluster upgrade --license license-v2.yaml
   ```

4. **Verify License Information:**
   ```bash
   # Check Installation CRD
   kubectl get installation -n embedded-cluster -o yaml

   # Verify license info fields populated
   # - isDisasterRecoverySupported
   # - isMultiNodeEnabled
   ```

5. **Test Template Rendering:**
   - Deploy an application that uses license template functions
   - Verify license fields accessible: `{{repl LicenseFieldValue "fieldname"}}`
   - Check channel name resolution works
   - Validate entitlement access

## Performance Considerations

**No significant performance impact expected:**
- Wrapper methods are lightweight accessors
- No additional parsing or transformation overhead
- License objects created once per operation
- Memory footprint identical (wrapper is struct with pointers)

**Potential micro-optimizations (not required):**
- License parsing happens once at command entry point
- Wrapper methods are inline-able by Go compiler
- No reflection or dynamic dispatch involved

## Migration Notes

**Backwards Compatibility:**
- All existing v1beta1 licenses continue to work unchanged
- No database migrations required (licenses stored as bytes)
- No API changes visible to users
- Template syntax remains identical

**Rollback Plan:**
If issues are discovered after merge:
1. Revert the feature branch merge
2. Original main branch state is preserved
3. No data migration needed (licenses are read-only in this context)

**Deployment:**
- Can be deployed without coordination (stateless change)
- No rolling restart requirements
- Works with existing licenses in-flight

## References

- **Original Research:** `docs/research/2025-10-31-license-wrapper-remaining-work.md`
- **Original v1beta2 Research:** `docs/research/2025-10-29-license-v1beta2-support.md`
- **kotskinds Repository:** https://github.com/replicatedhq/kotskinds
- **LicenseWrapper Implementation:** `pkg/licensewrapper/licensewrapper.go` in kotskinds (commit 174e89c93554)
- **Test Fixtures:**
  - `pkg/helpers/testdata/license-v1beta1.yaml`
  - `pkg/helpers/testdata/license-v1beta2.yaml`

## Estimated Effort

**Total: 6-8 hours** (assuming no unexpected issues)

- **Phase 1** (Upgrade Command): 1.5-2 hours
  - Update struct and parsing: 30 min
  - Fix field access: 30 min
  - Testing: 30-60 min

- **Phase 2** (Upgrade Manager): 2-2.5 hours
  - Fix license parsing (2 locations): 45 min
  - Update field access: 45 min
  - Testing: 30-60 min

- **Phase 3** (Entitlement Access): 1-1.5 hours
  - Refactor expiration check: 30 min
  - Add tests: 30-60 min

- **Phase 4** (Minor Cleanups): 30-45 min
  - Fix remaining field access: 15-30 min
  - Verification: 15 min

- **Phase 5** (Comprehensive Testing): 1-1.5 hours
  - Automated tests: 30 min
  - Manual testing: 30-60 min

**Parallelization Opportunities:**
- Phases 1 and 2 can be worked in parallel (different files)
- Phase 3 can start once Phase 1 is complete
- Phase 4 cleanup can happen alongside Phase 5 testing

## Risk Assessment

**Risk Level: LOW**

**Risks:**
1. **Missing wrapper methods** - MITIGATED: All needed methods verified to exist
2. **Type conversion issues** - MITIGATED: Pattern already proven in install path
3. **Test failures** - MITIGATED: Incremental testing after each phase
4. **Merge conflict already resolved** - One critical issue already fixed

**Mitigation Strategies:**
- Incremental changes with testing after each phase
- Following established patterns from install path
- Comprehensive automated test coverage
- Manual testing with both license versions
