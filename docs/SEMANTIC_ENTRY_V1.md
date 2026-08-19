# Model-Owned Semantic Entry v1

## Purpose

`semantic_entry_decision.v1` is the first decision made for a new ordinary-Agent turn. It classifies the requested interaction without selecting an effect family, plug-in, observation view, control axis, parameter identifier, vendor, product, or path.

The runtime accepts these routes:

| Route | Meaning | Runtime boundary |
| --- | --- | --- |
| `discussion` | explanation, comparison, reasoning, or advice | read-only; no semantic action or project mutation |
| `observation` | inspect/evaluate evidence without changing the project | observation/read tools only |
| `explicit_control` | concrete control operation requested by the user | existing typed control tools, confirmation, transaction, readback, and recovery |
| `open_semantic` | an acoustic outcome is requested while method and family remain open | starts the governed free-state loop; later family choice remains model-owned |
| `other` | ordinary Agent work outside acoustic treatment arbitration | existing Agent capabilities, subject to normal policy |
| `unresolved` | the request cannot be classified safely | no mutation; return an auditable clarification/rejection |

The protocol also carries `target_scope`, `control_mode`, `user_authorization`, `confidence`, and a human-readable `reason`. An unresolved decision must include `rejection_reason`.

## Ownership

The model owns entry classification. Deterministic code validates the schema, route/mode/authorization consistency, and whether the requested target scope is available. It does not infer an effect family from words. A new turn is allowed to start free-state only when the verified decision is `open_semantic` with `control_mode=semantic_loop` and `user_authorization=action_requested`.

The free-state loop remains orchestration memory, not mutation authority. It asks the model for observations and later accepts the model's `processor_type` decision. The governed treatment planner, PCA gate, typed controller, confirmation, transaction, readback, rollback, and snapshot verification remain authoritative for execution.

## No fallback routing

The entry classifier is never replaced by an EQ or Compressor keyword classifier. If the entry model is unavailable, the runtime does not start free-state or invoke family-specific semantic routers. Read-only/non-mutating Agent work may continue, but project and plug-in mutations are denied because target scope and user authorization were not classified. If the model returns `unresolved`, the turn ends without mutation.

The user-provided request context is stripped of semantic-entry fields before classification. Only the server's validated decision is written back into the context and continuation. Unknown JSON fields are rejected, so family, plug-in, parameter, and path injection cannot pass the entry protocol.

## Extension point

Future families such as reverb and delay add observation capabilities and governed planners after the same `open_semantic` entry. They do not add phrase-to-family rules to this protocol. Family selection remains a later model decision constrained by eligible capabilities and their attestation/PCA gates.
