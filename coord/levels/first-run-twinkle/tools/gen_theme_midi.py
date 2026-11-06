#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""first-run-twinkle 主题/改错版 MIDI 确定性生成器（279 EDU-LEVEL-CONTENT-1 定稿轮）。

设计决策（存档=assets/MANIFEST.yaml produced_by；开放问题⑫定稿随本脚本落）：
- Python 标准库单文件确定性生成：同 level.yaml 输入 -> 逐字节相同的 MIDI（可复现，零外部依赖/零 DAW）。
- MIDI format 0 / PPQ=480 / channel 0 / velocity=100 全曲固定（开放问题⑫：固定力度，制作简单可复现；
  与 rhythm_match 无关，试听质量取舍记录在案）。
- tempo meta=100 BPM、拍号 meta=4/4 仅为播放器显示信息——工程 tempo 由 n2 显式 set_tempo 写入
  （import 不读 tempo meta，rulings 2026-11-05 §六拍谱接线定案）。
- 音符数据唯一权威=level.yaml notes.theme（42 音全表）；本脚本是其确定性投影，不另持数据。
- 改错版=level.yaml assets[id=theme_corrupted].deviations 程序化应用（at_beat 定位；替换前断言原型音高）。

用法（在 first-run-twinkle/ 目录下）：
    python tools/gen_theme_midi.py --level level.yaml --out-dir assets
"""
import argparse
import copy
import hashlib
import sys
from pathlib import Path

import yaml

PPQ = 480          # ticks per quarter note；本曲 1 beat = 1 quarter
CHANNEL = 0
VELOCITY = 100     # 开放问题⑫定稿：全曲固定 velocity=100
TEMPO_BPM = 100    # 仅写入 meta 供播放器显示；工程 tempo 走 n2 set_tempo
FLOAT_EPS = 1e-9


def beats_to_ticks(beats: float) -> int:
    t = beats * PPQ
    if abs(t - round(t)) > FLOAT_EPS:
        raise SystemExit(f"gen: 非整数 tick（start/length 必须是 PPQ={PPQ} 的整数倍）：{beats}")
    return int(round(t))


def vlq(n: int) -> bytes:
    if n < 0:
        raise ValueError("vlq: negative")
    buf = [n & 0x7F]
    n >>= 7
    while n:
        buf.append((n & 0x7F) | 0x80)
        n >>= 7
    return bytes(reversed(buf))


def validate_table(notes, bar_count: int, beats_per_bar: int) -> None:
    if not notes:
        raise SystemExit("gen: 音符表为空")
    span = bar_count * beats_per_bar
    last_start = -1.0
    for i, n in enumerate(notes):
        if not (0 <= int(n["pitch"]) <= 127):
            raise SystemExit(f"gen: notes[{i}] pitch 越界: {n}")
        if n["start"] < 0 or n["length"] <= 0:
            raise SystemExit(f"gen: notes[{i}] start/length 非法: {n}")
        if n["start"] <= last_start - FLOAT_EPS:
            raise SystemExit(f"gen: notes[{i}] start 非严格递增: {n}")
        if n["start"] + n["length"] > span + FLOAT_EPS:
            raise SystemExit(f"gen: notes[{i}] 超出 bar_span 覆盖 [{span}] beats: {n}")
        beats_to_ticks(n["start"])
        beats_to_ticks(n["start"] + n["length"])
        last_start = n["start"]


def apply_deviations(notes, deviations, source_label: str):
    out = copy.deepcopy(notes)
    for dev in deviations:
        hit = [n for n in out if abs(n["start"] - dev["at_beat"]) < FLOAT_EPS]
        if len(hit) != 1:
            raise SystemExit(f"gen: deviation at_beat={dev['at_beat']} 未唯一命中音符（{len(hit)} hits）")
        if hit[0]["pitch"] != dev["replaces"]:
            raise SystemExit(
                f"gen: deviation at_beat={dev['at_beat']} 原型音高不符："
                f"期望 replaces={dev['replaces']}，实为 {hit[0]['pitch']}（{source_label}）"
            )
        hit[0]["pitch"] = dev["pitch"]
    return out


def build_midi(notes) -> bytes:
    events = [
        # (tick, prio, data)：tick 0 先 meta 后 note_on；同 tick note_off 先于 note_on（防悬挂音）
        (0, -2, b"\xff\x51\x03" + (60000000 // TEMPO_BPM).to_bytes(3, "big")),  # tempo meta
        (0, -1, b"\xff\x58\x04\x04\x02\x18\x08"),                                # 4/4 meta
    ]
    for n in notes:
        on = beats_to_ticks(n["start"])
        off = beats_to_ticks(n["start"] + n["length"])
        events.append((on, 1, bytes([0x90 | CHANNEL, int(n["pitch"]), VELOCITY])))   # note_on
        events.append((off, 0, bytes([0x80 | CHANNEL, int(n["pitch"]), 0])))         # note_off
    events.sort(key=lambda e: (e[0], e[1]))
    track = bytearray()
    last = 0
    for tick, _prio, data in events:
        track += vlq(tick - last)
        track += data
        last = tick
    track += vlq(0) + b"\xff\x2f\x00"  # end of track
    header = (
        b"MThd" + (6).to_bytes(4, "big")
        + (0).to_bytes(2, "big")            # format 0
        + (1).to_bytes(2, "big")            # ntrks 1
        + PPQ.to_bytes(2, "big")
    )
    return header + b"MTrk" + len(track).to_bytes(4, "big") + bytes(track)


def main() -> int:
    ap = argparse.ArgumentParser(description="first-run-twinkle 主题/改错版 MIDI 确定性生成器")
    ap.add_argument("--level", default="level.yaml", help="乐局卡 YAML 路径")
    ap.add_argument("--out-dir", default="assets", help="输出目录")
    args = ap.parse_args()

    level = yaml.safe_load(Path(args.level).read_text(encoding="utf-8"))
    theme = level["notes"]["theme"]
    bar_count = level["timing"]["bar_span"]["last_bar"] - level["timing"]["bar_span"]["first_bar"] + 1
    bpb = level["timing"]["beats_per_bar"]
    validate_table(theme, bar_count, bpb)

    corrupted_asset = next((a for a in level["assets"] if a["id"] == "theme_corrupted"), None)
    if corrupted_asset is None:
        raise SystemExit("gen: level.yaml 缺 assets[id=theme_corrupted]")
    corrupted = apply_deviations(theme, corrupted_asset.get("deviations", []), "assets.deviations")
    validate_table(corrupted, bar_count, bpb)

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    theme_asset = next(a for a in level["assets"] if a["id"] == "theme")
    outputs = [
        (Path(theme_asset["file"]).name, build_midi(theme)),
        (Path(corrupted_asset["file"]).name, build_midi(corrupted)),
    ]
    for name, blob in outputs:
        p = out_dir / name
        p.write_bytes(blob)
        print(f"{p}  bytes={len(blob)}  sha256={hashlib.sha256(blob).hexdigest()}")
    print(f"note_count: theme={len(theme)} corrupted={len(corrupted)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
