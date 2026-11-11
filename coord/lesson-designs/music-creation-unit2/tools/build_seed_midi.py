"""Build two small SMF fixtures from visually checked textbook references.

This is lesson material preparation, not a general OMR/MIDI converter.
Source PDF is never modified; display tempo/instrument are declared choices.
"""

from __future__ import annotations

import hashlib
import json
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PPQ = 480


def vlq(value: int) -> bytes:
    if value < 0:
        raise ValueError("Negative MIDI delta")
    values = [value & 0x7F]
    value >>= 7
    while value:
        values.insert(0, (value & 0x7F) | 0x80)
        value >>= 7
    return bytes(values)


def meta(kind: int, data: bytes) -> bytes:
    return bytes((0xFF, kind)) + vlq(len(data)) + data


def write_midi(path: Path, title: str, numerator: int, denominator: int,
               notes: list[dict], channel: int, bpm: int = 80) -> None:
    events = [
        (0, 0, meta(3, title.encode("utf-8"))),
        (0, 1, meta(0x51, int(60_000_000 / bpm).to_bytes(3, "big"))),
        (0, 2, meta(0x58, bytes((numerator, denominator.bit_length() - 1, 24, 8)))),
    ]
    if channel != 9:
        events.append((0, 3, meta(0x59, bytes((0, 0)))))  # demo C major
        events.append((0, 4, bytes((0xC0 | channel, 0))))  # GM piano hint
    for note in notes:
        start = round(note["start_quarter"] * PPQ)
        end = round((note["start_quarter"] + note["duration_quarter"]) * PPQ)
        events.append((start, 20, bytes((0x90 | channel, note["pitch"], 80))))
        events.append((end, 10, bytes((0x80 | channel, note["pitch"], 0))))
    events.sort(key=lambda row: (row[0], row[1]))
    track = bytearray()
    last = 0
    for tick, _, payload in events:
        track += vlq(tick - last) + payload
        last = tick
    track += b"\x00\xff\x2f\x00"
    header = b"MThd" + struct.pack(">IHHH", 6, 0, 1, PPQ)
    path.write_bytes(header + b"MTrk" + struct.pack(">I", len(track)) + track)


def main() -> None:
    data = json.loads((ROOT / "source/selected-symbolic-materials.json").read_text(encoding="utf-8"))
    source = {item["id"]: item for item in data["materials"]}
    output = ROOT / "assets/midi"
    output.mkdir(parents=True, exist_ok=True)
    degrees = {1: 0, 2: 2, 3: 4, 4: 5, 5: 7, 6: 9, 7: 11}
    prototype = source["exercise2-prototype"]
    notes = [{**n, "pitch": 60 + degrees[n["relative_degree"]]} for n in prototype["notes"]]
    proto_path = output / "exercise2-prototype_demo-C_80bpm.mid"
    write_midi(proto_path, "Textbook ex2 prototype (demo C, 80 BPM)", 4, 4, notes, 0)
    rhythm = source["sanshilipu-first-two-phrases-rhythm"]
    rhythm_notes = []
    start = 0
    for bar in rhythm["bars"]:
        for duration in bar["durations_quarter"]:
            rhythm_notes.append({"start_quarter": start, "duration_quarter": duration, "pitch": 37})
            start += duration
    rhythm_path = output / "sanshilipu-first8_rhythm-only_GM37_80bpm.mid"
    write_midi(rhythm_path, "Sanshilipu first 8 bars RHYTHM ONLY (GM37, 80 BPM)", 2, 4, rhythm_notes, 9)
    products = []
    for path, ref, note_rows in [(proto_path, prototype, notes), (rhythm_path, rhythm, rhythm_notes)]:
        products.append({
            "file": path.relative_to(ROOT).as_posix(),
            "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            "bytes": path.stat().st_size,
            "source_id": ref["id"],
            "pdf_page": ref["pdf_page"],
            "printed_page": ref["printed_page"],
            "time_signature": ref["time_signature"],
            "ppq": PPQ,
            "demo_bpm": 80,
            "expected_notes": note_rows,
            "sound_check": "not_performed",
        })
    result = {"schema_version": "vit.lesson_seed_midi.v0", "source_sha256": data["verification"]["source_sha256"], "products": products}
    (ROOT / "assets/seed-midi-manifest.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"generated": [row["file"] for row in products], "sound_check": "not_performed"}, ensure_ascii=False))


if __name__ == "__main__":
    main()
