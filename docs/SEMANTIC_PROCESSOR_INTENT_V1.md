# Semantic Processor Intent v1

## Scope

`semantic_processor_intent.v1` is the model-owned handoff from an observed
acoustic goal to a governed processor family. It carries only:

- `family`;
- an open `intent`;
- model-selected `required_coverage` keys;
- `scope`;
- `control_mode`;
- numeric `confidence`;
- exact `evidence_refs`;
- structured `rejection` when the status is `unresolved`.

It never carries a plug-in path, plug-in identifier, vendor/product mapping,
parameter identifier, normalized value, or control mapping.

## Ownership Boundary

The LLM chooses family, open intent, and required coverage after requesting and
reading the observation evidence it considers relevant. Deterministic code
validates the protocol, registered family, coverage vocabulary, scope, and
control mode. It does not fill missing fields, infer a family, add a default
coverage axis, or translate user wording into a family/view.

The processor registry has no keywords, phrases, trigger rules, examples, or
vendor matching. Its `CoverageProofs` table is a family-local deterministic
proof boundary: after the model has selected a semantic coverage key, a later
PCA resolver may obtain the exact attestation coverage entry. A missing proof
is a rejection, never a fallback.

## Boundary Families

Spectral Dynamics and Clipper may be represented for inspection, but their
registry surfaces are `inspect_only` and their PCA family is not executable.
They cannot enter a `needs_action` route. Clipper is not a Limiter alias and
does not share Limiter PCA, coverage, planner, or typed controller.

## Loaded Instances

Qualification is represented as independent `qualified processor surface`
rows. One loaded instance may expose EQ, compressor, or other independently
recognized surfaces. A deterministic priority does not collapse a mixed
instance into one family. If two surfaces claim the same parameter, all
conflicting surfaces are marked `ownership_conflict` and fail closed.

Before family selection, loaded identity and topology are withheld. After a
family is selected, only matching candidate envelopes are disclosed. Exact
identity and live topology are revalidated before materialization.

## Mutation Boundary

An active free-state loop may request only read-only CCB observation. Typed
apply, plug-in loading, `plugin.set_parameter`/`set_plugin_param`, generic
fallback writes, and `daw.invoke` wrappers are rejected by the AgentLoop,
internal workflow executor, and HTTP `/agent/invoke` boundary. Confirmation,
transactional typed execution, readback, restore, and snapshot verification
remain downstream authorities.

