# Multiband Dynamics v1 Validation

## Automated Evidence

- `go test ./internal/workflows/plugingrabber -run Multiband`: topology,
  named-band clustering, C4/C6/LinMB crossover ordering, MBC/354E split
  boundaries, band-local Pro-MB rejection, identity/value-independent
  generation, and adjacent negative fixtures.
- `go test ./internal/chat -run Multiband`: inspect/apply bridge, generic write
  guard, successful typed apply, and invalid crossover atomic rejection.
- `go test ./internal/chat ./internal/toolpolicy ./internal/tools ./internal/workflows/plugingrabber`: current focused package baseline.
- `python -m py_compile scripts/multiband_pluginprobe_census.py scripts/multiband_matrix_smoke.py`:
  census runner syntax.
- `cmake --build PluginProbe/native-host/build --config Release --target
  pluginprobe_vst3_worker`: isolated WaveShell-member selector and stdout
  protocol isolation build.

## Required Product Cases

`C4`, `C6`, `LinMB`, `Lindell MBC`, and `Lindell 354E` are the required positive
topology cases. The latter two were previously disclosed and are regression
samples, not sealed blind samples. C6 has retained complete parameter evidence
and remains the primary structural exemplar.

Negative coverage is required for sidechain EQ, dynamic EQ, de-esser,
multiband maximizer, spectral dynamics, and clipper-adjacent surfaces. A
negative case must prove rejection from the live parameter structure and must
show zero writes for any attempted typed apply.

The current PluginProbe census has 31 captured non-sealed cases with no load
errors, plus four `sealed_not_opened` IDX/IDX LIVE cases. Its structural
summary reports 8 confirmed Waves positive instances, Pro-MB as
`unresolved_band_local_crossovers`, and the remaining Waves multiband inventory
as rejected or unresolved from parameter-role evidence.

## Census Gate

Run the isolated census before treating target rows as classified:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -SkipBuild -StartKernel -StartUI -NoChatSmoke
python scripts/multiband_pluginprobe_census.py
```

The runner's `temp/multiband-pluginprobe-census/summary.json` is the
authoritative local inventory result for this survey. Sealed IDX/IDX Live rows
must remain `sealed_not_opened` until the rule freeze is recorded. The
per-case snapshots record the observed identity only as inventory metadata;
the recognizer and control bindings remain identity-free.
