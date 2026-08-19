# Semantic Pre-Family Disclosure v1

## Purpose

This boundary protects the model-owned treatment-family decision in the
free-state semantic loop. The model requests observations, reads the returned
evidence, and then chooses the next processor family. Deterministic code keeps
the complete DAW state for binding and execution, but does not expose that
state as a family prior.

## Before Family Selection

The MessageLoop model projection contains only:

- the user's current acoustic goal;
- target scope identifiers needed for CCB binding;
- bounded project revision/count metadata;
- CCB catalog/request results explicitly requested by the model;
- compact loop status and prior action family/status evidence needed for
  re-evaluation.

It does not contain:

- loaded plug-in IDs, names, products, vendors, paths, or semantic library
  candidates;
- `generic_eq_topology`, compressor identity cards, or any processor topology;
- recommended family, default coverage axes, expected/default observation
  views, parameter IDs, or control mappings;
- prior assistant history that may contain materialization identity;
- typed plug-in tools, loading tools, or mutation tools in the free-state tool
  catalog.

The only model-visible observation tools in this phase are
`ccb.observation_catalog` and `ccb.observation_request`. View selection remains
model-owned; no natural-language phrase is mapped to a view by the runtime.
The first processor-family action requires a successful CCB request selected
by the model. When a structured decision and tool call are emitted together,
their view-ID sets must be identical; duplicate or mismatched sets fail closed,
and the server cannot add, replace, or infer a view.
The opt-in legacy planner loop is restricted to non-semantic requests; an open
semantic request still enters this neutral MessageLoop path.

## After Family Selection

Once the model returns `free_state_decision.v1` with `needs_action` and one
processor family:

1. The local governed router accepts that family as a hard constraint.
2. Server-side discovery qualifies real loaded instances and filters the
   candidate set to that family.
3. The family-specific strategy prompt receives only exact candidate keys and
   post-family identity labels. It does not receive topology or identity cards.
4. The model selects an exact supplied instance key or returns `load_required`.
5. Only after exact instance binding does the typed family planner receive its
   certified topology/identity card and control surface.

PCA eligibility, freshness, revocation, fingerprint, coverage, typed
controller ownership, confirmation, transaction write, readback, rollback, and
snapshot/COM verification remain deterministic execution authority.

## Rejection and Audit Boundaries

The runtime must fail closed when an observation is insufficient, a family is
ambiguous, a candidate is not eligible, or an exact instance key is not in the
supplied candidate set. An unsupported family is reported as a capability
boundary; it is never silently substituted with EQ, Compressor, Limiter, or
another family.

The corresponding tests assert:

- full internal snapshots may retain identity for server execution while the
  model projection does not;
- free-state prompts contain only CCB observation tools;
- target ledgers do not persist plug-in identity;
- family candidate projection filters to the selected family and strips
  plugin IDs, topology, and identity cards;
- exact instance keys are validated against the disclosed candidate set.
