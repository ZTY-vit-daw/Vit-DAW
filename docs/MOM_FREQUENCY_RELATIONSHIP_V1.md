# MOM Frequency Relationship Projection v1

Status: implemented by MOM v1.5
Schema: `mom.frequency_relationship.v1`
Intent: `project_frequency_relationship_observation`
## Purpose

`frequency_relationship` is the reusable read-only relationship projection needed before C1 frequency cleanup. It is not named after C1 because B4, C5, ordinary Agent EQ diagnosis, and future verifiers may consume the same facts.

The projection answers only:

- which project tracks have fresh comparable frequency evidence;
- which of the six current analyzer regions each track occupies;
- which tracks are relative-energy overlap candidates in each region;
- each track's relative tonal shape;
- whether temporal persistence evidence exists;
- whether a later observation can be compared at the same tap and scope.

It does not choose an EQ, emit frequency/Q/gain parameters, load a plugin, create a Proposal/Pending action, or authorize execution. An energy-overlap candidate is never described as a proven psychoacoustic masking event.

## Authority and data flow

```text
existing L3 band summaries / existing L2 Render Probes
→ ProjectPackage frequency_relationship_inputs (DAD-derived facts)
→ MOM frequency_relationship (bounded relationships + limits)
→ future C1 readiness / diagnosis / verification
```

No observer, recognizer, topology, or executor is added. The ProjectPackage derivation prefers an existing L2 Render Probe row when it contains bands. Otherwise it uses the existing L3 band summary and labels its tap as `source_file_pre_fx`.

L3 source-file evidence may support coarse static diagnosis but cannot represent the current post-plugin signal. A mixed-tap project is `suspect` and cannot be used for before/after comparison.

## Envelope

Every materialized projection contains:

- `schema_version`
- `status`
- `freshness`
- `project_cut_ref`
- `scope`
- `tap_point`
- `coverage`
- `evidence_refs`
- `limitations`

The payload contains:

- `track_profiles`
- `frequency_regions`
- `conflict_candidates`
- `tonal_tendencies`
- `persistence_summary`
- `verification_dimensions`

`frequency_regions[].decision_tracks` is the complete compact decision set. It is never capped by a UI top-N excerpt; `decision_tracks_truncated` must remain false.

`project_cut_ref` is a reference, not a copy of ProjectCut authority. If a strong cut ref is supplied it is preserved. Otherwise the projection uses the current project state identity when available, or an observation authority ref with an explicit limitation. The observation ref never grants execution authority.

## Status and freshness

- `ready`: all project tracks have usable evidence at one known tap.
- `partial`: at least one track is usable, but coverage, freshness, or tap identity is incomplete.
- `suspect`: evidence contains a suspect row or mixes tap points.
- `stale`: no usable rows remain and stale evidence is present.
- `missing`: no usable frequency evidence exists.

Coverage reports project/profile/eligible/missing counts and IDs, region/candidate counts, the uncapped decision-set invariant, and three separate support flags:

- `supports_static_diagnosis`
- `supports_same_tap_compare`
- `supports_post_fx_compare`

Temporal persistence is never inferred from whole-song band totals. Until an existing evidence source supplies per-region persistence ratios, `persistence_summary.status` is `missing` with `no_time_frequency_overlap_evidence`.

## Before/after comparability

`FrequencyRelationshipsComparable` requires:

- schema v1 on both observations;
- the same known, non-mixed tap;
- the same project identity;
- the same complete track scope;
- neither side missing, stale, nor suspect.

Comparability only permits a verifier to inspect declared dimensions. It does not claim improvement and does not authorize a write.

## Read-only smoke contract

The acceptance smoke invokes only `mix.observe` or `mix.request_observation` with:

```json
{
  "mom_intent": "project_frequency_relationship_observation",
  "scope": "full_project",
  "observation_only": true,
  "disclosure": "digest_catalog"
}
```

The Harness canonicalizes an omitted scope to `full_project`, refreshes Project State, and does not apply the single-target acoustic ready gate. Partial project coverage is returned in the projection rather than hidden or promoted.

Acceptance gates:

1. MOM version is `v1.5` and schema is `mom.frequency_relationship.v1`.
2. Scope is full-project/full-song and all project tracks are represented.
3. Tap, coverage, freshness, ProjectCut ref, evidence refs, and limitations are present.
4. No decision track is removed by a UI top-N cap.
5. Missing persistence remains explicit.
6. Mixed tap, stale evidence, or incomplete coverage cannot become ready/comparable.
7. Context contains no waveform/time-segment arrays, spectral tiles, render paths, shared memory, or quality payloads.
8. No EQ/plugin/action/Proposal/Pending field is emitted.
9. Legacy observation intents do not materialize the new payload and retain their compact-size contract.
10. The smoke performs no DAW mutation. It does not add plugins, change parameters/faders, create Project History entries, or switch projects.

Automated coverage lives in:

- `agent/internal/mom/frequency_relationship_test.go`
- `agent/internal/mixboard/frequency_relationship_projection_test.go`
- `agent/internal/harness/frequency_relationship_observation_test.go`
