# Free-State Diagnostic-Only Contract v1

This contract isolates the open-ended, whole-project diagnosis question from
processor-family selection and A-F execution. It is a read-only experiment
boundary, not a replacement for the governed mixing capabilities.

## Model output

An active diagnostic-only loop may request only CCB observation tools. Its
terminal `free_state` decision must include:

```json
{
  "diagnostic": {
    "schema_version": "free_state_diagnostic.v1",
    "status": "confirmed|ruled_out|unresolved",
    "findings": [{
      "statement": "model-authored evidence-grounded statement",
      "scope": {"kind": "track|track_pair|project", "ids": ["visible id"]},
      "evidence_refs": ["observation_id or returned evidence_ref"],
      "confidence": 0.0,
      "limitation": "optional"
    }],
    "limitations": ["optional"]
  }
}
```

`confirmed` and `ruled_out` require at least one finding. `unresolved` may
contain no finding when the model can state why the available evidence cannot
settle the question. Every finding requires a model-authored statement and at
least one evidence reference.

## Runtime boundary

The runtime validates the schema, status, statement, confidence range, and
that each evidence reference is present in the current free-state observation
ledger. It does not evaluate the statement, inject expected fixture issues,
choose a target, infer a family, or disclose sealed evaluator truth.

Diagnostic-only turns reject `needs_action`, `semantic_processor_intent`,
processor selection, plugin loading, typed controller calls, transactions,
readback, and post-action execution. Continuation and checkpoint snapshots
retain only the compact diagnostic conclusion and observation references.

A `project.structure` observation may be used once to discover visible track
identities. After the first usable non-structural CCB bundle is returned, the
runtime closes the diagnostic observation window. The model must then author a
terminal `confirmed`, `ruled_out`, or `unresolved` conclusion from the visible
facts and limitations. The runtime does not choose that conclusion and does not
convert measurements into a finding; it only rejects another observation call.

The smoke runner's `--diagnostic-only` mode stops at the first valid terminal
diagnostic conclusion and records it in the checkpoint and run report. It is a
short read-only replay mode; it does not authorize the formal long smoke run.
