import argparse
import csv
import json
import math
from pathlib import Path
from typing import Dict, List, Tuple


BANDS = [
    ("low", 20.0, 200.0),
    ("mid", 200.0, 2000.0),
    ("high", 2000.0, 20000.0),
]


def parse_float(v: str) -> float:
    try:
        return float(v)
    except Exception:
        return 0.0


def band_name(freq_hz: float) -> str:
    for name, lo, hi in BANDS:
        if freq_hz >= lo and freq_hz < hi:
            return name
    return "out_of_range"


def stats(values: List[float]) -> Dict:
    if not values:
        return {"count": 0, "mean": 0.0, "abs_mean": 0.0, "rms": 0.0, "max_abs": 0.0}
    n = len(values)
    mean = sum(values) / n
    abs_mean = sum(abs(x) for x in values) / n
    rms = math.sqrt(sum(x * x for x in values) / n)
    max_abs = max(abs(x) for x in values)
    return {
        "count": n,
        "mean": mean,
        "abs_mean": abs_mean,
        "rms": rms,
        "max_abs": max_abs,
    }


def load_rows(path: Path) -> Tuple[List[str], List[List[str]]]:
    with path.open("r", newline="", encoding="utf-8") as f:
        rows = list(csv.reader(f))
    if not rows:
        raise ValueError("csv is empty")
    return rows[0], rows[1:]


def analyze(csv_path: Path) -> Dict:
    header, rows = load_rows(csv_path)
    h = {name: idx for idx, name in enumerate(header)}
    needed = [
        "channel_mode",
        "expected_left_hz",
        "expected_right_hz",
        "observed_left_hz",
        "observed_right_hz",
        "left_error_hz",
        "right_error_hz",
        "left_error_cents",
        "right_error_cents",
    ]
    for col in needed:
        if col not in h:
            raise ValueError(f"csv missing column: {col}")

    per_channel_hz: Dict[str, List[float]] = {"L": [], "R": []}
    per_channel_cents: Dict[str, List[float]] = {"L": [], "R": []}
    per_band_hz: Dict[str, Dict[str, List[float]]] = {
        "L": {"low": [], "mid": [], "high": [], "out_of_range": []},
        "R": {"low": [], "mid": [], "high": [], "out_of_range": []},
    }
    samples = []

    for row in rows:
        mode = row[h["channel_mode"]].strip().upper()
        exp_l = parse_float(row[h["expected_left_hz"]])
        exp_r = parse_float(row[h["expected_right_hz"]])
        obs_l = parse_float(row[h["observed_left_hz"]])
        obs_r = parse_float(row[h["observed_right_hz"]])
        err_l = parse_float(row[h["left_error_hz"]])
        err_r = parse_float(row[h["right_error_hz"]])
        cen_l = parse_float(row[h["left_error_cents"]])
        cen_r = parse_float(row[h["right_error_cents"]])

        if exp_l > 0 and obs_l > 0:
            b = band_name(exp_l)
            per_channel_hz["L"].append(err_l)
            per_channel_cents["L"].append(cen_l)
            per_band_hz["L"][b].append(err_l)
            samples.append(
                {"channel": "L", "mode": mode, "expected_hz": exp_l, "observed_hz": obs_l, "error_hz": err_l}
            )
        if exp_r > 0 and obs_r > 0:
            b = band_name(exp_r)
            per_channel_hz["R"].append(err_r)
            per_channel_cents["R"].append(cen_r)
            per_band_hz["R"][b].append(err_r)
            samples.append(
                {"channel": "R", "mode": mode, "expected_hz": exp_r, "observed_hz": obs_r, "error_hz": err_r}
            )

    summary = {
        "overall": {
            "left_hz": stats(per_channel_hz["L"]),
            "right_hz": stats(per_channel_hz["R"]),
            "left_cents": stats(per_channel_cents["L"]),
            "right_cents": stats(per_channel_cents["R"]),
        },
        "bands_hz": {
            "L": {k: stats(v) for k, v in per_band_hz["L"].items()},
            "R": {k: stats(v) for k, v in per_band_hz["R"].items()},
        },
        "sample_points": samples,
    }
    return summary


def write_markdown(path: Path, summary: Dict) -> None:
    def fmt(s: Dict) -> str:
        return (
            f'count={s["count"]}, mean={s["mean"]:.4f}, abs_mean={s["abs_mean"]:.4f}, '
            f'rms={s["rms"]:.4f}, max_abs={s["max_abs"]:.4f}'
        )

    lines: List[str] = []
    lines.append("# Stereo Frequency Offset Report")
    lines.append("")
    lines.append("## Overall")
    lines.append(f'- Left (Hz): {fmt(summary["overall"]["left_hz"])}')
    lines.append(f'- Right (Hz): {fmt(summary["overall"]["right_hz"])}')
    lines.append(f'- Left (cents): {fmt(summary["overall"]["left_cents"])}')
    lines.append(f'- Right (cents): {fmt(summary["overall"]["right_cents"])}')
    lines.append("")
    lines.append("## By Band (Hz error)")
    for ch in ("L", "R"):
        lines.append(f"### Channel {ch}")
        for band in ("low", "mid", "high"):
            lines.append(f'- {band}: {fmt(summary["bands_hz"][ch][band])}')
        lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Build stereo offset report from observations.csv")
    parser.add_argument("--csv", required=True, help="path to observations.csv")
    parser.add_argument(
        "--out-prefix",
        default="",
        help="output prefix; default uses csv path without .csv",
    )
    args = parser.parse_args()

    csv_path = Path(args.csv)
    prefix = Path(args.out_prefix) if args.out_prefix else csv_path.with_suffix("")
    json_path = prefix.with_suffix(".report.json")
    md_path = prefix.with_suffix(".report.md")

    summary = analyze(csv_path)
    json_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_markdown(md_path, summary)

    print(f"[ok] report json: {json_path}")
    print(f"[ok] report markdown: {md_path}")
    print("[summary] left_hz_abs_mean=%.4f right_hz_abs_mean=%.4f" % (
        summary["overall"]["left_hz"]["abs_mean"],
        summary["overall"]["right_hz"]["abs_mean"],
    ))


if __name__ == "__main__":
    main()
