# Stereo Freq Calibrator

`stereo_freq_calibrator.py` is a stereo calibration helper for Vit-DAW frequency view.

It generates:

- a stereo WAV test signal (left-only / right-only per frequency, optional both-channel),
- a JSON manifest with exact segment timing and expected frequency,
- a CSV template for manual observations and error recording.

## Quick start

```bash
python scripts/stereo_freq_calibrator.py
```

Default output:

- `D:/Vit_DAW/calibration/stereo_freq_calibration.wav`
- `D:/Vit_DAW/calibration/stereo_freq_calibration.manifest.json`
- `D:/Vit_DAW/calibration/stereo_freq_calibration.observations.csv`

## Typical stereo workflow

1. Run generator.
2. Import WAV into target stereo track.
3. Switch to frequency view.
4. Play and observe left/right segment peaks.
5. Fill `observed_left_hz` / `observed_right_hz` in CSV.
6. Re-run with `--annotate-csv` to auto-calc Hz/cents error.

## Useful options

- custom frequency set:

```bash
python scripts/stereo_freq_calibrator.py --freqs "50,100,200,400,800,1600,3200,6400"
```

- include both-channel segments:

```bash
python scripts/stereo_freq_calibrator.py --include-both
```

- change timing and level:

```bash
python scripts/stereo_freq_calibrator.py --segment-seconds 1.5 --gap-seconds 0.25 --level-db -12
```

- generate and auto import/play via IPC:

```bash
python scripts/stereo_freq_calibrator.py --track-id 1007 --start-time 0 --auto-play-seconds 25
```

Requires `pyzmq` for IPC mode:

```bash
pip install pyzmq
```

## Notes

- `fft_size_reference` in manifest is only for expected bin quantization reference.
- The WAV is 48 kHz, 16-bit stereo PCM by default.
- Short fade in/out is applied to reduce clicks between segments.
