# Prevent Installation on Strict Preflight Failures (V3 Installer)

## TL;DR

Implement strict app preflight blocking in embedded-cluster's V3 installer UI and backend to prevent installation from proceeding when vendor-provided app preflights marked as `strict: true` fail. This mirrors KOTS behavior where strict app preflights cannot be bypassed even with the `--ignore-app-preflights` flag. The `strict` field already exists in app preflight specs - this proposal adds the missing logic to respect it in embedded-cluster (V3 installer) by detecting strict failures in the backend and updating the frontend to completely block progression, ensuring installations only proceed when mandatory app requirements set by vendors are met.

## The problem

Currently, embedded-cluster (V3 installer) allows **all vendor-provided app preflight failures** to be bypassed through the `--ignore-app-preflights` flag, even when the vendor marked certain checks as `strict: true` in their app preflight specs. The `allowIgnoreAppPreflights` field was temporarily hardcoded to `true` in the backend until support for strict app preflights could be implemented. This creates risk where critical app requirements can be ignored, leading to failed installations or runtime issues. Users can proceed with installation even when mandatory app prerequisites are not met, resulting in:

- Failed installations that could have been prevented
- Support burden from installations in unsupported app environments
- Poor user experience when apps fail due to unmet requirements

**Important distinctions:**
- This is specifically about **app preflights** provided by vendors in their app specs
- The `strict` field **already exists** in app preflight specs - this proposal adds support for respecting it
- This is **separate from host preflights** that embedded-cluster runs for k0s/system requirements, host preflights are not affected by this change

Evidence: Multiple TODO comments in codebase (e.g., "TODO: implement once we check for strict app preflights") indicate this was always intended functionality and the hardcoded `true` was temporary. KOTS already implements this pattern successfully for app preflights.

## Prototype / design

The solution adds strict **app preflight** detection following the KOTS pattern. The user experience design is based on recent updates made to our Bolt prototype, demonstrated in this [Loom video showing the overall user experience](https://www.loom.com/share/14a3edaa0dbf45088275725df1187c7b?sid=e645c156-d4c2-464f-8fe0-989105e1c123):

```
┌─────────────────┐     ┌────────────────────┐     ┌─────────────────┐
│  Preflight Spec │────▶│ Backend Detection  │────▶│ Frontend Block  │
│  (strict: true) │     │ HasStrictFailures  │     │ Disable Next    │
└─────────────────┘     └────────────────────┘     └─────────────────┘
                                  │
                                  ▼
                       ┌───────────────────────┐
                       │ InstallController     │
                       │ Blocks on Strict Fail │
                       └───────────────────────┘
```

Key flow:
1. **App Preflight Specification**: Vendor-provided app preflight specs already include `strict: true` for mandatory app environment checks that must pass
2. **Backend Detection**: Preflight manager reads existing strict field from troubleshoot analyzer results and populates it on `PreflightsRecord`
3. **Dual-Layer Blocking**: 
   - **Backend**: Install controller blocks installation when `HasStrictFailures()` returns true, ignoring `--ignore-app-preflights` flag to prevent API bypass
   - **Frontend**: v3 installer UI disables next button when `hasStrictAppPreflightFailures: true` in API response
4. **Visual Distinction**: Strict app preflight failures display with red borders and "Critical" badges so users understand which app issues must be resolved vs. which can be bypassed
5. **Bypass Prevention**: Even with `--ignore-app-preflights` flag, strict app preflight failures cannot be bypassed because:
   - Backend API call will return error regardless of ignore flag
   - Frontend button remains disabled when strict failures exist

## New Subagents / Commands

No new subagents or commands will be created for this implementation.

## Database

**No database changes required.**

The preflight results are stored in memory/temporary storage during the installation process and don't require persistent database schema changes.

## Implementation plan

### Backend Implementation

#### 1. Update Types (`/api/types/preflight.go`)
```go
// PreflightsRecord with strict field
type PreflightsRecord struct {
    Title   string `json:"title"`
    Message string `json:"message"`
    Strict  bool   `json:"strict"` // NEW FIELD
}

// Add helper method to check for strict failures
func (o *PreflightsOutput) HasStrictFailures() bool {
    for _, fail := range o.Fail {
        if fail.Strict {
            return true
        }
    }
    return false
}
```

#### 2. Update Install Controller (`/api/controllers/app/install/install.go`)
```go
func (c *InstallController) InstallApp(ctx context.Context, ignoreAppPreflights bool) error {
    // Check for strict preflight failures first - these cannot be bypassed
    if c.stateMachine.CurrentState() == states.StateAppPreflightsFailed {
        preflightOutput, _ := c.appPreflightManager.GetOutput()
        hasStrictFailures := preflightOutput.HasStrictFailures()
        
        // Block if strict failures exist, completely ignore the ignoreAppPreflights flag
        if hasStrictFailures {
            return types.NewBadRequestError(
                errors.New("installation blocked: strict preflight checks failed")
            )
        }
        
        // Only allow bypass for non-strict failures with flag
        if !ignoreAppPreflights {
            return types.NewBadRequestError(ErrAppPreflightChecksFailed)
        }
        
        // Transition to bypassed state for non-strict failures
        err = c.stateMachine.Transition(lock, states.StateAppPreflightsFailedBypassed)
    }
    // ... rest of implementation
}
```

#### 3. Update API Response (`/api/types/responses.go`)
```go
type InstallAppPreflightsStatusResponse struct {
    Status   *Status           `json:"status,omitempty"`
    Titles   []string          `json:"titles,omitempty"`
    Output   *PreflightsOutput `json:"output,omitempty"`
    HasStrictAppPreflightFailures bool `json:"hasStrictAppPreflightFailures"` // NEW FIELD
    AllowIgnoreAppPreflights      bool `json:"allowIgnoreAppPreflights"`
}
```

4. Update API handler (`/api/handlers/app/install/preflight.go`) to populate `hasStrictAppPreflightFailures` field:
```go
func (h *InstallHandler) GetAppPreflightsStatus(ctx context.Context) (*types.InstallAppPreflightsStatusResponse, error) {
    // ... existing code to get status, titles, output ...
    
    response := &types.InstallAppPreflightsStatusResponse{
        Status:   status,
        Titles:   titles, 
        Output:   output,
        AllowIgnoreAppPreflights: h.getAllowIgnoreAppPreflights(), // set based on CLI flag
    }
    
    // Set hasStrictAppPreflightFailures based on output
    if output != nil {
        response.HasStrictAppPreflightFailures = output.HasStrictFailures()
    }
    
    return response, nil
}
```

### Frontend Implementation

The user experience design is based on recent updates made to our Bolt prototype, demonstrated in this [Loom video showing the overall user experience](https://www.loom.com/share/14a3edaa0dbf45088275725df1187c7b?sid=e645c156-d4c2-464f-8fe0-989105e1c123)

#### 1. Update AppPreflightPhase (`/web/src/components/wizard/installation/phases/AppPreflightPhase.tsx`)
```typescript
const AppPreflightPhase: React.FC<AppPreflightPhaseProps> = ({ onNext, setNextButtonConfig, onStateChange }) => {
    // ... existing state ...
    const [hasStrictFailures, setHasStrictFailures] = useState(false);
    
    // Update to check for strict failures in output.fail array
    const onComplete = useCallback((success: boolean, allowIgnore: boolean, response: AppPreflightResponse) => {
        setPreflightComplete(true);
        setPreflightSuccess(success);
        setAllowIgnoreAppPreflights(allowIgnore);
        
        // Use backend-provided hasStrictAppPreflightFailures field
        const hasStrict = response?.hasStrictAppPreflightFailures || false;
        setHasStrictFailures(hasStrict);
        
        onStateChange(success ? 'Succeeded' : 'Failed');
    }, []);
    
    // Update existing onRun callback to reset strict failures state
    const onRun = useCallback(() => {
        setPreflightComplete(false);
        setPreflightSuccess(false);
        setAllowIgnoreAppPreflights(false);
        setHasStrictFailures(false);
        onStateChange('Running');
    }, []);
    
    const canProceed = useMemo(() => {
        if (!preflightComplete) return false;
        if (preflightSuccess) return true;
        
        // Block completely for strict failures - this is the key change
        // hasStrictFailures is determined by backend hasStrictAppPreflightFailures field
        if (hasStrictFailures) return false;
        
        // Allow bypass for non-strict failures if CLI flag allows
        return allowIgnoreAppPreflights;
    }, [preflightComplete, preflightSuccess, hasStrictFailures, allowIgnoreAppPreflights]);
    
    // Note: handleNextClick requires no changes - strict blocking is handled by canProceed disabling the button
    // Note: InstallationStep requires no changes - existing auto-advance logic only triggers on 'Succeeded' status
    
    // Existing UI will be updated to show strict failures with distinct visual indicators
    // Note: Button blocking is controlled by hasStrictAppPreflightFailures from API response
    return (
        <div>
            {/* ... existing preflight results UI ... */}
            {/* Failed checks now include distinct visual indicators */}
            <div className="space-y-3">
                {failedChecks.map(({ key, title, message, strict }) => (
                    <div 
                        key={key} 
                        className={`flex items-start space-x-3 p-3 rounded-md ${
                            strict ? 'border-l-4 border-red-500 bg-red-50' : 'bg-gray-50'
                        }`}
                    >
                        <XCircle className="w-5 h-5 text-red-500 mt-0.5 flex-shrink-0" />
                        <div className="flex-1 min-w-0">
                            <h5 className="text-sm font-medium text-red-800">
                                {title}
                            </h5>
                            <p className="mt-1 text-sm text-red-700">{message}</p>
                        </div>
                        {strict && (
                            <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-800 flex-shrink-0">
                                Critical
                            </span>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
};
```

#### 2. Update Types (`/web/src/types/index.ts`)
```typescript
export interface PreflightRecord {
    title: string;
    message: string;
    strict: boolean; // NEW FIELD
}

export interface PreflightOutput {
    pass: PreflightRecord[];
    warn: PreflightRecord[];
    fail: PreflightRecord[];
}

export interface AppPreflightResponse {
    titles: string[];
    output?: PreflightOutput;
    status?: PreflightStatus;
    hasStrictAppPreflightFailures: boolean; // NEW FIELD
    allowIgnoreAppPreflights?: boolean;
}
```

### Files to modify

- `/api/types/preflight.go` - Add strict field to PreflightsRecord and HasStrictFailures method to PreflightsOutput
- `/api/controllers/app/install/install.go` - Implement strict blocking logic (ignore allowIgnoreAppPreflights when strict failures exist)
- `/api/types/responses.go` - Add hasStrictAppPreflightFailures field to InstallAppPreflightsStatusResponse
- `/api/internal/handlers/kubernetes/install.go` - Update hardcoded AllowIgnoreAppPreflights to use CLI flag and populate hasStrictAppPreflightFailures field
- `/api/internal/handlers/linux/install.go` - Update hardcoded AllowIgnoreAppPreflights to use CLI flag and populate hasStrictAppPreflightFailures field
- `/api/integration/kubernetes/install/apppreflight_test.go` - Update tests to verify AllowIgnoreAppPreflights field
- `/api/integration/linux/install/apppreflight_test.go` - Update tests to verify AllowIgnoreAppPreflights field
- `/web/src/components/wizard/installation/phases/AppPreflightPhase.tsx` - Use hasStrictAppPreflightFailures field to disable next button
- `/web/src/components/wizard/installation/phases/AppPreflightCheck.tsx` - Display strict indicators
- `/web/src/types/index.ts` - Add strict field to PreflightRecord interface and hasStrictAppPreflightFailures to AppPreflightResponse

## Testing

### Unit Tests
```go
// Test strict field detection
func TestHasStrictFailures(t *testing.T) {
    output := PreflightsOutput{
        Fail: []PreflightsRecord{
            {Title: "Check 1", Strict: true},
            {Title: "Check 2", Strict: false},
        },
    }
    assert.True(t, output.HasStrictFailures())
}

// Test API response includes hasStrictAppPreflightFailures field
func TestInstallAppPreflightsStatusResponse(t *testing.T) {
    response := InstallAppPreflightsStatusResponse{
        HasStrictAppPreflightFailures: true,
        AllowIgnoreAppPreflights: true,
    }
    assert.True(t, response.HasStrictAppPreflightFailures)
}

// Test blocking logic
func TestInstallAppBlocksOnStrictFailure(t *testing.T) {
    // Setup controller with failing strict preflight
    err := controller.InstallApp(ctx, true) // ignore flag = true
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "strict preflight checks failed")
}
```

### Integration Tests
- Test API endpoint returns `hasStrictAppPreflightFailures: true` when strict failures exist
- Test API endpoint returns `hasStrictAppPreflightFailures: false` when no strict failures exist
- Test installation blocked in the backend even with `ignoreAppPreflights: true` when strict failures present
- Test `allowIgnoreAppPreflights` field remains unchanged regardless of strict failures
- Test non-strict failures can be bypassed with flag
- Test mixed strict/non-strict failures: `hasStrictAppPreflightFailures: true`, installation blocked


## Monitoring & alerting

- Log when installation blocked due to strict app preflights

Logging:
```go
logger.Warn("Installation blocked due to strict app preflight failures",
    "failedChecks", getFailedStrictChecks(output),
    "installID", installID)
```

## Backward compatibility

- Existing app preflight specs without `strict` field default to `strict: false`
- Current bypass behavior preserved for non-strict app preflight checks
- No breaking API changes - `allowIgnoreAppPreflights` field behavior changes but structure remains unchanged
- New `strict` field added to `PreflightsRecord` struct (additive change)
- New `hasStrictAppPreflightFailures` field added to API response (additive change)
- Frontend gracefully handles API responses without new fields (defaults to false)

## Migrations

No special migration handling required.

## Trade-offs

**Optimizing for:** Safety and reliability of installations

**Trade-offs made:**
1. **Stricter blocking vs flexibility** - We prevent bypass of critical checks, reducing flexibility but ensuring stable installations
2. **Additional complexity vs simplicity** - Adding strict field increases complexity but provides necessary granularity
3. **Breaking bypass workflows vs safety** - Some users may have workflows that bypass all preflights; strict checks will break these but for good reason

## Alternative solutions considered

1. **Client-side strict failure detection**
   - Initially proposed having frontend iterate through `output.fail` array to check for `strict: true`
   - Rejected: Better to centralize logic in backend and provide `hasStrictAppPreflightFailures` field

2. **Separate `strictFailures` field in API response**
   - Considered adding array of strict failures alongside existing output structure
   - Rejected: Simpler to add boolean flag and keep strict info in existing `PreflightsRecord` structure

3. **Client-side only enforcement**
   - Rejected: Not secure, could be bypassed by direct API calls

4. **Multiple API response approaches tried**
   - Tried `strictFail` field in Output struct
   - Tried separate strict failures array
   - Settled on `hasStrictAppPreflightFailures` boolean for simplicity

## Research

**Reference:** See `/proposals/strict_preflight_blocking_research.md` for detailed codebase analysis

### Prior art in codebase
- **Host preflights** already have `AllowIgnoreHostPreflights` pattern (separate from app preflights)
- State machine already handles app preflight failure states
- KOTS implementation provides proven pattern for **app preflights**: https://github.com/replicatedhq/kots/blob/db6e1dc/pkg/version/version.go#L195
- Troubleshoot library already outputs `strict` field in app preflight results

### External references
- Replicated Preflight documentation on strict checks
- KOTS strict preflight implementation patterns

### Key findings
- Multiple TODO comments indicate this was always planned functionality for **app preflights**
- The `strict` field already exists in troubleshoot library output for app preflights
- Frontend already has modal/bypass infrastructure for non-strict app preflight failures
- State machine supports bypassed state for non-strict app preflight failures
- Frontend already stops auto-advancing on app preflight failures
- **Host preflights are separate** and already have their own bypass logic

## Checkpoints (PR plan)

### PR 1: Backend Implementation
**Deliverable:** Add strict preflight support to the API (additive changes only)

1. Add `strict` field to `PreflightsRecord` struct in `/api/types/preflight.go`
2. Add `HasStrictFailures()` helper method to `PreflightsOutput`
3. Add `hasStrictAppPreflightFailures` field to `InstallAppPreflightsStatusResponse` in `/api/types/responses.go`
4. Update API handlers to populate `hasStrictAppPreflightFailures` field in both:
   - `/api/internal/handlers/kubernetes/install.go` - `GetAppPreflightsStatus` method
   - `/api/internal/handlers/linux/install.go` - `GetAppPreflightsStatus` method
5. Update hardcoded `AllowIgnoreAppPreflights: true` in kubernetes and linux install handlers to use CLI flag
6. Update install controller to block installation when strict failures exist (regardless of `ignoreAppPreflights` flag)
7. Update existing integration tests to verify `AllowIgnoreAppPreflights` field behavior with CLI flag
8. Add tests for strict detection logic and API responses with hasStrictAppPreflightFailures field

**Outcome:** API now returns `strict` field on app preflight records and `hasStrictAppPreflightFailures` field. `AllowIgnoreAppPreflights` now uses CLI flag instead of hardcoded true. Frontend gracefully ignores new fields until PR 2 is deployed.

### PR 2: Frontend Implementation
**Deliverable:** Add strict preflight blocking to the UI

**Prerequisites:** PR 1 must be deployed first

1. Add `hasStrictAppPreflightFailures` field to TypeScript interfaces in `/web/src/types/index.ts`
2. Update `AppPreflightPhase` component to:
   - Use `hasStrictAppPreflightFailures` field from API response instead of checking individual failures
   - Disable next button when `hasStrictAppPreflightFailures` is true
   - Update `onComplete` callback to receive full API response instead of just output
3. Update `AppPreflightCheck` component to display distinct visual indicators:
   - Red left border for strict failures
   - "Critical" badge for strict failures
4. Add tests for strict failure detection using hasStrictAppPreflightFailures field

**Outcome:** Complete strict **app preflight** blocking functionality. Installation blocked for strict app preflight failures, distinct UI indicators, maintains bypass for non-strict app preflight failures. Host preflights remain unaffected.
