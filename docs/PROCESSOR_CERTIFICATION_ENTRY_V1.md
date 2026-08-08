# Processor Capability Certification Entry V2

The product entry is exposed by VitAgent and the Plugin Manager settings page.
It is plugin-first: one row represents one installed effect plug-in, with a
capability status list attached to that row. Scanning and semantic-index
refresh are read-only and never issue PCA badges.

## API

- `GET /agent/processor-certification/candidates?family=<family>` returns
  candidates from the local semantic index plus current PCA status. Omit the
  family (or use `all`) for the unified plugin library view. In that view each
  row includes `capabilities`; a family filter projects one capability back to
  the row-level fields. The API fingerprints a binary only when an attestation
  exists, so an unattested row remains `unattested` without making a topology
  claim.
- `POST /agent/processor-certification/start` accepts an exact `identifier`, a
  certifiable capability family, `confirmed=true`, and the consent token
  `temporary_track_apply_readback_restore`.
- `GET /agent/processor-certification/status?job_id=<id>` returns the durable
  in-process job snapshot and stage progress.

The UI supplies a capability hypothesis, while the Agent owns the
family-to-typed-tool pairing. The typed inspector and certification gates remain
authoritative; wrong-family or composite surfaces fail closed and do not
produce a badge. Selecting `all` is browse-only; certification requires a
specific capability filter.

## Certification contract

The job uses the existing local PCA v2 runner and records progress for health
check, temporary-track creation, plug-in load, parameter snapshot, topology
inspection, typed apply/readback, typed restore, full-snapshot verification, and
temporary-track cleanup. A receipt is imported and promoted only after every
gate succeeds. The current project is not saved, and the temporary track is
deleted before a successful job completes.

The unified directory exposes PCA v1 `static_eq` and
`broadband_compressor`, PCA v2 `limiter`, `gate_expander`, `de_esser`,
`transient_shaper`, and `multiband_dynamics`, plus explicit non-certifying
boundaries for `spectral_dynamics` (inspect-only) and `clipper` (separate
controller family). Broadband Compressor uses the existing PCA v1 runner.
Static EQ attestations are shown, but generic new static-EQ certification is
not offered because no generic runner exists yet.

There is intentionally no batch "inspect all" or automatic certification
operation in this entry. A future whole-library inspection queue must first
produce evidence; only proven positive stages may then be submitted to a
typed certification runner.
