import argparse
import csv
import json
import math
import struct
import time
import wave
from pathlib import Path
from typing import Dict, List, Tuple


DEFAULT_FREQS = [
    40.0,
    60.0,
    80.0,
    100.0,
    150.0,
    200.0,
    300.0,
    500.0,
    800.0,
    1000.0,
    2000.0,
    5000.0,
    8000.0,
    12000.0,
]


def parse_freqs(text: str) -> List[float]:
    if not text.strip():
        return list(DEFAULT_FREQS)
    out: List[float] = []
    for part in text.split(","):
        value = float(part.strip())
        if value <= 0.0:
            raise ValueError(f"invalid frequency: {value}")
        out.append(value)
    return out


def db_to_gain(db: float) -> float:
    return math.pow(10.0, db / 20.0)


def cents_error(observed_hz: float, expected_hz: float) -> float:
    if observed_hz <= 0.0 or expected_hz <= 0.0:
        return 0.0
    return 1200.0 * math.log2(observed_hz / expected_hz)


def nearest_fft_bin(freq_hz: float, sample_rate: int, fft_size: int) -> Tuple[float, float]:
    df = float(sample_rate) / float(fft_size)
    k = int(round(freq_hz / df))
    bin_hz = k * df
    return bin_hz, bin_hz - freq_hz


def build_timeline(
    freqs: List[float],
    seg_seconds: float,
    gap_seconds: float,
    include_both: bool,
    sample_rate: int,
    level_db: float,
    fade_seconds: float,
    fft_size: int,
) -> Tuple[List[int], List[Dict]]:
    gain = db_to_gain(level_db)
    fade_samples = max(1, int(round(fade_seconds * sample_rate)))
    seg_samples = int(round(seg_seconds * sample_rate))
    gap_samples = int(round(gap_seconds * sample_rate))

    if seg_samples <= 2 * fade_samples:
        raise ValueError("segment is too short for fade in/out")

    pcm_interleaved: List[int] = []
    manifest: List[Dict] = []
    sample_cursor = 0
    segment_id = 0

    def write_segment(freq_hz: float, mode: str) -> None:
        nonlocal sample_cursor, segment_id
        start_sample = sample_cursor
        left_active = mode in ("L", "BOTH")
        right_active = mode in ("R", "BOTH")

        bin_hz, bin_err = nearest_fft_bin(freq_hz, sample_rate, fft_size)
        for i in range(seg_samples):
            env = 1.0
            if i < fade_samples:
                env = float(i) / float(fade_samples)
            elif i >= seg_samples - fade_samples:
                env = float(seg_samples - i - 1) / float(fade_samples)
            phase = 2.0 * math.pi * freq_hz * (i / float(sample_rate))
            tone = math.sin(phase) * gain * env
            l = tone if left_active else 0.0
            r = tone if right_active else 0.0
            pcm_interleaved.append(int(max(-1.0, min(1.0, l)) * 32767.0))
            pcm_interleaved.append(int(max(-1.0, min(1.0, r)) * 32767.0))

        sample_cursor += seg_samples
        end_sample = sample_cursor
        segment_id += 1
        manifest.append(
            {
                "segment_id": segment_id,
                "freq_hz": freq_hz,
                "channel_mode": mode,
                "start_seconds": start_sample / float(sample_rate),
                "end_seconds": end_sample / float(sample_rate),
                "expected_left_hz": freq_hz if left_active else 0.0,
                "expected_right_hz": freq_hz if right_active else 0.0,
                "nearest_fft_bin_hz": bin_hz,
                "bin_quantization_error_hz": bin_err,
            }
        )

        for _ in range(gap_samples):
            pcm_interleaved.append(0)
            pcm_interleaved.append(0)
        sample_cursor += gap_samples

    for f in freqs:
        write_segment(f, "L")
        write_segment(f, "R")
        if include_both:
            write_segment(f, "BOTH")

    return pcm_interleaved, manifest


def write_wav(path: Path, sample_rate: int, pcm_interleaved: List[int]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with wave.open(str(path), "wb") as wf:
        wf.setnchannels(2)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        packed = struct.pack("<" + "h" * len(pcm_interleaved), *pcm_interleaved)
        wf.writeframes(packed)


def write_manifest(path: Path, payload: Dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")


def write_csv_template(path: Path, manifest: List[Dict]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(
            [
                "segment_id",
                "channel_mode",
                "start_seconds",
                "end_seconds",
                "expected_left_hz",
                "expected_right_hz",
                "observed_left_hz",
                "observed_right_hz",
                "left_error_hz",
                "right_error_hz",
                "left_error_cents",
                "right_error_cents",
                "notes",
            ]
        )
        for seg in manifest:
            w.writerow(
                [
                    seg["segment_id"],
                    seg["channel_mode"],
                    f'{seg["start_seconds"]:.6f}',
                    f'{seg["end_seconds"]:.6f}',
                    f'{seg["expected_left_hz"]:.6f}',
                    f'{seg["expected_right_hz"]:.6f}',
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                ]
            )


def annotate_csv_with_errors(csv_path: Path) -> None:
    rows: List[List[str]] = []
    with csv_path.open("r", newline="", encoding="utf-8") as f:
        rows = list(csv.reader(f))
    if not rows:
        return

    header = rows[0]
    h = {name: idx for idx, name in enumerate(header)}
    required = [
        "expected_left_hz",
        "expected_right_hz",
        "observed_left_hz",
        "observed_right_hz",
        "left_error_hz",
        "right_error_hz",
        "left_error_cents",
        "right_error_cents",
    ]
    if any(col not in h for col in required):
        return

    for i in range(1, len(rows)):
        row = rows[i]
        try:
            exp_l = float(row[h["expected_left_hz"]] or 0.0)
            exp_r = float(row[h["expected_right_hz"]] or 0.0)
            obs_l = float(row[h["observed_left_hz"]] or 0.0)
            obs_r = float(row[h["observed_right_hz"]] or 0.0)
        except ValueError:
            continue

        if exp_l > 0.0 and obs_l > 0.0:
            row[h["left_error_hz"]] = f"{obs_l - exp_l:.6f}"
            row[h["left_error_cents"]] = f"{cents_error(obs_l, exp_l):.3f}"
        if exp_r > 0.0 and obs_r > 0.0:
            row[h["right_error_hz"]] = f"{obs_r - exp_r:.6f}"
            row[h["right_error_cents"]] = f"{cents_error(obs_r, exp_r):.3f}"

    with csv_path.open("w", newline="", encoding="utf-8") as f:
        csv.writer(f).writerows(rows)


def send_command(socket, payload: Dict) -> Dict:
    socket.send_string(json.dumps(payload, ensure_ascii=False))
    return socket.recv_json()


def optional_import_and_play(
    wav_path: Path,
    track_id: str,
    start_time: float,
    zmq_url: str,
    play_seconds: float,
) -> None:
    try:
        import zmq  # type: ignore
    except Exception as e:
        raise RuntimeError(
            "pyzmq is required for --track-id / --auto-play. install with: pip install pyzmq"
        ) from e

    context = zmq.Context()
    sock = context.socket(zmq.REQ)
    sock.connect(zmq_url)
    try:
        print("[ipc] set_click=false")
        print(send_command(sock, {"cmd": "set_click", "enabled": False}))
        payload = {
            "cmd": "add_audio_clip",
            "track_id": str(track_id),
            "file_path": wav_path.as_posix(),
            "start_time": float(start_time),
        }
        print(f"[ipc] add_audio_clip track_id={track_id}")
        print(send_command(sock, payload))
        print("[ipc] play")
        print(send_command(sock, {"cmd": "play"}))
        if play_seconds > 0:
            time.sleep(play_seconds)
            print("[ipc] stop")
            print(send_command(sock, {"cmd": "stop"}))
    finally:
        sock.close(0)
        context.term()


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Generate stereo frequency calibration audio + manifest + CSV template."
    )
    parser.add_argument(
        "--out-dir",
        default="D:/Vit_DAW/calibration",
        help="output directory for wav/manifest/csv",
    )
    parser.add_argument(
        "--base-name",
        default="stereo_freq_calibration",
        help="basename for generated files",
    )
    parser.add_argument(
        "--freqs",
        default="",
        help="comma-separated frequencies in Hz, empty uses defaults",
    )
    parser.add_argument("--sample-rate", type=int, default=48000)
    parser.add_argument("--segment-seconds", type=float, default=1.2)
    parser.add_argument("--gap-seconds", type=float, default=0.35)
    parser.add_argument("--fade-seconds", type=float, default=0.01)
    parser.add_argument("--level-db", type=float, default=-10.0)
    parser.add_argument("--fft-size", type=int, default=4096)
    parser.add_argument(
        "--include-both",
        action="store_true",
        help="also add BOTH(L+R) segment for each test frequency",
    )
    parser.add_argument(
        "--annotate-csv",
        action="store_true",
        help="re-open csv and compute error columns if observed_* fields were filled",
    )

    parser.add_argument(
        "--track-id",
        default="",
        help="if set, send add_audio_clip to this core track id",
    )
    parser.add_argument("--start-time", type=float, default=0.0)
    parser.add_argument("--zmq-url", default="tcp://127.0.0.1:5555")
    parser.add_argument(
        "--auto-play-seconds",
        type=float,
        default=0.0,
        help="if >0 and --track-id set, auto play and stop after N seconds",
    )

    args = parser.parse_args()

    freqs = parse_freqs(args.freqs)
    out_dir = Path(args.out_dir)
    wav_path = out_dir / f"{args.base_name}.wav"
    manifest_path = out_dir / f"{args.base_name}.manifest.json"
    csv_path = out_dir / f"{args.base_name}.observations.csv"

    pcm, segments = build_timeline(
        freqs=freqs,
        seg_seconds=args.segment_seconds,
        gap_seconds=args.gap_seconds,
        include_both=args.include_both,
        sample_rate=args.sample_rate,
        level_db=args.level_db,
        fade_seconds=args.fade_seconds,
        fft_size=args.fft_size,
    )
    write_wav(wav_path, args.sample_rate, pcm)

    payload = {
        "tool": "stereo_freq_calibrator",
        "sample_rate": args.sample_rate,
        "fft_size_reference": args.fft_size,
        "segment_seconds": args.segment_seconds,
        "gap_seconds": args.gap_seconds,
        "fade_seconds": args.fade_seconds,
        "level_db": args.level_db,
        "include_both": bool(args.include_both),
        "frequencies_hz": freqs,
        "segments": segments,
    }
    write_manifest(manifest_path, payload)

    if not csv_path.exists():
        write_csv_template(csv_path, segments)
    if args.annotate_csv:
        annotate_csv_with_errors(csv_path)

    print(f"[ok] wav: {wav_path}")
    print(f"[ok] manifest: {manifest_path}")
    print(f"[ok] observations csv: {csv_path}")
    print(f"[info] segments: {len(segments)}")

    if args.track_id.strip():
        optional_import_and_play(
            wav_path=wav_path,
            track_id=args.track_id.strip(),
            start_time=args.start_time,
            zmq_url=args.zmq_url,
            play_seconds=args.auto_play_seconds,
        )


if __name__ == "__main__":
    main()
