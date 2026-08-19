# Semantic Progressive Disclosure v1

## Purpose

This contract is the shared workflow boundary for abstract processor
treatment. It lets the model choose an acoustic method and request evidence
while keeping plugin identity, PCA admission, parameter binding, and mutation
in deterministic services.

The workflow is:

```text
model intent and evidence plan
  -> model-requested CCB observation
  -> filtered control brief
  -> model physical target decision
  -> deterministic control_ref binding
  -> explicit confirmation
  -> typed controller transaction
  -> receipt and verification
```

The registry selects an adapter only after the model has supplied a validated
family. It is not a keyword router and does not infer a family from user text.

## Persisted State

`semantic_progressive_disclosure.v1` persists these stages:

`intent`, `observation`, `control_brief`, `physical_target`,
`control_binding`, `confirmation`, `applied`, `receipt`, and `rejected`.

Persisted state is replayed through the deterministic state machine before it
is used again. A forged family, coverage entry, view set, control reference,
or receipt ordering fails closed.

## Model-Owned Fields

The model owns:

- semantic processor family;
- open acoustic intent;
- required semantic coverage axes;
- target scope and control mode;
- confidence and evidence references;
- the CCB view IDs it requests;
- physical targets after the filtered control brief is disclosed.

The model never owns plugin paths, vendor mappings, arbitrary parameter IDs,
PCA subject keys, fingerprints, attestation IDs, or executable authorization.

## Observation Boundary

An action cannot leave the observation stage unless the observation is
successful and carries a CCB audit receipt. The original free-state model view
request, the receipt's model-requested view IDs, and the actual executed view
IDs must be three exactly equal sets. The service cannot add a default view,
replace a requested view, or accept a generic `views` map as a substitute for
the receipt.

## Loaded Instance Admission

Before a loaded instance enters a semantic planner, the service rereads its
live parameter surface and rebuilds its topology. It resolves the exact
project identity and queries the appropriate PCA store using:

- exact identifier or installed path plus name and format;
- current binary fingerprint;
- promoted/current attestation status;
- selected processor family;
- deterministic PCA coverage derived from model-selected semantic axes.

Missing identity, missing fingerprint, stale or revoked attestation, family
mismatch, ownership conflict, and insufficient coverage produce inspect-only
or rejected results. They never fall back to a generic parameter writer.

## Candidate Loading

The recommendation LLM receives only the PCA-filtered candidate envelope:

- opaque `candidate_key`;
- exact PCA-admitted `identifier`;
- display-only `name`.

Paths, vendors, formats, fingerprints, subject keys, and attestation IDs stay
server-side. The submitted key and identifier must match the supplied set.
Selection is rechecked against the current PCA library before a load plan is
created. Immediately before execution, the Agent load gate recomputes the
binary fingerprint and queries current PCA again. A changed or revoked
attestation prevents the load.

## Post-Load Admission

After a confirmed load, the service reads the returned instance live and
rechecks exact identity, fingerprint, family, required coverage, and current
topology. Only a qualified result may enter a typed planner. A successful
load is not parameter-write authority. If no adapter is materialized for the
family, the result is an auditable `qualified_planner_pending` boundary with
no parameter mutation.

## Typed Execution

Static EQ and broadband compressor, plus the five current dynamic families,
use this state machine end to end. The dynamic adapters are:

| Family | Model-requested source view | Representative semantic axes |
| --- | --- | --- |
| Limiter | `track.peak_structure` | `protection_intensity`, `output_ceiling`, `recovery_motion`, `detector_latency` |
| Gate / Expander | `track.activity_structure` | `activation_threshold`, `attenuation_floor`, `detector_focus`, `state_timing`, `direction_mode` |
| De-esser | `track.frequency_time_events` | `threshold_sensitivity`, `detector_focus`, `sibilance_reduction`, `recovery_motion` |
| Transient Shaper | `track.transient_structure` | `envelope_emphasis`, `envelope_timing`, `detector_focus`, `shape_mode` |
| Multiband Dynamics | `track.band_dynamics` | `band_dynamics`, `band_timing`, `crossover_layout`, `detector_focus` |

The view is selected by the model through CCB. The server does not infer or
add a view from user wording. After family and required coverage are validated,
the server discloses only a structural `path_key`, role, current value, domain,
and reachable values. A fresh topology resolves that path to an internal
`control_ref`; neither `control_ref` nor a parameter ID is model-generated.

Each family delegates writes to its existing typed controller. Confirmation,
atomic transaction writes, readback, restore, full snapshot validation, and
the family receipt remain mandatory. A successful receipt marks the loop as
requiring a fresh model-requested CCB observation; it does not mark the
original acoustic goal satisfied by itself.

Coverage is converted to PCA proof entries by the registry. Missing proof,
stale/revoked/fingerprint-mismatched identity, insufficient coverage, changed
topology, or a path outside the disclosed brief fails closed before any write.

Reverb and Delay are not part of this v1 adapter set and cannot be routed into
one of these dynamic controllers. Future families must register their own
observation, coverage, typed executor, and receipt contract before they can be
added.

Spectral Dynamics is inspect-only/unresolved. Clipper is a separate boundary
and cannot use Limiter PCA, coverage, planner, or controller.

## Free-State Post-Action Loop

An applied EQ, compressor, or dynamic-family receipt is appended to the
free-state reasoning ledger with its cycle, processor type, workflow status,
and complete governed receipt. The ledger sets
`requires_post_action_observation=true`; the next model turn must request a
fresh CCB observation itself. The service requires freshness and audit
agreement, but does not inject a default review view or tell the model which
view to request.

After fresh decisive evidence, the model may select another action, including
the same family. Each action still creates a new plan and confirmation. The
runtime bounds one family to two applied actions and the complete loop to six
applied actions. An inconclusive or insufficient post-action result cannot
authorize another write; the loop remains blocked or asks for observation.

The post-action gate distinguishes a successful CCB request executed in the
current reasoning turn (or its resumable continuation trace) from an older
observation carried in context. This prevents stale evidence from satisfying
the review requirement.

## Final Boundary Matrix

| Boundary | Result |
| --- | --- |
| Spectral Dynamics | inspect-only / unresolved; no semantic dynamic planner or typed executor |
| Clipper | independent inspect-only boundary; never a Limiter candidate and never shares Limiter PCA, coverage, planner, or controller |
| Reverb / Delay | outside v1; no fallback into an existing family |

The acceptance suite covers ledger persistence for all five dynamic families,
fresh-observation enforcement, bounded repeat actions, candidate/PCA fail
closed behavior, transaction receipts, and legacy EQ/compressor regression.

Representative evidence is kept close to the implementation:

- `internal/agentloop/semantic_guidance_audit_baseline_test.go` and
  `internal/chat/semantic_guidance_audit_baseline_test.go` verify neutral
  prompts, model-owned view requests, and the absence of pre-family identity
  injection;
- `internal/chat/free_state_reasoning_loop_test.go` verifies all five dynamic
  receipts enter the ledger and set the post-action observation gate;
- `internal/agentloop/free_state_reasoning_test.go` verifies fresh evidence,
  inconclusive stopping, same-family and total-action bounds;
- `internal/chat/semantic_dynamic_workflow_test.go` verifies typed dispatch,
  transaction/readback/restore receipts, PCA proof, model-owned post-action
  review, and the Spectral/Clipper boundary matrix.
- `internal/chat/plugin_recommendation_test.go` verifies that all five dynamic
  families preserve their exact post-load adapter through load confirmation
  without mutating the project.
