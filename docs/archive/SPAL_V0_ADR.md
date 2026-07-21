Status: superseded by ADR-AGENT-CLEANUP-0001

# ADR-SPAL-0001: Semantic Parameter Application Layer v0

Status: accepted

## Decision

Vit uses **SPAL** as the formal name for the **Semantic Parameter Application
Layer**. SPAL exposes a finite, versioned, vendor-neutral semantic control
language to capability layers. A capability chooses a semantic control and its
values; SPAL resolves a verified Provider instance, captures a preimage,
compiles physical parameters, and executes the frozen action only through the
existing Proposal, Project Cut, Coordinator, VSP, Receipt, and rollback path.

SPAL does not judge whether a mixing action is artistically correct. It
guarantees that a selected action is applied faithfully or explicitly refused.

## v0 control language

SPAL v0 publishes exactly one canonical schema:

```text
spectral.static_bell.v1
  center_frequency_hz
  gain_db
  q
```

The schema is a stable API. Provider-specific parameter IDs, enum values,
plug-in names, and tool workflows are not visible to capability code.

## Provider policy

1. Use only a verified instance already bound to the target.
2. Do not fall back to TDR Nova or any other built-in plug-in when no verified
   instance is available.
3. Return `no_verified_provider` and request Plugin Learning / a Plugin Skill
   when no Provider instance can satisfy the requested schema.
4. A verified Provider may be provisioned only by a separate, confirmed
   Proposal. Provisioning is not a fallback resolver action.

TDR Nova is an experimental laboratory Provider and conformance fixture only.
It is never a production default.

## Runtime invariants

- `Plugin Learning -> Adapter Candidate -> Conformance -> Verified Provider`
- a Provider only exposes canonical schemas it can prove it implements
- a static Bell binding requires a verified Bell-shape invariant; SPAL never
  guesses a vendor enum value
- a frozen manifest stores the Provider signature, instance binding, physical
  writes, complete preimage, expected signal direction, optional frozen
  same-tap verification scope, and evidence refs
- VSP batches are non-atomic, so every failed or mismatched batch attempts a
  compensation against the complete preimage
- compensation fails closed if current values are neither frozen preimage nor
  the write attempted by this execution
- structural verification, signal-direction verification, and musical/user
  acceptance are separate results
- a signal-direction mismatch never rewrites a successful parameter receipt as
  a failed mutation; it remains a durable review finding
- even a passing signal-direction check does not complete musical/user
  acceptance, which remains explicitly `unknown`

## Implemented v0 seams

```text
agent/internal/spal
  finite schema, Registry, Runtime, manifest, Provider conformance,
  experimental TDR Nova Adapter, signal-direction comparison

agent/internal/capabilityadapters/spal.go
  capability-layer planning and Proposal freezing seam

agent/internal/executionports/spal_vsp.go
  VSP batch execution, readback, compensation, reconciliation, Receipt facts

agent/internal/executionports/spal_l2_signal.go
  frozen-scope before/after L2 render capture, precise target-band request,
  and durable signal evidence attached to the Action Receipt

agent/internal/executionverifiers/spal.go
  structural/readback and optional L2 signal verification separation

agent/internal/spallab and agent/cmd/spallab
  developer-only TDR Nova lab conformance, persistent verified-instance store,
  frozen Proposal / separate approval / execution, automatic same-tap L2
  signal evidence when a bounded scope is supplied,
  and separately confirmed fail-closed rollback Proposal
```

## Explicit non-goals

- arbitrary VST control
- raw plug-in parameter exposure to capability/Agent code
- dynamic EQ, compression, sidechain, reverb, automation curves
- automatic artistic optimization or autonomous replanning
- automatic TDR Nova loading as a fallback
- migration of B1-B3
- exposing the SPAL Lab CLI as a normal B4/chat user capability
