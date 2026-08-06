# Plugin Alliance Compressor Control v1 Blind Round 1

## Verdict

The frozen round-1 verdict is **failed**.

- Total: 18/18 executed.
- Passed: 9.
- Failed: 9.
- Positive end-to-end pass rate: 7/12 (58.3%).
- Positive inspect recognition rate: 10/12 (83.3%).
- Negative rejection pass rate: 2/6 (33.3%).
- Overall pass rate: 9/18 (50.0%).

This result is evaluation evidence only. No recognizer, role rule, tolerance, control code, test
expectation, or training fixture was changed after freeze. The opened 18-case cohort is no longer
blind and must not be used to claim a later v2 blind pass.

## Freeze Integrity

| Item | SHA-256 |
| --- | --- |
| Blind manifest | `a9f62a7cda18901f711109a9c625011619efb5718004cac990018627dd3d1e22` |
| Blind runner | `5ce413580ff0cb0c99110134e528b6feb0c0da494b3263418a739f0fa5999f96` |
| Production Agent | `82a7de7d6291ee8df0cd4e2e110a879808c2b44761bd50f239756388618a2483` |
| Recognizer/control closure | `72ed72acb2547960a63ead44ef9d8f7926815c4e2909b6d715b4a1d35dbbca45` |
| Evidence index | `cbae4a4cdd2dc1c7a3b4bfc3f9ed867f8e06b1d34174f3a0941143c1e42e8279` |
| Conclusion JSON | `1e3c9234802a6253f929cf86dd1c263570937937cf2206c94b65211de9a043fa` |

Freeze verification passed both immediately before and immediately after execution with zero file
mismatches. Preflight uniquely resolved all 18 product identities but loaded no plug-in and read
zero parameter surfaces. The 12-case reserve set remained sealed with zero parameter reads.

## Case Results

| Case | Expected | Result | Deterministic finding |
| --- | --- | --- | --- |
| Acme Opticom XLA-3 | compressor | fail | False negative: `not_compressor` |
| AMEK Mastering Compressor | compressor | fail | Recognized; apply and restore reported exact, but full snapshot drifted |
| elysia alpha compressor V2 | compressor | pass | `threshold_driven`; threshold/input exact and full restore |
| elysia mpressor | compressor | fail | Recognized, but no preferred binding exposed a reversible measured target |
| Lindell 254E | compressor | pass | `threshold_driven`; threshold/ratio exact and full restore |
| Maag MAGNUM-K | compressor | pass | `bidirectional_curve`; threshold/input exact and full restore |
| Purple Audio MC 77 | compressor | pass | `multi_path_channel_control`; input/ratio exact and full restore |
| Shadow Hills Mastering Compressor | compressor | pass | `threshold_driven`; threshold/ratio exact and full restore |
| SPL IRON | compressor | pass | `multi_path_channel_control`; threshold/input exact and full restore |
| TBTECH Cenozoix Compressor | compressor | fail | False negative: `not_compressor` |
| Unfiltered Audio Zip | compressor | fail | Recognized; threshold target read back as `-inf dB`; atomic rollback succeeded |
| Vertigo VSC-2 | compressor | pass | `threshold_driven`; ratio/attack exact and full restore |
| bx_limiter True Peak | not compressor | fail | False positive: `hybrid`, confidence 0.82 |
| Lindell MBC | not compressor | fail | False positive: `multi_path_channel_control`, confidence 0.96 |
| SPL De-Esser Dual-Band | not compressor | pass | Rejected `not_compressor`; full snapshot unchanged |
| SPL Transient Designer Plus | not compressor | pass | Rejected `not_compressor`; full snapshot unchanged |
| Unfiltered Audio G8 | not compressor | fail | False positive: `threshold_driven`, confidence 0.96 |
| Pro Audio DSP DSM V3 | not compressor | fail | False positive: `threshold_driven`, confidence 0.96 |

## Failure Taxonomy

### False-negative recognition

`Acme Opticom XLA-3` exposes Input Gain, Response, Output Gain, Dry/Wet Mix, and related utility
controls but no parameter names that satisfy the frozen broadband-stage evidence threshold.

`TBTECH Cenozoix Compressor` exposes explicit Input Gain, Threshold, Ratio, Knee, Range, attack,
release, hold, and lookahead controls. Its large sidechain EQ surface contains multiple numbered
bands, which causes the frozen multiband exclusion to reject the complete product even though the
numbered bands belong to the detector EQ rather than multiple gain-reduction bands.

### Recognized but not reliably controllable

`AMEK Mastering Compressor` was recognized as `threshold_driven`. Threshold and Ratio 1 apply and
restore both reported `exact`, but Ratio 1 returned from normalized 0.5 to 0.508731722831726. The
physical display target returned to 2.0 while the complete normalized preimage did not.

`elysia mpressor` was recognized as `threshold_driven`, but threshold, ratio, attack, and release
each exposed only the current single-point physical observation. No reversible alternate target
could be proven from the frozen measured surface, so no write was attempted.

`Unfiltered Audio Zip` was recognized as `threshold_driven`. The deterministic target chooser
selected -60 dB, which the plug-in displayed as `-inf dB`; physical readback parsing rejected the
transaction and the atomic engine reported that the complete preimage was restored.

### False-positive boundary recognition

`bx_limiter True Peak` published limiter auxiliary evidence, but Input Trim plus Release was enough
for the frozen recognizer to retain a `hybrid` compressor topology.

`Lindell MBC` exposes Low, Mid, and High threshold/ratio/timing families without literal numbered
`Band N` tokens. The frozen multiband exclusion did not treat those named frequency regions as
bands and instead emitted three control paths.

`Unfiltered Audio G8` published gate/expander auxiliary evidence, but Threshold plus timing and
sidechain controls were accepted as a `threshold_driven` compressor path.

`Pro Audio DSP DSM V3` exposes a broadband-looking compressor ratio alongside several spectral
threshold controls. The frozen recognizer collapsed them into one `threshold_driven` path instead
of rejecting the spectral dynamics topology.

## Safety And Cleanup

All 18 temporary tracks returned successful `track.delete` responses. A final independent
`track.list` audit found no track whose name started with `PA blind v1`.

The seven positive passes restored their complete parameter snapshots. AMEK exposed one residual
normalized mismatch before its disposable track was deleted. Zip reported verified atomic
rollback. Cases that failed before mutation issued no typed apply call. The four false-positive
negative cases failed at the rejection gate; no mutation tool was issued, but their post-inspect
snapshot gate was not reached and is therefore not claimed as measured.

## Evidence

Authoritative artifacts are under:

`VitApp\Workspace\Artifacts\smoke\product_path_20260804_081349\compressor_control\plugin_alliance_blind_v1`

- `freeze.json`: frozen identities, hashes, policy, and zero-read preflight.
- `run/summary.json`: complete 18-case machine-readable result.
- `run/cases/*/evidence.json`: raw per-case load, parameter, inspect, apply, restore, and cleanup evidence.
- `evidence_index.json`: SHA-256 index for freeze, summary, verifications, and all 18 case files.
- `conclusion.json`: immutable machine-readable verdict and failure taxonomy.

## Next Boundary

Round 1 is complete even though v1 failed. These 18 products may be used only as exposed regression
evidence for a future v2 development cycle. A v2 blind claim requires the sealed 12-case reserve
set; none of those reserve parameter surfaces was opened in this round.
