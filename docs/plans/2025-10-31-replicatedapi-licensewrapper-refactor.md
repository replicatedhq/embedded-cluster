# Comprehensive Plan: Refactor ReplicatedAPI Client for LicenseWrapper Support

**Date**: 2025-10-31
**Author**: Claude Code
**Status**: Ready for Implementation
**Related**: [2025-10-29-license-v1beta2-support.md](../research/2025-10-29-license-v1beta2-support.md)

## Executive Summary

The ReplicatedAPI client is the **final critical component** that needs refactoring to fully support v1beta2 licenses. Currently, it extracts the `.V1` field from LicenseWrapper (line 28 in `replicatedapi.go`), which will be `nil` for v1beta2-only licenses, causing license sync to fail.

**Scope**: 3 files, ~150 lines of changes
**Estimated Time**: 2-3 hours
**Risk Level**: Medium (affects license syncing functionality)

---

## Current State Analysis

### Problem Areas

#### 1. **pkg-new/replicatedapi/client.go** (Primary Issue)

**Current Implementation:**
```go
type client struct {
    license *kotsv1beta1.License  // Line 26 - WRONG TYPE
    // ...
}

func NewClient(..., license *kotsv1beta1.License, ...) // Line 46 - WRONG TYPE
```

**Direct .Spec Access (9 locations):**
- Line 64: `c.license.Spec.AppSlug` - API URL construction
- Line 67: `c.license.Spec.LicenseSequence` - Query parameter
- Line 101: `licenseResp.Spec.LicenseID` - Validation
- Line 128: `c.license.Spec.LicenseID` - Auth header (2x)
- Line 138: `c.license.Spec.LicenseID` - Validation
- Line 141: `c.license.Spec.Channels` - Channel iteration
- Line 146: `c.license.Spec.ChannelID` - Channel matching
- Lines 148-149: `c.license.Spec.ChannelID`, `c.license.Spec.ChannelName` - Fallback

**Interface Definition:**
```go
type Client interface {
    SyncLicense(ctx context.Context) (*kotsv1beta1.License, []byte, error)  // Line 21 - RETURNS WRONG TYPE
}
```

#### 2. **cmd/installer/cli/replicatedapi.go** (Workaround)

**Current Workaround (Lines 24-28):**
```go
func newReplicatedAPIClient(license licensewrapper.LicenseWrapper, clusterID string) (replicatedapi.Client, error) {
    // Extract the underlying v1beta1 license for the API client
    // The API client only supports v1beta1 licenses
    // For v1beta2 licenses, we use the V1 field which contains the converted v1beta1 representation
    underlyingLicense := license.V1  // ⚠️ WILL BE NIL FOR v1beta2-ONLY LICENSES
```

**Current Return Wrapping (Line 47, 55):**
```go
newSeq := updatedLicense.Spec.LicenseSequence  // Line 47 - Direct .Spec access
return licensewrapper.LicenseWrapper{V1: updatedLicense}, licenseBytes, nil  // Line 55 - Manual wrapping
```

#### 3. **pkg-new/replicatedapi/client_test.go** (Needs Updates)

All tests use `kotsv1beta1.License` directly:
- Line 21: Test struct field type
- Lines 29-43: Test license creation
- Lines 89-111: Expected license assertions
- Lines 247-249: Direct `.Spec` field comparisons

### Why This Matters

**Critical Impact:**
- License syncing **will fail completely** for v1beta2-only licenses
- `license.V1` will be `nil` for v1beta2 licenses that haven't been converted
- Any license sync attempt will panic or fail validation

**User-Facing Impact:**
- Customers with v1beta2 licenses won't be able to sync license updates
- Install/upgrade with license sync enabled will fail
- No way to update entitlements without manual license file replacement

---

## Implementation Plan

### Phase 1: Update Client Interface and Types (30 min)

**File**: `pkg-new/replicatedapi/client.go`

#### Step 1.1: Update imports

```go
import (
    // ... existing imports
    "github.com/replicatedhq/kotskinds/pkg/licensewrapper"  // ADD THIS
    kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)
```

#### Step 1.2: Update Client interface

**Before:**
```go
type Client interface {
    SyncLicense(ctx context.Context) (*kotsv1beta1.License, []byte, error)
}
```

**After:**
```go
type Client interface {
    SyncLicense(ctx context.Context) (licensewrapper.LicenseWrapper, []byte, error)
}
```

#### Step 1.3: Update client struct

**Before:**
```go
type client struct {
    replicatedAppURL string
    license          *kotsv1beta1.License  // Line 26
    releaseData      *release.ReleaseData
    clusterID        string
    httpClient       *retryablehttp.Client
}
```

**After:**
```go
type client struct {
    replicatedAppURL string
    license          licensewrapper.LicenseWrapper  // CHANGED TYPE
    releaseData      *release.ReleaseData
    clusterID        string
    httpClient       *retryablehttp.Client
}
```

#### Step 1.4: Update NewClient signature

**Before:**
```go
func NewClient(replicatedAppURL string, license *kotsv1beta1.License, releaseData *release.ReleaseData, opts ...ClientOption) (Client, error) {
```

**After:**
```go
func NewClient(replicatedAppURL string, license licensewrapper.LicenseWrapper, releaseData *release.ReleaseData, opts ...ClientOption) (Client, error) {
```

---

### Phase 2: Update License Field Access (45 min)

**File**: `pkg-new/replicatedapi/client.go`

Replace all direct `.Spec` field access with wrapper methods:

#### Change 2.1: SyncLicense() function (Lines 63-112)

**Before:**
```go
func (c *client) SyncLicense(ctx context.Context) (*kotsv1beta1.License, []byte, error) {
    u := fmt.Sprintf("%s/license/%s", c.replicatedAppURL, c.license.Spec.AppSlug)  // Line 64

    params := url.Values{}
    params.Set("licenseSequence", fmt.Sprintf("%d", c.license.Spec.LicenseSequence))  // Line 67
    // ...

    var licenseResp kotsv1beta1.License
    if err := kyaml.Unmarshal(body, &licenseResp); err != nil {
        return nil, nil, fmt.Errorf("unmarshal license response: %w", err)
    }

    if licenseResp.Spec.LicenseID == "" {  // Line 101
        return nil, nil, fmt.Errorf("license is empty")
    }

    c.license = &licenseResp  // Line 105

    // ...
    return &licenseResp, body, nil  // Line 111
}
```

**After:**
```go
func (c *client) SyncLicense(ctx context.Context) (licensewrapper.LicenseWrapper, []byte, error) {
    u := fmt.Sprintf("%s/license/%s", c.replicatedAppURL, c.license.GetAppSlug())  // Use wrapper method

    params := url.Values{}
    params.Set("licenseSequence", fmt.Sprintf("%d", c.license.GetLicenseSequence()))  // Use wrapper method
    // ...

    // Parse response into wrapper (handles both v1beta1 and v1beta2 responses)
    licenseWrapper, err := licensewrapper.LoadLicenseFromBytes(body)
    if err != nil {
        return licensewrapper.LicenseWrapper{}, nil, fmt.Errorf("parse license response: %w", err)
    }

    if licenseWrapper.GetLicenseID() == "" {  // Use wrapper method
        return licensewrapper.LicenseWrapper{}, nil, fmt.Errorf("license is empty")
    }

    c.license = licenseWrapper  // Store wrapper

    // ...
    return licenseWrapper, body, nil  // Return wrapper
}
```

#### Change 2.2: injectHeaders() function (Lines 126-132)

**Before:**
```go
func (c *client) injectHeaders(header http.Header) {
    header.Set("Authorization", "Basic "+basicAuth(c.license.Spec.LicenseID, c.license.Spec.LicenseID))  // Line 128
    header.Set("User-Agent", fmt.Sprintf("Embedded-Cluster/%s", versions.Version))

    c.injectReportingInfoHeaders(header)
}
```

**After:**
```go
func (c *client) injectHeaders(header http.Header) {
    licenseID := c.license.GetLicenseID()  // Use wrapper method
    header.Set("Authorization", "Basic "+basicAuth(licenseID, licenseID))
    header.Set("User-Agent", fmt.Sprintf("Embedded-Cluster/%s", versions.Version))

    c.injectReportingInfoHeaders(header)
}
```

#### Change 2.3: getChannelFromLicense() function (Lines 134-153)

**Before:**
```go
func (c *client) getChannelFromLicense() (*kotsv1beta1.Channel, error) {
    if c.releaseData == nil || c.releaseData.ChannelRelease == nil || c.releaseData.ChannelRelease.ChannelID == "" {
        return nil, fmt.Errorf("channel release is empty")
    }
    if c.license == nil || c.license.Spec.LicenseID == "" {  // Line 138
        return nil, fmt.Errorf("license is empty")
    }
    for _, channel := range c.license.Spec.Channels {  // Line 141
        if channel.ChannelID == c.releaseData.ChannelRelease.ChannelID {
            return &channel, nil
        }
    }
    if c.license.Spec.ChannelID == c.releaseData.ChannelRelease.ChannelID {  // Line 146
        return &kotsv1beta1.Channel{
            ChannelID:   c.license.Spec.ChannelID,  // Line 148
            ChannelName: c.license.Spec.ChannelName,  // Line 149
        }, nil
    }
    return nil, fmt.Errorf("channel %s not found in license", c.releaseData.ChannelRelease.ChannelID)
}
```

**After:**
```go
func (c *client) getChannelFromLicense() (*kotsv1beta1.Channel, error) {
    if c.releaseData == nil || c.releaseData.ChannelRelease == nil || c.releaseData.ChannelRelease.ChannelID == "" {
        return nil, fmt.Errorf("channel release is empty")
    }
    if c.license.GetLicenseID() == "" {  // Use wrapper method
        return nil, fmt.Errorf("license is empty")
    }

    // Check multi-channel licenses first
    channels := c.license.GetChannels()  // Use wrapper method
    for _, channel := range channels {
        if channel.ChannelID == c.releaseData.ChannelRelease.ChannelID {
            return &channel, nil
        }
    }

    // Fallback to legacy single-channel license
    if c.license.GetChannelID() == c.releaseData.ChannelRelease.ChannelID {  // Use wrapper method
        return &kotsv1beta1.Channel{
            ChannelID:   c.license.GetChannelID(),  // Use wrapper method
            ChannelName: c.license.GetChannelName(),  // Use wrapper method
        }, nil
    }

    return nil, fmt.Errorf("channel %s not found in license", c.releaseData.ChannelRelease.ChannelID)
}
```

---

### Phase 3: Remove Workaround from CLI (15 min)

**File**: `cmd/installer/cli/replicatedapi.go`

#### Change 3.1: Update newReplicatedAPIClient

**Before:**
```go
func newReplicatedAPIClient(license licensewrapper.LicenseWrapper, clusterID string) (replicatedapi.Client, error) {
    // Extract the underlying v1beta1 license for the API client
    // The API client only supports v1beta1 licenses
    // For v1beta2 licenses, we use the V1 field which contains the converted v1beta1 representation
    underlyingLicense := license.V1  // ⚠️ Line 28 - BREAKS FOR v1beta2

    return replicatedapi.NewClient(
        replicatedAppURL(),
        underlyingLicense,  // Passing raw v1beta1 license
        release.GetReleaseData(),
        replicatedapi.WithClusterID(clusterID),
    )
}
```

**After:**
```go
func newReplicatedAPIClient(license licensewrapper.LicenseWrapper, clusterID string) (replicatedapi.Client, error) {
    // Pass the wrapper directly - the API client now handles both v1beta1 and v1beta2
    return replicatedapi.NewClient(
        replicatedAppURL(),
        license,  // Pass wrapper directly
        release.GetReleaseData(),
        replicatedapi.WithClusterID(clusterID),
    )
}
```

#### Change 3.2: Update syncLicense return handling

**Before:**
```go
func syncLicense(ctx context.Context, client replicatedapi.Client, license licensewrapper.LicenseWrapper) (licensewrapper.LicenseWrapper, []byte, error) {
    logrus.Debug("Syncing license")

    updatedLicense, licenseBytes, err := client.SyncLicense(ctx)
    if err != nil {
        return licensewrapper.LicenseWrapper{}, nil, fmt.Errorf("get latest license: %w", err)
    }

    oldSeq := license.GetLicenseSequence()
    newSeq := updatedLicense.Spec.LicenseSequence  // Line 47 - Direct .Spec access
    if newSeq != oldSeq {
        logrus.Debugf("License synced successfully (sequence %d -> %d)", oldSeq, newSeq)
    } else {
        logrus.Debug("License is already up to date")
    }

    // Wrap the updated license - it comes back as v1beta1
    return licensewrapper.LicenseWrapper{V1: updatedLicense}, licenseBytes, nil  // Line 55 - Manual wrapping
}
```

**After:**
```go
func syncLicense(ctx context.Context, client replicatedapi.Client, license licensewrapper.LicenseWrapper) (licensewrapper.LicenseWrapper, []byte, error) {
    logrus.Debug("Syncing license")

    updatedLicense, licenseBytes, err := client.SyncLicense(ctx)
    if err != nil {
        return licensewrapper.LicenseWrapper{}, nil, fmt.Errorf("get latest license: %w", err)
    }

    oldSeq := license.GetLicenseSequence()
    newSeq := updatedLicense.GetLicenseSequence()  // Use wrapper method
    if newSeq != oldSeq {
        logrus.Debugf("License synced successfully (sequence %d -> %d)", oldSeq, newSeq)
    } else {
        logrus.Debug("License is already up to date")
    }

    // Return wrapper directly - already wrapped by SyncLicense
    return updatedLicense, licenseBytes, nil
}
```

---

### Phase 4: Update Tests (45 min)

**File**: `pkg-new/replicatedapi/client_test.go`

#### Change 4.1: Add v1beta2 test case

Add new test case to `TestSyncLicense`:

```go
{
    name: "successful license sync with v1beta2",
    license: kotsv1beta1.License{  // Start with v1beta1
        Spec: kotsv1beta1.LicenseSpec{
            AppSlug:         "test-app",
            LicenseID:       "test-license-id",
            LicenseSequence: 5,
            ChannelID:       "test-channel-123",
            ChannelName:     "Stable",
            Channels: []kotsv1beta1.Channel{
                {
                    ChannelID:   "test-channel-123",
                    ChannelName: "Stable",
                },
            },
        },
    },
    releaseData: &release.ReleaseData{
        ChannelRelease: &release.ChannelRelease{
            ChannelID: "test-channel-123",
        },
    },
    serverHandler: func(t *testing.T) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            // Return v1beta2 license
            resp := `apiVersion: kots.io/v1beta2
kind: License
spec:
  licenseID: test-license-id
  appSlug: test-app
  licenseSequence: 6
  customerName: Test Customer
  channelID: test-channel-123
  channels:
    - channelID: test-channel-123
      channelName: Stable`

            w.WriteHeader(http.StatusOK)
            w.Write([]byte(resp))
        }
    },
    wantLicenseSequence: 6,  // Updated assertion strategy
    wantIsV2: true,
},
```

#### Change 4.2: Update test structure and assertions

**Before:**
```go
tests := []struct {
    name            string
    license         kotsv1beta1.License
    releaseData     *release.ReleaseData
    serverHandler   func(t *testing.T) http.HandlerFunc
    expectedLicense *kotsv1beta1.License  // Line 24 - WRONG TYPE
    wantErr         string
}
```

**After:**
```go
tests := []struct {
    name                string
    license             kotsv1beta1.License  // Input still v1beta1 for compatibility
    releaseData         *release.ReleaseData
    serverHandler       func(t *testing.T) http.HandlerFunc
    wantLicenseSequence int64  // Assert on sequence instead of full license
    wantAppSlug         string
    wantLicenseID       string
    wantIsV1            bool
    wantIsV2            bool
    wantErr             string
}
```

#### Change 4.3: Update test execution

**Before:**
```go
// Create client with v1beta1 license directly
client, err := NewClient(server.URL, &tt.license, tt.releaseData)
require.NoError(t, err)

// Execute sync
license, licenseBytes, err := client.SyncLicense(context.Background())

// Assert on full license struct
if tt.expectedLicense != nil {
    require.NotNil(t, license)
    assert.Equal(t, tt.expectedLicense.Spec.AppSlug, license.Spec.AppSlug)  // Line 247
    assert.Equal(t, tt.expectedLicense.Spec.LicenseID, license.Spec.LicenseID)  // Line 248
    assert.Equal(t, tt.expectedLicense.Spec.LicenseSequence, license.Spec.LicenseSequence)  // Line 249
}
```

**After:**
```go
// Wrap the v1beta1 license first
wrapper := licensewrapper.LicenseWrapper{V1: &tt.license}

// Create client with wrapper
client, err := NewClient(server.URL, wrapper, tt.releaseData)
require.NoError(t, err)

// Execute sync
license, licenseBytes, err := client.SyncLicense(context.Background())

// Assert using wrapper methods
if tt.wantErr == "" {
    require.NoError(t, err)
    assert.NotNil(t, licenseBytes)

    // Assert on wrapper methods (works for both v1beta1 and v1beta2)
    assert.Equal(t, tt.wantLicenseSequence, license.GetLicenseSequence())
    assert.Equal(t, tt.wantAppSlug, license.GetAppSlug())
    assert.Equal(t, tt.wantLicenseID, license.GetLicenseID())

    // Assert version
    if tt.wantIsV1 {
        assert.True(t, license.IsV1())
        assert.False(t, license.IsV2())
    }
    if tt.wantIsV2 {
        assert.False(t, license.IsV1())
        assert.True(t, license.IsV2())
    }
}
```

---

## Testing Strategy

### Unit Tests (Required)

1. **Test SyncLicense with v1beta1 response**
   - Verify wrapper correctly wraps v1beta1 response
   - Verify all fields accessible via wrapper methods

2. **Test SyncLicense with v1beta2 response**
   - Verify wrapper correctly wraps v1beta2 response
   - Verify all fields accessible via wrapper methods
   - Verify `.V1` is nil and `.V2` is populated

3. **Test getChannelFromLicense with both versions**
   - Multi-channel license (both versions)
   - Single-channel legacy license (v1beta1 only)

4. **Test with empty/nil licenses**
   - Verify proper error handling

### Integration Tests (Manual)

1. **Install with v1beta1 license + sync enabled**
   ```bash
   ./embedded-cluster install --license license-v1beta1.yaml
   ```

2. **Install with v1beta2 license + sync enabled**
   ```bash
   ./embedded-cluster install --license license-v1beta2.yaml
   ```

3. **Upgrade with license sync**
   ```bash
   ./embedded-cluster upgrade --license license-v1beta2.yaml
   ```

4. **Verify license update from v1beta1 → v1beta2**
   - Start with v1beta1 license
   - Server returns v1beta2 license
   - Verify sync succeeds and uses v1beta2

---

## Risk Assessment

### High Risks

1. **Breaking license sync for existing installations**
   - **Mitigation**: Maintain backward compatibility with v1beta1
   - **Mitigation**: Comprehensive test coverage for both versions
   - **Mitigation**: Test with real licenses from vendor portal

2. **API response format changes**
   - **Mitigation**: Server always returns v1beta1 currently (check with vendor team)
   - **Mitigation**: Handle both v1beta1 and v1beta2 responses

### Medium Risks

1. **Channel matching logic changes**
   - **Mitigation**: Keep same logic, just use wrapper methods
   - **Mitigation**: Test multi-channel and single-channel licenses

2. **Auth header construction**
   - **Mitigation**: LicenseID is same field in both versions
   - **Mitigation**: Test auth header is correct format

### Low Risks

1. **Test flakiness**
   - **Mitigation**: Use table-driven tests
   - **Mitigation**: Clear test fixtures

---

## Success Criteria

### Functional

- [ ] Install with v1beta1 license + sync: SUCCESS
- [ ] Install with v1beta2 license + sync: SUCCESS
- [ ] Upgrade with v1beta1 license + sync: SUCCESS
- [ ] Upgrade with v1beta2 license + sync: SUCCESS
- [ ] License sequence increments correctly
- [ ] Auth headers constructed correctly
- [ ] Channel matching works for both versions

### Code Quality

- [ ] No direct `.Spec.*` field access in replicatedapi package
- [ ] All license access through wrapper methods
- [ ] Client interface uses LicenseWrapper
- [ ] No `license.V1` extraction in CLI code

### Testing

- [ ] All unit tests pass
- [ ] New v1beta2 test cases added
- [ ] Test coverage maintained or improved
- [ ] Manual integration tests pass

---

## Implementation Checklist

### Phase 1: Client Interface (30 min)
- [ ] Add licensewrapper import
- [ ] Update Client interface return type
- [ ] Update client struct field type
- [ ] Update NewClient parameter type
- [ ] Verify compilation

### Phase 2: Field Access (45 min)
- [ ] Update SyncLicense() - use wrapper methods (9 changes)
- [ ] Update injectHeaders() - use wrapper methods (2 changes)
- [ ] Update getChannelFromLicense() - use wrapper methods (5 changes)
- [ ] Remove all direct `.Spec.*` access
- [ ] Verify compilation

### Phase 3: CLI Workaround Removal (15 min)
- [ ] Remove `.V1` extraction in newReplicatedAPIClient
- [ ] Update syncLicense to use wrapper methods
- [ ] Remove manual wrapper construction
- [ ] Verify compilation

### Phase 4: Tests (45 min)
- [ ] Add v1beta2 test case
- [ ] Update test struct to use assertions not full license
- [ ] Wrap test licenses in LicenseWrapper
- [ ] Update assertions to use wrapper methods
- [ ] Run tests: `go test ./pkg-new/replicatedapi -v`

### Phase 5: Integration Testing (30 min)
- [ ] Create test v1beta1 license file
- [ ] Create test v1beta2 license file
- [ ] Test install with v1beta1 + sync
- [ ] Test install with v1beta2 + sync
- [ ] Test upgrade with license sync
- [ ] Verify logs show correct sequence numbers

---

## Estimated Timeline

| Phase | Time | Cumulative |
|-------|------|------------|
| Phase 1: Client Interface | 30 min | 30 min |
| Phase 2: Field Access | 45 min | 1h 15min |
| Phase 3: CLI Workaround | 15 min | 1h 30min |
| Phase 4: Tests | 45 min | 2h 15min |
| Phase 5: Integration Testing | 30 min | 2h 45min |
| **Buffer** | 15 min | **3h total** |

---

## Rollback Plan

If issues are discovered:

1. **Immediate**: Revert commits (single PR)
2. **Short-term**: Add feature flag to disable license sync
3. **Long-term**: Fix issues and re-deploy

**Rollback Command:**
```bash
git revert <commit-sha>
git push
```

---

## Next Steps

1. **Review this plan** - Any questions or concerns?
2. **Create feature branch**: `git checkout -b feature/replicatedapi-licensewrapper`
3. **Start with Phase 1** - Update types and interface
4. **Test incrementally** - Run tests after each phase
5. **Create PR when complete** - Include all test results

---

## Related Documentation

- [License v1beta2 Support Research](../research/2025-10-29-license-v1beta2-support.md)
- [License v1beta2 Implementation Plan](2025-10-29-license-v1beta2-implementation.md)
- [LicenseWrapper API Documentation](https://github.com/replicatedhq/kotskinds/blob/main/pkg/licensewrapper/README.md)
