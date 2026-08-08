# PCA-Constrained Agent Plugin Gate v1

The Agent has two independent fail-closed checks for processor plug-in
selection and loading.

1. Recommendation computes a deterministic processor family and required
   action coverage. It reads the PCA v1/v2 store and the current installed
   binary fingerprint. Only promoted records with matching stable identity,
   current fingerprint, matching family, and complete coverage are exposed to
   the model. The model receives an exact identifier and opaque candidate key;
   executable paths and PCA internals are not model evidence.
2. The Harness runs a second check immediately before Kernel dispatch. It
   requires an in-process authorization object, compares the exact track,
   path, and identifier, recomputes the binary fingerprint, rereads PCA, and
   rejects any status, family, coverage, or attestation change.

Missing PCA, issued-only records, stale or revoked records, changed binaries,
wrong family, insufficient coverage, missing identifiers, Spectral Dynamics,
and Clipper are rejected. Clipper never consumes Limiter PCA.

The gate applies only to Agent-originated `plugin.load_to_rack`,
`rack.add_node`, `rack_add_node`, and `plugin.instantiate` loads. The DAW/UI
manual loading path calls the native rack service directly and is unchanged.

The PCA certification runner is the sole loading exception. After explicit
consent, the server creates a short-lived, single-use token bound to the exact
identifier, installed path, family, subject key, and pre-load fingerprint.
The token is consumed by the load request and converted to an in-process
authorization object; a forged `source` string is insufficient. Existing
temporary-track creation, snapshot, typed apply/readback/restore, cleanup, and
no-project-save boundaries remain in force.

Every rejection is returned with a stable `pca_load_gate:` reason and is
recorded by the Harness pre-journal failure audit path.
