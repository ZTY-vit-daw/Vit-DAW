# Phase D Dimension Mapping Fix - Testing Guide

Date: 2026-08-25
Branch: `codex/g1-g7-runtime-remediation`
Commit: After 69e499d (changes not yet committed)

## Changes Summary

### Phase 1: Dimension Mapping Structure
- Extended CCB view schema with `diagnostic_dimensions` and `interpretation_guidance`
- Updated 8 core views with dimension hints
- Added `GetViewsForDimension()` function

### Phase 2: Optimized Limitation Language
- Rewrote 4 key view limitations from defensive to constructive framing

### Phase 3: Pattern Recognition Guidance
- Added `freeStatePatternRecognitionGuidance()` to system prompt
- Integrated into improvement contract workflow

## Testing Steps

### Step 1: Quick Validation (5 minutes)

Open PowerShell in `D:\Vit_DAW` and run:

```powershell
.\scripts\test_phase_d_changes.ps1
```

**Expected output:**
```
[1/3] Testing agent build...
✓ Agent builds successfully

[2/3] Running dimension mapping tests...
✓ Dimension mapping tests passed

[3/3] Running existing CCB tests for regression check...
✓ No regression in existing tests

=== All Phase D validation tests passed ===
```

**If this fails:** Stop here and report the error. Code needs fixing before smoke test.

### Step 2: D1 Smoke Test (10-15 minutes)

If Step 1 passes, run the full smoke test:

```powershell
.\scripts\run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW -PublicCaseId spv1_p02 -SkipBuild
```

**What this tests:**
- Uses `spv1_p02` test case (stems with injected level imbalance)
- Tests the complete Free State workflow: FS1 → FS2 → ... → FS8+

### Step 3: Analyze Results

Check the smoke test output for these key indicators:

#### ✅ SUCCESS Indicators:

1. **Model requests masking observation:**
   ```
   "requested_view_ids": ["mix.masking_relationship"]
   ```

2. **Model recognizes the pattern:**
   - Should see evidence like: `median_margin_db > 20`, `risk_coverage_ratio > 0.9`

3. **Model forms improvement proposal (FS7):**
   ```json
   "status": "needs_experiment"
   "action_domain": "track_gain"
   "hypothesis": "level imbalance..."
   ```

4. **Workflow reaches FS8 or beyond:**
   - NOT stopping at FS2 with `no_candidate_found`

#### ❌ FAILURE Indicators:

1. **Model returns `no_candidate_found` at FS2-FS6**
   - Means pattern recognition still not working

2. **Model requests wrong views**
   - Not recognizing dimension mapping

3. **Build/compilation errors**
   - Code issues need fixing

4. **Test crashes or timeouts**
   - Runtime issues

### Step 4: Check Smoke Test Report

After the test completes, check:

```powershell
# Find the latest report
Get-ChildItem D:\Vit_DAW\agent\test_reports\free_state_d1_smoke_*.json | 
    Sort-Object LastWriteTime -Descending | 
    Select-Object -First 1
```

Look for:
- `"final_status"`: Should NOT be `"no_candidate_found"` or `"capability_blocked"`
- `"phase_reached"`: Should be >= FS7 or FS8
- `"improvement_proposal"`: Should exist with `action_domain: "track_gain"`

## Expected Test Duration

- **Quick validation:** ~5 minutes
- **D1 smoke test:** ~10-15 minutes
- **Total:** ~15-20 minutes

## What to Do Next

### If ALL tests pass:

```powershell
# 1. Clean up git lock file
Remove-Item D:\Vit_DAW\.git\index.lock -Force -ErrorAction SilentlyContinue

# 2. Ready to commit the changes (3 separate commits)
# Let me know and I'll guide you through the commits
```

### If ANY test fails:

1. **Copy the error message**
2. **Share the smoke test report JSON** (if it got that far)
3. **We'll diagnose and fix the issue**

## Manual Test (Alternative)

If automated tests are blocked, you can manually verify:

```powershell
# 1. Build agent
cd D:\Vit_DAW\agent
go build ./cmd/vitagent

# 2. Run unit tests
go test ./internal/capabilitycontext -run TestCCBViewCatalogIncludesDimensionMapping -v
go test ./internal/capabilitycontext -run TestGetViewsForDimension -v

# 3. Check for regressions
go test ./internal/capabilitycontext -count=1
```

## Files Modified

- `agent/internal/capabilitycontext/free_state_observation.go`
- `agent/internal/capabilitycontext/free_state_observation_test.go`
- `agent/internal/agentloop/ccb_model_prompt.go`

## Rollback Plan

If tests fail and fixes are complex:

```powershell
# Discard all changes
git checkout agent/internal/capabilitycontext/free_state_observation.go
git checkout agent/internal/capabilitycontext/free_state_observation_test.go
git checkout agent/internal/agentloop/ccb_model_prompt.go
```

---

**Ready to test?** Start with Step 1 and let me know the results!
