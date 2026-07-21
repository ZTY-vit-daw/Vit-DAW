# VPS Forge VST3 worker

`vpsforge_vst3_worker` is the native, one-plugin-per-process worker used by
the Vit VPS Forge loopback adapter. It accepts one JSON object per stdin line
and writes one JSON object per stdout line. It has no network listener and no
Vit dependency.

The parent adapter owns HTTP, staging workspace writes, session supervision and
crash reporting. This process owns only a loaded VST3 instance and therefore
contains plugin crashes to the worker process.

Supported worker operations are `load`, `snapshot`, `write`, `save_state`,
`restore_state`, `roundtrip_state`, `rollback`, `render`, `unload`, and
`shutdown`. All writes capture a full state-and-parameter preimage before any
mutation and return a fresh parameter readback.

`vpsforge_vst3_witness` is a separate, one-plugin GUI process. Its live
monitor plays a user-started local WAV/AIFF source through the visible VST3 to
the selected Windows stereo output, so a human can listen while operating the
editor. It remains isolated from Vit, the Forge API host, VPS Library,
Credential, Catalog and SPAL routing; closing it discards the transient
instance.
