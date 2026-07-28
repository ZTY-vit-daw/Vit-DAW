# EQ VPS Control Graph v1

## Purpose

`vit.eq_vps.control_graph.v1` is a data-only, externally authored description
of how public EQ edits map to one installed plug-in's parameters. The runtime
must not know plug-in names, manufacturers, parameter IDs, or named compound
structures such as "signed magnitude". A new plug-in is integrated by adding a
VPS document and a verification attestation, never by adding executable code.

The graph compiles into parameter targets consumed by the existing
`plugin_grabber.apply_eq_edits` transaction executor. Preimage capture, batch
write, fresh readback, unexpected-change detection, rollback, formal undo, and
zero-drift checks remain runtime-owned and cannot be changed by a VPS.

## Trust boundary

A VPS is declarative data, not a script. The decoder rejects unknown fields.
Expressions are finite trees with bounded depth and node count. They cannot
contain loops, named function calls, filesystem/network access, host commands,
or arbitrary code. The only possible effect is a target for a binding declared
in the same document; every binding is revalidated against a fresh Vit
parameter surface before compilation.

The loader also bounds document/attestation size, directory entries, bindings,
sections, curves, enums, programs, inputs, and writes. These limits are runtime
policy and cannot be raised by a VPS.

Runtime use requires a sidecar `vit.eq_vps.verification.v1` attestation whose
VPS hash and parameter-surface signature match both the document and the live
plug-in. Missing or contradictory evidence fails closed.

## Document outline

```json
{
  "schema_version": "vit.eq_vps.control_graph.v1",
  "plugin": {
    "name": "Example EQ",
    "format": "VST3",
    "version": "1.0",
    "manufacturer": "Example"
  },
  "surface_signature": "p_example",
  "channel_contract": "shared",
  "bindings": {
    "gain": {
      "kind": "number",
      "parameter_id": "42",
      "parameter_name": "Gain",
      "channel": "shared",
      "domain": {
        "unit": "dB",
        "minimum": -18,
        "maximum": 18,
        "curve": [[0, -18], [0.5, 0], [1, 18]]
      }
    }
  },
  "sections": [],
  "provenance": {},
  "unresolved": []
}
```

Bindings are either `number` or `enum`. Numeric curve points and enum
normalized values/labels are observed facts, not author guesses. Section-local
binding aliases refer to keys in the document-level binding table, so reusable
program shapes never contain real parameter IDs.

## Section, capability, and program

Each resident or allocatable section publishes public shapes and actions. An
action contract declares exactly which public inputs are required or optional.
Supplying any undeclared input rejects the whole edit before writes.

```json
{
  "section_key": "top",
  "addressing": "resident",
  "activation": {"state": "always_active"},
  "selection": {"frequency_binding": "frequency"},
  "bindings": {
    "frequency": "top_frequency",
    "gain": "top_gain",
    "mode": "top_mode"
  },
  "shapes": {
    "bell": {
      "actions": {
        "upsert": {
          "program": "set",
          "required_inputs": ["frequency_hz", "gain_db"]
        },
        "modify": {
          "program": "set",
          "optional_inputs": ["frequency_hz", "gain_db"]
        }
      }
    }
  },
  "programs": {
    "set": {
      "inputs": {
        "frequency_hz": {"type": "number"},
        "gain_db": {"type": "number"}
      },
      "writes": []
    }
  }
}
```

`upsert`, `modify`, `disable`, and `remove` are the only forward actions. Undo
is journal-owned and never authored as an inverse graph.

## Expression language

The v1 expression set is deliberately small and composable:

- values: `input`, `constant`;
- numeric: `abs`, `negate`, `add`, `subtract`, `multiply`, `divide`, `minimum`,
  `maximum`, `clamp`;
- boolean: `present`, `less_than`, `less_or_equal`, `greater_than`,
  `greater_or_equal`, `equal`, `and`, `or`, `not`;
- branching: `select`, `lookup`.

Expressions are typed as `number`, `string`, or `boolean`. Numeric bindings
accept only numeric results. Enum bindings accept only string keys present in
that binding's verified value table. Write conditions accept only booleans.

An elysia-style non-negative gain plus Boost/Cut polarity is data, not a named
runtime structure:

```json
{
  "writes": [
    {
      "binding": "gain",
      "role": "gain",
      "when": {"op": "present", "input": "gain_db"},
      "value": {"op": "abs", "args": [{"op": "input", "input": "gain_db"}]}
    },
    {
      "binding": "mode",
      "role": "gain_polarity",
      "when": {"op": "present", "input": "gain_db"},
      "value": {
        "op": "select",
        "condition": {
          "op": "less_than",
          "args": [
            {"op": "input", "input": "gain_db"},
            {"op": "constant", "value": 0}
          ]
        },
        "then": {"op": "constant", "value": "cut"},
        "else": {"op": "constant", "value": "boost"}
      }
    }
  ]
}
```

A conventional bipolar gain uses the same public input directly:

```json
{"binding":"gain","role":"gain","value":{"op":"input","input":"gain_db"}}
```

Program writes can fan one public input out to any finite set of declared
bindings, including left/right channel pairs. A write may be marked
`phase: activation`; the compiler places all such writes last. VPS data cannot
change any other transaction ordering or safety rule.

Numeric writes always receive exact normalized readback validation. Physical
display parsing, tolerance checks, and iterative correction are strongest for
the existing executor roles `freq`, `gain`, `q`, and `slope`. V1 documents
intended for verified installation should use those roles for physical EQ
targets. A nonstandard numeric role is not evidence of equivalent physical
verification and must be treated as a review limitation until a generic,
runtime-owned physical readback contract exists.

## Verification and installation

The external directory defaults to `%APPDATA%/Vit/Agent/vps/eq-control-graph`
and can be overridden for tests. Each `*.vps.json` must have a matching
`*.verification.json` sidecar. Verification performs:

1. strict schema and expression type validation;
2. fresh identity and complete surface-signature match;
3. exact parameter ID/name validation;
4. exact observed curve-point and enum-value/label validation;
5. representative compile checks for every published action;
6. live apply/readback/formal-undo/zero-drift cases before an attestation is
   promoted for installation.

Directory resolution is repeated from live state, so adding or removing a VPS
is reversible without changing production recognizer rules. A matching but
invalid/stale document fails closed; absence of a matching document preserves
the original generic recognizer result.

The reference CLI separates the trust stages:

```text
eqvps validate  -vps <plugin.vps.json>
eqvps candidate -vps <plugin.vps.json> -surface <page_0000.json> [...]
eqvps promote   -vps <plugin.vps.json> -smoke <A_B_C_summary.json>
```

`candidate` requires complete fresh Vit pagination and writes a hash-bound
sidecar, but it is not installable by default. The test-only
`VIT_EQ_VPS_ALLOW_CANDIDATE=1` override exists solely to run isolated smoke
verification. `promote` accepts only a passing generic Control Graph A/B/C
summary with actual readback, verified formal undo, final zero drift, unload
restoration, and zero prohibited audio/learn/profile/SPAL/B4 calls. Production
loading accepts `verified` sidecars only.

## Product acceptance invariant

After this platform is established, adding a plug-in may change only:

- `<plugin>.vps.json`;
- `<plugin>.verification.json`.

Any plug-in name, manufacturer, parameter ID, or compound-structure tag added
to Go or to the generic verifier is a platform failure, not a normal extension.
