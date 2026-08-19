# Project Change Receipt v1

## Purpose

`project_change_receipt.v1` is the deterministic engineering-change surface
between the Shadow Project / Mix Board and the free-state Agent loop. It tells
the model what changed in the DAW state and which observation scopes should be
refreshed. It does not claim that an acoustic or musical outcome improved.

## Authority and freshness

- `authoritative_snapshot`: derived from a complete DAW/VSP project snapshot.
- `executor_delta`: low-latency post-command state hint produced by a governed
  Executor. It remains `pending_authoritative_refresh` until a project snapshot
  confirms the state.
- `telemetry_delta`: low-latency external telemetry hint. It has the same
  pending refresh rule and may be lost by transport, so it is never the sole
  source of project truth.

Each receipt carries `from_state_epoch` and `to_state_epoch`. Acoustic CCB
receipts must bind to the current project revision separately; an engineering
receipt cannot substitute for acoustic evidence.

## Model-visible view

The CCB semantic view `project.change_delta` exposes a bounded receipt with:

- changed entities and before/after engineering values;
- the project revision/epoch transition;
- affected engineering/observation scopes;
- whether the transition is authoritative or awaiting refresh;
- explicit limitation that no acoustic consequence is established.

The Shadow Project retains a bounded history for audit and comparison. The
Context Assembler should place only the latest receipt and a small number of
references in hot/warm context; full state snapshots and receipt history stay
outside the model window.

## Scope rules

Track gain/level changes affect level, relationship, headroom and masking
scopes. Pan changes affect stereo and relationship scopes. Plugin-chain changes
affect processor and downstream relationship scopes. Clip or structural changes
affect project/track observations and downstream relationship scopes. These are
refresh obligations, not acoustic conclusions.

