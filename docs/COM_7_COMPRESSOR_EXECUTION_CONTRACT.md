# COM-7 Compressor Semantic Execution Contract

Status: implementation contract
Date: 2026-08-04

## Purpose

COM-7 closes the execution boundary after a valid COM-6 planning result. It converts a planning-only
`semantic_effect.compressor_plan.v1` into a generation-scoped internal execution ticket, pauses for
explicit Vit confirmation, then uses the existing atomic compressor controller to apply and verify
the requested physical or enum targets.

COM-7 is an execution workflow, not a new compressor recognizer and not a second LLM decision loop.

## State Machine

```text
COM-6 plan
  -> deterministic materialization
  -> waiting_for_user confirmation
       -> cancel: no write, completed/cancelled
       -> approve: fresh target + topology + baseline checks
            -> fresh paired_io before projection
            -> existing atomic compressor controller
            -> full readback and parameter audit
            -> fresh paired_io after projection
            -> read-only COM change_delta
```

Any failure before or during mutation stops the workflow. The existing controller restores the full
preimage on write, readback or unplanned-change failure. If post-action COM evidence cannot be
captured, the controller result remains auditable and the receipt reports evaluation unavailable;
there is no automatic retuning loop.

## Materialization

The deterministic materializer receives the typed COM-6 plan and the current local compressor
surface. It resolves every proposal by `path_key + role`, obtains the current generation-scoped
`control_ref` locally, and converts the absolute target to the existing typed control request.

The materializer records internally:

- exact track and plugin instance;
- topology generation;
- full parameter-count and normalized-value baseline fingerprint;
- proposal IDs, semantic axes, roles and local control references;
- absolute physical or enum targets;
- the frozen evaluation contract and evidence identity.

Control references, parameter IDs and normalized values never enter the LLM plan or the visible
confirmation preview. They are internal execution data only.

Materialization rejects when the plan is not valid, the plan target differs from the live target, the
COM evidence is not `paired_io / ready` for semantic execution, a selected role is absent, a target
unit/domain is unreachable, or the current topology/baseline does not match the captured state.

## Confirmation

Vit receives a dedicated `semantic_compressor_execution` interaction. The preview contains only the
processor, semantic axes, roles, current displayed values, absolute proposed values, confidence and
the frozen evaluation statement. It does not show parameter IDs, control references or normalized
curves.

No parameter write is permitted before the explicit `approve` action. `cancel` expires the ticket and
leaves the plugin unchanged. Tickets are tied to the conversation, goal, run, target, topology
generation and baseline fingerprint.

## Product Observation Window

The Godot chat path supplies selected clip/range/time context in seconds. For semantic compressor
execution only, the Agent resolves that context before the first paired capture: an explicit selected
range wins; otherwise the playhead inside the resolved clip, or the clip start, anchors a bounded
10-second window. The kernel converts this window using its authoritative render sample rate and the
terminal paired receipt freezes the exact clip, `start_sample`, `end_sample`, sample rate and channel
count into the server-side ticket. Confirmation-time before/after captures reuse that exact window.

Selecting a clip is not itself an explicit time-range selection. The selected clip supplies identity
and bounds; only a real timeline time selection, a clip-range selection, or an exact caller-supplied
sample window prevents automatic window choice. This keeps long clips from being mistaken for an
unbounded observation request.

If the automatically chosen 10-second window yields `paired_io / partial` solely because event
coverage is insufficient, the Agent may perform up to three additional read-only probes centered near
the clip's 25%, 50% and 75% positions. The first `paired_io / ready` result becomes the frozen window.
An explicit user range is never relocated. If all bounded candidates remain partial, materialization
still fails closed and no parameter write occurs. These probes happen only before confirmation and are
not an automatic post-write tuning loop.

Paired event selection uses deterministic 150 ms non-maximum suppression. Within each local exclusion
neighborhood the strongest onset candidate wins, and selected events are at least 150 ms apart. This
prevents dense music from being transitively collapsed into one event by chained adjacent-candidate
clustering. Selection is independent of input candidate ordering.

Recovery evidence is `ready` only when at least three recovery events are observable and observable
coverage is at least 80 percent. One locally unobservable event does not demote an otherwise well-covered
music window, while sparse or poorly covered recovery evidence remains `partial` or `missing`.

This product resolver does not relax the general COM tool contract: direct `paired_io` callers must
still provide an exact sample window unless they are the governed semantic-compressor workflow.

## Execution And Audit

Approval re-reads the live compressor surface and fails closed if target, generation, parameter count
or baseline fingerprint changed. The existing `plugin_grabber.apply_compressor_controls` transaction
then performs the writes with `atomic=true`, reads every target back, detects non-target changes and
retains an exact restore reference.

COM-7 adds a full parameter audit around the controller result:

- target readback is exact or explicitly quantized;
- non-target parameters retain their preimage values;
- parameter count is unchanged;
- restore/rollback status is explicit and verified on failure.

Before and after paired observations use the same source, clip, sample window, tap identity, analyzer
and deterministic render policy. They are combined with `com.ModeChangeDelta`. COM receives no control
values and the result is read-only; user acceptance remains separate from the bounded behavior delta.

COM-7 paired captures explicitly use the general band/stereo MOM observation intent. The compressor
dual-tap projection remains governed by COM's own readiness gates; compressor wording in the internal
capture goal must not reroute the capture through MOM's general action-preflight gate or require unrelated
spectral, stereo and loudness layers to become ready first.

## Product-Path Acceptance

The repaired path was accepted on 2026-08-04 using the normal Godot project lifecycle. Godot 4.6.1
started the production `VitApp.exe`, `VspHub.exe` and `VitAgent.exe`; the loaded project supplied track
`1007`, clip `1011` (`Paper Crown`) and Pro-C 2 instance `1015`. The semantic request supplied the same
selection identity fields as the Godot chat controller and no explicit time or sample window.

Acceptance evidence:

- the automatically resolved COM observation reached `paired_io / ready`;
- the proposal reached `waiting_confirmation` with zero parameter writes;
- approval completed with `status=executed`, `mutation_performed=true` and controller `atomic=true`;
- the full parameter audit passed and all non-target parameters were unchanged;
- independent parameter reads showed Threshold `-18.00 -> -14.00 dB`, Attack `0.255 -> 9.997 ms`,
  Release `209.2 -> 349.9 ms`, Auto Gain `On -> Off`, and plugin-quantized Output Level `+0.03 dB`;
- the post-action observation produced `change_delta / ready` with no automatic iteration.

This acceptance drove the product through Godot ownership and real project material. HTTP was used only
to submit the controller-equivalent chat and confirmation messages because UI automation was explicitly
out of scope; no standalone Kernel, Hub or Agent process was substituted for the Godot-owned children.

## Non-Goals

COM-7 does not support limiters, multiband compressors, C2 orchestration, Profile, VPS, SPAL,
Plugin Alliance product mappings, automatic iteration, or natural-language-to-fixed-increment rules.
Exact parameter requests continue to use the existing typed compressor control path.
