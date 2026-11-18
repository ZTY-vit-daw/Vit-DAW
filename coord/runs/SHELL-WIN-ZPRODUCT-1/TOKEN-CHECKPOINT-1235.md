# Local usage checkpoint, 2026-11-18 12:35 +08:00

Local event counters only; these are not billing, money or provider quota measurements. The model-switch rollout continues the supervisor's cumulative counters, so the pre-switch and post-switch supervisor totals must not be added together.

11:30 baseline: last supervisor token event at 11:29:40, input 1,022,810, cached input 802,176, output 7,810. Supervisor latest sample 12:34:42, first worker 12:20:03, replacement worker 12:34:38.

| Stream | Input | Cached input | Uncached input | Output |
|---|---:|---:|---:|---:|
| Supervisor since baseline | 17,112,435 | 15,428,736 | 1,683,699 | 61,089 |
| First execution worker | 6,003,173 | 4,638,208 | 1,364,965 | 46,468 |
| Replacement execution worker | 1,353,626 | 989,568 | 364,058 | 18,756 |
| Combined | 24,469,234 | 21,056,512 | 3,412,722 | 126,313 |

The 12:03 stage-one estimate was approximate. Use raw event timestamps and this subtraction method when comparing later checkpoints. Source: local rollout token-count events for supervisor `01a1eca3…`, first worker `01a1ecb4…`, replacement worker `01a1ece5…`; raw conversation contents were not copied into this report.

Acceptance at checkpoint: narrow startup fixes and no-window builds/tests have passed; official 4.6.1 DLL registration has passed in an isolated headless project; no product runtime lease, window run, final acceptance, production merge or user hand-test approval. The only in-flight work is the 409 smoke preparation and its minimal in-process observations.
