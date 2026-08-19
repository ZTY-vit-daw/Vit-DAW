# G4 Real Kernel Audition Audio Plane Spike

- **Baseline:** `de652ed`
- **Branch:** `codex/g4-gap-real-audition-audio`
- **Scope:** `target` + `source_kind=audio_file`
- **Authority:** Kernel/JUCE; WebUI remains control/presentation only.

## What is real in this spike

`audition.prepare` validates two recoverable candidate identities, decodes each
source file on the Kernel message thread, resamples to the Kernel device sample
rate, and stores immutable Kernel-owned `juce::AudioBuffer<float>` sources.
Readiness is emitted only after both sources are decoded and prepared.

The Kernel installs an `AuditionPreviewAudioPlane` as Tracktion
`DeviceManager::setGlobalOutputAudioProcessor`. When preview playback is active,
the processor replaces the final output block with the selected prepared source.
The selected source is read on the audio thread without file I/O or allocation.
Candidate changes are observed at the start of an audio block and use a bounded
128-sample crossfade. The preview position advances by audio blocks and is not
implemented by changing the Active Project transport.

## Safety boundaries

- `audition.select` only selects an already-prepared audio buffer.
- It does not render, checkout, open a project, reload the Kernel, or mutate the
  Active Project Plane.
- `audition.inspect_candidate` and `audition.apply_candidate` are explicit
  commands with an intentional `capability_not_supported` response in this
  target audio spike; they are not hidden inside select.
- `stale` and `failed` invalidate the preview playback plane and prevent select.
- Unsupported source kinds and non-target scopes fail preparation explicitly.

## Remaining boundary

This is not a full-project audition renderer. `checkpoint`, `branch`,
`worktree`, `experiment`, and `render_artifact` sources are identity-bearing
contract inputs but are not rendered by this spike. Full-project graph/render,
plugin/media revision invalidation, loop/tempo-aware transport alignment,
and GUI inspect/apply remain future work.

## Evidence

- `VitAuditionPreviewAudioPlaneTests` writes two different WAV files, decodes
  both, renders blocks through the same output-processor code path, verifies
  different RMS values, verifies position continuity, and verifies crossfade.
- `VitAuditionPreviewStateTests` covers ready gating, stale/failed rejection,
  plane isolation, position, and stop.
- VitApp Debug builds with the audio plane wired into the real Kernel target.
