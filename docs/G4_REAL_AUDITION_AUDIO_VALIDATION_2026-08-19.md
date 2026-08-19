# G4 Real Kernel Audition Audio Plane Validation — 2026-08-19

## Branch and baseline

- Branch: `codex/g4-gap-real-audition-audio`
- Starting commit: `6dc340c`
- Integration baseline: `de652ed`
- Tracktion Engine: `D:/Vit_DAW/tracktion_engine`
- Tracktion commit: `0981e7ce`

## Test-path repair

The worktree contains an empty gitlink directory at
`g4-gap-real-audition-audio/tracktion_engine`, so the previous relative test
path silently skipped the real audio target. `VitApp/Tests/CMakeLists.txt` now:

1. accepts `-DVIT_TRACKTION_ENGINE_DIR=...`;
2. falls back to `D:/Vit_DAW/tracktion_engine` when the local gitlink is empty;
3. fails configuration instead of silently omitting the real audio test target
   when no populated Tracktion checkout is available.

Default configure selected:

```text
VIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine
```

## Automated real-audio evidence

`VitAuditionPreviewAudioPlaneTests` passed through the real Tracktion/JUCE-linked
target. It generated two WAV sources, decoded them, resampled them to the
Kernel device rate, held them in Kernel-owned `AudioBuffer<float>` instances,
processed blocks through the output processor, switched A/B sources, verified
position continuity, and verified bounded 128-sample crossfade. It also checked
that the diagnostic render/checkout/project-open/reload counters remained zero.

`VitAuditionPreviewStateTests` passed, including ready gating, stale/failed
selection rejection, Active Project Plane isolation, transport-anchor
preservation, position and stop behavior.

## Real Kernel/device sequence

The rebuilt `VitApp.exe` opened the real JUCE device:

```text
Device type: Windows Audio
Output:      扬声器 (USB AUDIO  CODEC)
Input:       Line (USB AUDIO  CODEC)
Sample rate: 48000 Hz
Block size:  480 samples
Device open: true
```

A Kernel/VSP sequence ran against that process:

1. Active transport play;
2. `audition.prepare` with two 10-second WAV files (440 Hz and 880 Hz);
3. `audition.ready` returned only after both files were decoded/prepared;
4. `audition.select(candidate-a)` and 4-second output phase;
5. `audition.select(candidate-b)` and 4-second output phase;
6. `audition.stop` and active transport stop.

Observed state evidence included:

```text
prepare: status=ok, message=audition.ready
select A: status=playing, audio_candidate_id=candidate-a
select B: status=playing, audio_candidate_id=candidate-b
A position: approximately 0.526 s
B position: approximately 4.526 s
stop:      audio_is_playing=false, audio_source_active=false
```

The Active Project Plane remained `project:active / manual-r1` throughout.
The Kernel process reported the real output device as open and its audio CPU
telemetry remained live during both phases.

This is an executed device-output sequence. The coding agent cannot directly
verify human hearing; a person monitoring the named `USB AUDIO CODEC` output
must confirm the audible 440 Hz-to-880 Hz change. The automated block evidence
and the real-device output-path evidence are separate claims.

## Regression commands

Passed:

```text
go test ./internal/kernel
go test ./internal/chat
go test ./...

VitAuditionPreviewAudioPlaneTests
VitAuditionPreviewStateTests

npm test
npm run build

VitApp Debug build
 git diff --check
```

## Remaining boundary

This validation does not claim full-project audition. Checkpoint/branch/worktree
identity is carried and audited, but only `target + audio_file` is rendered by
this gap closure. Full-project render, graph construction from project
checkpoints, tempo/loop-aware chase, persistent render cache, and GUI
inspect/apply remain future work.
