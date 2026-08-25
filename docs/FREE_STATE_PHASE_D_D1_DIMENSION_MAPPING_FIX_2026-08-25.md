# Free State Phase D - D1 Dimension Mapping Fix

Date: 2026-08-25

## Context

Current Status: Phase D D1-S1 on branch `codex/g1-g7-runtime-remediation`, commit `69e499d`.

**Problem Diagnosis:**
- Free State smoke tests reach FS2 capacity_assessed but return `no_candidate_found`
- Model requests `mix.masking_relationship` which shows clear evidence (margin 14-85dB)
- Model fails to form improvement hypothesis despite plausible evidence
- Root causes identified:
  1. **Structure**: CCB views lack dimension mapping (view → diagnostic dimension → action domain)
  2. **Semantics**: View limitations use defensive language that discourages hypothesis formation
  3. **Reasoning**: Missing pattern recognition guidance for evidence interpretation

**Design Reference:**
- `docs/FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md` - Free State diagnostic spine and priority queue design
- Priority queue: `level_headroom → frequency_occupancy → dynamics → stereo_space → transient_event`
- Current implementation: `agent/internal/agentloop/ccb_model_prompt.go`, `agent/internal/agentloop/free_state_reasoning.go`

## Implementation Plan

### Phase 1: Establish Dimension Mapping Structure (修复 A)

**Goal:** Add structured dimension hints to CCB views so Runtime and Model understand which diagnostic dimension each view serves.

#### Task A1: Extend CCB View Schema

**File to locate/modify:** Find where CCB view catalog is defined and generated. Likely candidates:
- `agent/internal/capabilitycontext/free_state_observation.go`
- `agent/internal/capabilitycontext/ccb_*.go`
- Search for files generating view catalog JSON with fields like `view_id`, `questions`, `limitations`

**Action:**
1. Locate the CCB view descriptor struct (currently returns fields: `view_id`, `questions`, `supported_target_kinds`, `temporal_resolution`, `availability`, `cost_latency_class`, `quality_ceiling`, `limitations`, `required_dependencies`)

2. Extend the struct with new fields:
   ```go
   // Add to existing view descriptor struct
   DiagnosticDimensions    []string                   `json:"diagnostic_dimensions,omitempty"`
   InterpretationGuidance  *CCBInterpretationGuidance `json:"interpretation_guidance,omitempty"`
   ```

3. Define supporting types:
   ```go
   type CCBInterpretationGuidance struct {
       Patterns []CCBEvidencePattern `json:"patterns,omitempty"`
   }
   
   type CCBEvidencePattern struct {
       Name        string   `json:"name"`
       Signal      string   `json:"signal"`
       Suggests    []string `json:"suggests"`
       Description string   `json:"description"`
   }
   ```

4. Ensure backward compatibility: these fields are optional (`omitempty`), existing code should not break

#### Task A2: Update Key Views with Dimension Mapping

**Views to update (in priority order):**

1. **mix.masking_relationship** (highest priority - this is what failed in smoke test):
   ```go
   DiagnosticDimensions: []string{"level_headroom", "frequency_occupancy"},
   InterpretationGuidance: &CCBInterpretationGuidance{
       Patterns: []CCBEvidencePattern{
           {
               Name: "large_consistent_margin",
               Signal: "median_margin_db > 20 and risk_coverage_ratio > 0.9",
               Suggests: []string{"level_imbalance", "track_gain"},
               Description: "Large consistent margins across time and bands suggest level-imbalance improvement candidates",
           },
           {
               Name: "band_specific_margin",
               Signal: "margin concentrated in specific frequency bands",
               Suggests: []string{"frequency_conflict", "eq"},
               Description: "Band-specific patterns may indicate frequency-domain considerations",
           },
       },
   },
   ```

2. **mix.multitrack_relationship**:
   ```go
   DiagnosticDimensions: []string{"level_headroom"},
   InterpretationGuidance: &CCBInterpretationGuidance{
       Patterns: []CCBEvidencePattern{
           {
               Name: "level_difference",
               Signal: "consistent RMS or peak differences across tracks",
               Suggests: []string{"level_imbalance", "track_gain"},
               Description: "Consistent level differences suggest gain adjustment candidates",
           },
       },
   },
   ```

3. **track.basic_energy**:
   ```go
   DiagnosticDimensions: []string{"level_headroom"},
   ```

4. **mix.frequency_relationship**:
   ```go
   DiagnosticDimensions: []string{"frequency_occupancy"},
   ```

5. **track.timbre_frequency**:
   ```go
   DiagnosticDimensions: []string{"frequency_occupancy"},
   ```

6. **track.time_dynamics**:
   ```go
   DiagnosticDimensions: []string{"dynamics"},
   ```

7. **track.stereo_space**:
   ```go
   DiagnosticDimensions: []string{"stereo_space"},
   ```

8. **track.transient_structure**:
   ```go
   DiagnosticDimensions: []string{"transient_event"},
   ```

#### Task A3: Wire Dimension Mapping to Priority Queue

**Goal:** Allow priority queue to recommend views based on dimension.

**Action:**
1. Locate priority queue implementation (likely in `agent/internal/audioclosure` or `agentloop`)
2. Add method to query recommended views for a dimension:
   ```go
   func GetViewsForDimension(dimension string, catalog []CCBViewDescriptor) []string {
       var views []string
       for _, view := range catalog {
           for _, dim := range view.DiagnosticDimensions {
               if dim == dimension {
                   views = append(views, view.ViewID)
                   break
               }
           }
       }
       return views
   }
   ```

3. Optional: Update system prompt to inform model of dimension-view mapping when priority queue advances

#### Task A4: Testing

**Test coverage:**
1. Unit test: CCB catalog generation includes new fields
2. Unit test: GetViewsForDimension correctly filters views
3. Integration test: Priority queue can access dimension-view mapping
4. Smoke test: Run D1 smoke with updated catalog, verify model receives dimension hints

**Validation:**
```powershell
cd D:\Vit_DAW\agent
go test ./internal/capabilitycontext -run 'Test.*CCB.*Catalog' -count=1
go test ./internal/agentloop -count=1
```

---

### Phase 2: Optimize View Limitation Language (修复 B)

**Goal:** Replace defensive "not proof of defect" language with constructive "plausible evidence for hypothesis" guidance.

#### Task B1: Audit All View Limitations

**Action:**
Search for all views with defensive language patterns:
- "not deterministic"
- "not proof"
- "does not establish"
- "does not prove"

**Files to check:**
- Where CCB view catalog is generated (found in Task A1)
- Grep for limitation text:
  ```powershell
  cd D:\Vit_DAW\agent
  rg "not proof|not deterministic|does not establish|does not prove" -g "*.go"
  ```

#### Task B2: Rewrite Limitations for Key Views

**Principle:** Preserve epistemic humility but use constructive framing.

**Pattern to follow:**
- ❌ OLD: "X is not proof that Y is wrong"
- ✅ NEW: "X provides plausible evidence for Y improvement candidates"

**Specific rewrites:**

1. **mix.masking_relationship**:
   - OLD: `"Candidates are relative energetic-risk evidence, not deterministic perceptual facts or proof that the mix is wrong."`
   - NEW: `"Candidates are relative energetic-risk evidence showing directional energy relationships. Large consistent margins often indicate level-imbalance improvement opportunities; band-specific patterns may indicate frequency considerations. This is plausible evidence for bounded improvement hypotheses, not proof of defects."`

2. **mix.frequency_relationship**:
   - OLD: `"Does not claim psychoacoustic masking certainty."`
   - NEW: `"Band overlaps suggest frequency-relationship improvement candidates. This is plausible evidence for bounded experiments, not deterministic masking proof."`

3. **track.peak_structure**:
   - OLD: `"Sample peaks do not establish true peak or clipping by themselves."`
   - NEW: `"Sample-peak structure provides bounded clipping-risk evidence. Extreme values suggest peak-management improvement candidates, though true-peak confirmation requires additional measurement."`

4. **project.change_delta**:
   - OLD: `"A project change receipt does not establish an acoustic result or an improvement."`
   - NEW: `"Change receipts confirm engineering mutations occurred. Acoustic evaluation requires fresh observation of the new state."`

Apply similar rewrites to other views systematically.

#### Task B3: Testing

**Validation:**
1. Verify all limitations still compile and serialize correctly
2. Check smoke test report JSON to confirm new limitation text appears in catalog
3. Run D1 smoke test, observe if model forms hypotheses more readily

```powershell
cd D:\Vit_DAW
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW -PublicCaseId spv1_p02 -SkipBuild
```

---

### Phase 3: Add Pattern Recognition Guidance to System Prompt (修复 C)

**Goal:** Provide model with explicit pattern-recognition hints for common evidence types.

#### Task C1: Create Pattern Recognition Prompt Section

**File to modify:** `agent/internal/agentloop/ccb_model_prompt.go`

**Action:**
1. Add new function:
   ```go
   func freeStatePatternRecognitionGuidance() string {
       return `Evidence Pattern Recognition for Open Improvement Tasks:
   
   When interpreting CCB observation results, consider these common patterns:
   
   Level & Headroom Dimension:
   - mix.masking_relationship with large consistent margins (median >20dB, coverage >0.9) across multiple bands
     → often indicates level-imbalance improvement candidates → consider track_gain domain
   - mix.multitrack_relationship showing consistent level differences
     → suggests gain adjustment candidates
   - track.basic_energy showing extreme headroom or crest differences
     → may indicate level normalization opportunities
   
   Frequency Dimension:
   - mix.masking_relationship with band-specific patterns (margin concentrated in 1-2 bands)
     → may indicate frequency-domain considerations → if eq admitted, consider frequency separation
   - mix.frequency_relationship showing band overlap between tracks
     → suggests frequency separation candidates
   - track.timbre_frequency showing band imbalance within one track
     → may indicate tonal adjustment opportunities
   
   Dynamics Dimension:
   - track.time_dynamics showing extreme crest or envelope variation
     → suggests dynamics control candidates
   - processor.behavior showing excessive gain reduction or pumping
     → may indicate dynamics recalibration needs
   
   Stereo Dimension:
   - track.stereo_space showing extreme correlation or imbalance
     → suggests stereo width or balance candidates
   
   Transient Dimension:
   - track.transient_structure showing onset/sustain imbalance
     → suggests transient shaping candidates
   
   Important Epistemic Notes:
   - These patterns are guidance for hypothesis formation, not deterministic rules
   - Partial or bounded evidence supporting a plausible improvement hypothesis is sufficient for needs_experiment
   - You are NOT required to prove an objective defect before proposing a bounded reversible experiment
   - When evidence plausibly relates to the user's listening goal but cannot prove a defect, return needs_experiment with improvement_proposal
   - Large consistent patterns across time and bands are stronger signals than isolated or brief variations
   
   `
   }
   ```

2. Insert into system prompt after improvement contract section:
   ```go
   func messageLoopNeutralFamilySystemPrompt(state *runState) string {
       // ... existing code ...
       
       if strings.EqualFold(messageLoopTaskContractKind(state), "improvement") {
           prefix += `This is an open improvement contract. A local diagnosis or evidence-sufficient dimension does not complete the user's task. You MUST NOT return satisfied. After observation, return needs_experiment with one bounded evidence-backed improvement_proposal, no_candidate_found with a diagnostic covering the bounded search and its limitations, or capability_blocked with a concrete runtime boundary. The product runtime alone settles the task after the experiment contract.
   
   `
           // NEW: Add pattern recognition guidance
           prefix += freeStatePatternRecognitionGuidance()
       }
       
       // ... rest of function ...
   }
   ```

#### Task C2: Testing

**Validation:**
1. Check generated system prompt contains pattern recognition section:
   - Add temporary log or inspection point in message loop
   - Or check smoke test logs for prompt content

2. Run D1 smoke test with all three fixes applied:
   ```powershell
   cd D:\Vit_DAW
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW -PublicCaseId spv1_p02 -SkipBuild
   ```

3. Verify model behavior:
   - Model should request masking_relationship
   - Model should recognize margin >20dB pattern
   - Model should form track_gain improvement_proposal
   - Should reach FS7 with valid proposal

4. Run full test suite:
   ```powershell
   cd D:\Vit_DAW\agent
   go test ./... -count=1
   ```

---

## Acceptance Criteria

### Phase 1 (A) Complete When:
- [ ] CCB view schema extended with `diagnostic_dimensions` and `interpretation_guidance` fields
- [ ] At least 8 core views updated with dimension mappings (masking_relationship, multitrack_relationship, basic_energy, frequency_relationship, timbre_frequency, time_dynamics, stereo_space, transient_structure)
- [ ] Catalog generation includes new fields in JSON output
- [ ] Unit tests pass
- [ ] Smoke test shows dimension hints in catalog

### Phase 2 (B) Complete When:
- [ ] All defensive limitation language audited
- [ ] At least 4 key views rewritten with constructive limitations (masking_relationship, frequency_relationship, peak_structure, change_delta)
- [ ] Smoke test shows new limitation text in catalog
- [ ] Model shows increased hypothesis formation (not definitive, but observable trend)

### Phase 3 (C) Complete When:
- [ ] Pattern recognition guidance function implemented
- [ ] Guidance integrated into improvement contract system prompt
- [ ] Smoke test logs confirm guidance present in prompt
- [ ] D1 smoke test succeeds: model forms track_gain proposal from masking evidence
- [ ] Full test suite passes

### Overall Success Criteria:
- [ ] D1 smoke test with `spv1_p02` (injected-error stems) produces:
  - Model requests mix.masking_relationship
  - Model recognizes level-imbalance pattern
  - Model forms track_gain improvement_proposal at FS7
  - Phase advances to FS8 (not FS9 no_candidate_found)
- [ ] No regression in existing test suite
- [ ] Git history clean: one commit per phase with clear message

---

## Implementation Notes

### File Discovery Strategy

If you cannot immediately locate CCB catalog generation:

1. Search for view_id definitions:
   ```powershell
   cd D:\Vit_DAW\agent
   rg "mix.masking_relationship" -g "*.go"
   rg "view_id.*questions.*limitations" -g "*.go"
   ```

2. Search for catalog assembly:
   ```powershell
   rg "observation_catalog|ccb.*catalog" -g "*.go"
   ```

3. Check capability context:
   ```powershell
   ls internal/capabilitycontext/
   rg "CCB|Catalog" internal/capabilitycontext/ -g "*.go"
   ```

4. Check agentloop CCB handling:
   ```powershell
   rg "ccb_observation_catalog" internal/agentloop/ -g "*.go"
   ```

### Commit Strategy

**Phase 1 commit:**
```
feat(free-state): add dimension mapping to CCB views

- Extend CCB view schema with diagnostic_dimensions and interpretation_guidance
- Update 8 core views with dimension hints and pattern descriptions
- Wire dimension mapping to priority queue
- Closes issue: dimension → view → action mapping断裂

Ref: docs/FREE_STATE_PHASE_D_D1_DIMENSION_MAPPING_FIX_2026-08-25.md Phase 1
```

**Phase 2 commit:**
```
refactor(free-state): optimize CCB view limitation language

- Replace defensive "not proof" language with constructive guidance
- Rewrite limitations for masking, frequency, peak, and change views
- Preserve epistemic humility while encouraging hypothesis formation

Ref: docs/FREE_STATE_PHASE_D_D1_DIMENSION_MAPPING_FIX_2026-08-25.md Phase 2
```

**Phase 3 commit:**
```
feat(free-state): add pattern recognition guidance to system prompt

- Add freeStatePatternRecognitionGuidance() with dimension-specific patterns
- Integrate guidance into improvement contract prompt
- Provide evidence interpretation hints without hard rules

Ref: docs/FREE_STATE_PHASE_D_D1_DIMENSION_MAPPING_FIX_2026-08-25.md Phase 3
```

### Rollback Plan

If any phase causes regression:
- Each phase is a separate commit
- `git revert <commit-hash>` to roll back problematic phase
- Phases are designed to be independently functional

### Risk Assessment

**Low Risk:**
- Phase 1: New fields are optional, backward compatible
- Phase 2: Only changes text content, no logic changes

**Medium Risk:**
- Phase 3: Adds significant prompt content, may affect token budget or model behavior in unexpected ways

**Mitigation:**
- Test each phase independently
- Run full smoke test suite after each phase
- Keep phases small and reversible

---

## Post-Implementation

After all phases complete:

1. **Update documentation:**
   - Mark this fix document as "implemented"
   - Update `docs/FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md` with implementation notes

2. **Run extended validation:**
   - Multiple D1 smoke test runs (at least 3) to account for model randomness
   - Check that NOT_EXERCISED rate decreases
   - Verify when model does choose track_gain, it succeeds through FS7→FS8

3. **Report results:**
   - Document success rate improvement
   - Note any remaining edge cases
   - Identify next bottlenecks in Free State execution

4. **Consider follow-up work:**
   - Extend dimension mapping to all views (not just 8 core)
   - Add dimension-specific guidance to other phases (not just improvement contract)
   - Instrument dimension selection for analytics

---

## References

- Design: `docs/FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md`
- Current handoff: `docs/FREE_STATE_PHASE_D_D1_S1_CONTINUATION_2026-08-25.md`
- Priority queue design: Section 5 of FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE
- Diagnostic spine: Section 4 of FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE
- Epistemic policy: Section 6 of FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE

---

End of implementation guide.
