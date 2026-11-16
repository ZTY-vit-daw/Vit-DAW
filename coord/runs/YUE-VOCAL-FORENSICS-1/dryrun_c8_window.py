"""381-C8 补腿 dry-run: verify the C8 duration-window assertion logic
WITHOUT any engine/agent/kernel (zero-stack, deterministic).

Loads scripts/yue_wire_smoke.py as a module (zmq stubbed — only used inside
the KernelZMQ class methods, never at import time) and exercises:
  - wav_duration_seconds() against the two REAL renders from
    run_20261116_000009 (main-repo artifacts, opened READ-ONLY);
  - wav_duration_in_request_window() acceptance/rejection matrix, including
    two synthetic counter-example WAVs written to a temp dir (severe short
    output 0.5x and over-long 1.667x) and fail-closed invalid inputs.

Expected matrix (each row: label, computed duration, window verdict):
  v1 real  59.999s -> True   (ratio 0.99998)
  v2 real  50.959s -> True   (ratio 0.849 — the observed engine short
                              output stays a recorded observation, not a
                              failure; the window catches SEVERE cases)
  synth    30s    -> False  (ratio 0.5  < 0.6 — short output DETECTED)
  synth   100s    -> False  (ratio 1.667 > 1.5 — over-long DETECTED)
  invalid  -1s    -> False  (fail closed)
  invalid  req=0  -> False  (fail closed)

Run: python -I dryrun_c8_window.py <v1.wav> <v2.wav>
Exit 0 = every row matched its expectation.
"""

import os
import sys
import tempfile
import types
import wave

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..", "scripts"))
if "zmq" not in sys.modules:
    sys.modules["zmq"] = types.ModuleType("zmq")  # import-time stub only

import yue_wire_smoke as drv  # noqa: E402


def synth_wav(seconds, sample_rate=8000):
    """Canonical 44-byte PCM16 mono header + silence, sized to `seconds`."""
    path = os.path.join(tempfile.mkdtemp(prefix="c8dryrun_"), "probe_%ds.wav" % seconds)
    with wave.open(path, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(sample_rate)
        w.writeframes(b"\x00\x00" * int(sample_rate * seconds))
    return path


def main():
    v1, v2 = sys.argv[1], sys.argv[2]
    rows = []
    for label, path, expect_window in (
        ("v1 real artifact", v1, True),
        ("v2 real artifact", v2, True),
        ("synth 30s (0.5x)", synth_wav(30), False),
        ("synth 100s (1.667x)", synth_wav(100), False),
    ):
        dur = drv.wav_duration_seconds(path)
        ok = drv.wav_duration_in_request_window(dur, drv.REQUESTED_DURATION_S)
        rows.append((label, dur, ok, ok == expect_window))
    for label, dur, req, expect in (
        ("invalid dur=-1", -1.0, drv.REQUESTED_DURATION_S, False),
        ("invalid requested=0", 50.0, 0, False),
    ):
        ok = drv.wav_duration_in_request_window(dur, req)
        rows.append((label, dur, ok, ok == expect))

    print("window = %s, requested = %ss" % (drv.C8_DURATION_RATIO_WINDOW, drv.REQUESTED_DURATION_S))
    all_ok = True
    for label, dur, ok, match in rows:
        print("  %-22s dur=%-8s in_window=%-5s %s" % (label, round(dur, 3) if isinstance(dur, float) else dur, ok, "OK" if match else "MISMATCH"))
        all_ok = all_ok and match
    print("DRYRUN_VERDICT %s" % ("PASS" if all_ok else "FAIL"))
    return 0 if all_ok else 1


if __name__ == "__main__":
    sys.exit(main())
