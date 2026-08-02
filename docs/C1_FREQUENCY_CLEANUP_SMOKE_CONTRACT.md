# C1 Frequency Cleanup v1 Smoke Contract

## Safety boundary

Automated smoke must use fixtures and temporary stores. It must not mutate the currently open VitApp project, write Project History for a real project, load a real plug-in, or write real EQ parameters. Live DAW acceptance is a separate explicit manual step.

## Automated gates

From `D:\Vit_DAW\agent`:

```powershell
go test ./internal/frequencycleanup ./internal/capabilitycontext ./internal/capabilityadapters
go test ./internal/orchestration ./internal/orchestrationruntime
go test ./internal/chat ./internal/harness ./internal/mixboard
go test ./...
```

Or run the bounded wrapper from the repository root:

```powershell
.\scripts\run_c1_frequency_cleanup_smoke.ps1
```

The wrapper points `VIT_MIXBOARD_ROOT` at a newly created temporary directory and removes only that validated temporary directory on exit.

The gates prove:

- source-file pre-FX evidence supports diagnosis but blocks mutation;
- C1 assembles its diagnosis CCB from existing Feature Snapshot/Acoustic Package facts with zero `mix.observe` calls and zero persistence writes;
- exact current-project/track/clip/source Acoustic Package L2 can feed diagnosis in memory, while mismatched material is rejected;
- exact source-file L3 may survive session/project relabeling, but a different source revision is rejected;
- render revision changes invalidate L2 while preserving exact-source pre-FX L3;
- Acoustic Package compaction preserves `track_state_fingerprint` for future L2 cache safety;
- a pre-LLM readiness repair selects only explicit missing tracks, refuses more than four tracks and refuses an all-project scope; it never starts full-project audio analysis;
- mutation readiness is unavailable until classification selects an exact target scope and that scope has a same-tap post-FX baseline;
- the complete project track roster is never top-N truncated;
- every track must receive exactly one valid treatment classification;
- C2/C3/C4 deferrals, source/arrangement issues, and no-change items do not become EQ actions;
- C1 freezes its identity around ordinary generic-EQ leaves;
- the configured shared batch port preserves structural leaf readbacks and all-or-rollback behavior;
- selected-track EQ remains owned by the ordinary Agent;
- C1 uses the existing Kernel L2 Render Probe only for selected static-EQ targets and their diagnosis-candidate peers;
- an exact track-state cache hit performs zero renders, while a fresh rendered batch creates no Mixboard Observation;
- the frozen ActionSet contains the exact target baseline, and verification is bound to that frozen scope;
- after plug-in loading, C1 reuses the existing treatment classification instead of repeating full-project classification;
- C1 Mixboard impact marks B2 and B4 `needs_review` while leaving unrelated B3 verified;
- the C1 plug-in-load prerequisite is not misreported as the final frequency-cleanup decision.

## Controlled read-only smoke

The domain/context/harness tests construct synthetic project frequency evidence and use `t.TempDir()` stores. After the run, verify the repository's real VitApp action and Project History locations have not changed. `git status --short` may show pre-existing user-owned workspace files; the smoke must not create a new real action receipt or project-history record.

## Optional manual acceptance

Manual DAW acceptance is permitted only after the automated gates pass and the user explicitly chooses a disposable or backed-up project. The expected interaction is:

1. Read-only C1 analysis shows full track coverage and classifications without mutation.
2. A mutation request normally classifies the full project before any target L2 render. If diagnosis has a small explicit evidence gap, only those missing tracks receive one bounded readiness probe before reassembly and classification.
3. Only selected static-EQ targets and their relationship peers receive mutation `track_post_fader` preflight probes; valid fingerprinted cache rows are reused.
4. Missing EQ instances, if any, appear in a separate load Proposal; the loaded-instance handoff does not reclassify the project.
5. EQ parameters appear in a second exact Proposal with the target baseline frozen into the ActionSet.
6. Rejecting either Proposal performs no corresponding mutation.
7. Approving the parameter Proposal applies one project batch; an injected leaf failure restores all earlier leaves.
8. Post-execution verification freshly probes only the frozen scope and reports `user_acceptance=unknown`.
9. Mixboard shows C1 plus the expected B2/B4 revalidation state.
