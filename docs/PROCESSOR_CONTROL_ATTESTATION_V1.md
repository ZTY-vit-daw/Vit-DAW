# Processor Control Attestation v1

PCA v1 is a global, per-machine admission library for deterministic plug-in
control. It is stored outside project files at:

`C:\Users\<user>\.vit\processor_control_attestations.v1.json`

The path can be overridden for tests with
`VIT_PROCESSOR_ATTESTATIONS_PATH`. A project therefore does not rescan its
plug-ins when it is opened. The semantic plug-in index remains the catalog;
PCA is the separate evidence-backed control admission layer.

## Scope

v1 covers only `static_eq` and `broadband_compressor`. Limiter, de-esser,
transient shaper, and C2 are intentionally outside the vocabulary and are not
admitted by the store or recommendation boundary.

## Badge Contract

Each badge records only:

- stable subject identity (`subject_key`, name, manufacturer, format,
  identifier, installed path);
- current installed-binary SHA-256 fingerprint;
- processor family;
- action-only coverage (`upsert`/shape for static EQ, or `adjust`/semantic axis
  for compressors) retained as historical test evidence;
- evidence references, issuer, immutable digest, and lifecycle timestamps.

Badges never store `param_id`, parameter mappings, Profile, VPS, SPAL logic, or
`topology_generation`. A badge admits a verified binary into the governed load
lifecycle; it is not an execution plan or a declaration that every possible
control is available.

## Lifecycle

The deterministic authority issues a badge from a strong receipt, promotes the
current fingerprint, and marks superseded promoted badges `stale`. Operators
can mark a badge `stale` or `revoked` with an explicit reason. The load-admission
query requires the exact subject, current binary fingerprint, family, and
promoted status. A fingerprint mismatch is effectively stale even when the
stored status is promoted.

Coverage is retained as evidence of the concrete reversible tests performed at
certification time. It can be queried for audit, regression analysis, and
backward-compatible reports, but it is not a request-time permission check for
a new shape or semantic axis.

`pcactl` provides `import`, `certify-compressor`, `query`, `list`,
`fingerprint`, `promote`, `stale`, and `revoke`. Import is idempotent for an
unchanged evidence set and promotes one current badge per subject/family.

## Evidence Order

The authority first consumes existing strong receipts:

- Waves static EQ live smoke;
- Plugin Alliance EQ two-level live smoke;
- Plugin Alliance compressor regression, fix, and blind reports.

Recognizer-only compressor matrices are held as evidence gaps because they do
not prove typed write, actual readback, restore, and cleanup. The
`pcactl certify-compressor` path is the fallback for an exact semantic-index
identifier. It performs health check, disposable track creation, exact load,
full snapshot capture, two stable compressor inspections, deterministic
reversible typed apply/readback, restore, full snapshot comparison, and track
deletion. It writes a generic receipt and imports it through the same authority.
No chat endpoint, prompt, or LLM client is involved.

## Runtime and Recommendation Boundary

PCA filters the local catalog to exact identities with a promoted attestation,
matching current binary fingerprint, and the requested processor family. Only
those candidates are sent to the recommendation LLM. Selection confirmation
repeats that family-level admission query against the global store.

After load, the live EQ/compressor topology and its current
`topology_generation` remain the execution authority. The Typed Inspector
identifies controls available on the loaded instance; the Typed Executor decides
whether a concrete action can be applied and performs write, readback, and
restore. PCA does not replace those guards.

## Experiment Reuse

Future recognizer experiments can use the same disposable-track certification
runner and receipt adapter. A new recognizer does not need a new DAW project:
run it against an exact semantic-index entry, retain the receipt under an
artifact directory, and import only if all strong gates pass. The global badge
library is the reusable test admission record; it is not copied into projects.
