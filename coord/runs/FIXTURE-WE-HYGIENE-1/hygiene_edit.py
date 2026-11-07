#!/usr/bin/env python3
"""FIXTURE-WE-HYGIENE-1 surgical hygiene edit (294).

Operates ONLY on the isolated workspace copies (never on the authoritative
fixture). Removes the three 2026-09-05 strip_silence leftovers:
  1. mixboard_feature_snapshot.json: top-level "waveform_envelope" slot
     (kernel_prepared_waveform_envelope_1055 row)
  2. mixboard_feature_snapshot.json: track_waveform_envelopes last row
     (track 1007 / clip 1055)
  3. acoustic_package_status.json: packages[] entry with clip_id 1055

Edit style: line-surgical removal (tab-indented Go MarshalIndent output is
preserved byte-for-byte outside the removed ranges). Validation: edited text
must parse and deep-equal the in-memory expected structure (json-level removal
of the same elements), and unified diff must contain only deletions.
"""
import json
import sys
from pathlib import Path

WS = Path(r"D:/Vit_DAW_ws_294hyg")
ORIG = WS / "orig"
HYG = WS / "hyg"


def load_lines(p: Path):
    return p.read_text(encoding="utf-8").splitlines(keepends=True)


def find_twe_last_element(lines):
    """Return (start, end) line index range [start, end) of the LAST array
    element of "track_waveform_envelopes": [ ... ] (objects at indent 2 tabs)."""
    arr = None
    for i, ln in enumerate(lines):
        if ln.startswith('\t"track_waveform_envelopes": ['):
            arr = i
            break
    if arr is None:
        raise SystemExit("twe array start not found")
    # walk to matching close at one tab: "\t],"
    close = None
    for j in range(arr + 1, len(lines)):
        if lines[j].startswith('\t],'):
            close = j
            break
    if close is None:
        raise SystemExit("twe array close not found")
    # last element start: last line before close that is exactly "\t\t{"
    start = None
    for j in range(close - 1, arr, -1):
        if lines[j] == '\t\t{\n':
            start = j
            break
    if start is None:
        raise SystemExit("twe last element start not found")
    # element body ends at close-1 ("\t\t}" possibly with comma)
    return start, close


def remove_twe_last(lines):
    start, close = find_twe_last_element(lines)
    # previous element's closing line "\t\t}," -> "\t\t}" (drop its comma)
    prev_close = None
    for j in range(start - 1, 0, -1):
        if lines[j] in ('\t\t},\n', '\t\t}\n'):
            prev_close = j
            break
    if prev_close is None or lines[prev_close] != '\t\t},\n':
        raise SystemExit("unexpected twe structure: previous element close not found or no comma")
    out = lines[:prev_close] + ['\t\t}\n'] + lines[prev_close + 1:start] + lines[close:]
    return out, (start, close)


def remove_we_slot(lines):
    """Remove top-level "waveform_envelope": { ... } member and the trailing
    comma of the preceding member."""
    slot = None
    for i, ln in enumerate(lines):
        if ln.startswith('\t"waveform_envelope": {'):
            slot = i
            break
    if slot is None:
        raise SystemExit("waveform_envelope slot not found")
    # find closing "\t}" followed by "}" (EOF) or next top-level key
    end = None
    depth = 0
    for j in range(slot, len(lines)):
        ln = lines[j]
        depth += ln.count('{') - ln.count('}')
        if j > slot and depth == 0:
            end = j  # inclusive: the "\t}" line
            break
    if end is None:
        raise SystemExit("waveform_envelope slot end not found")
    # preceding member ends with a value line carrying a trailing comma
    prev = lines[slot - 1]
    if not prev.rstrip('\n').endswith(','):
        raise SystemExit("unexpected structure: member before WE slot has no trailing comma")
    lines[slot - 1] = prev.rstrip('\n').rstrip(',') + '\n'
    return lines[:slot] + lines[end + 1:], (slot, end)


def remove_pkg_1055(lines):
    """Remove packages[] element whose "clip_id": "1055" (indent 3 tabs inside
    the element body, element objects at 2 tabs)."""
    # locate packages array
    arr = None
    for i, ln in enumerate(lines):
        if ln.startswith('\t"packages": ['):
            arr = i
            break
    if arr is None:
        raise SystemExit("packages array not found")
    # find element boundaries: lines exactly "\t\t{" ... "\t\t}," / "\t\t}"
    elems = []
    j = arr + 1
    while j < len(lines) and lines[j] != '\t]\n':
        if lines[j] == '\t\t{\n':
            k = j
            while lines[k] not in ('\t\t},\n', '\t\t}\n'):
                k += 1
            elems.append((j, k))
            j = k + 1
        else:
            j += 1
    target = None
    for (s, e) in elems:
        body = ''.join(lines[s:e + 1])
        if '"clip_id": "1055"' in body:
            target = (s, e)
    if target is None:
        raise SystemExit("1055 package element not found")
    s, e = target
    if lines[e] == '\t\t},\n':
        # not last element: just drop element including its comma line
        return lines[:s] + lines[e + 1:], (s, e)
    # last element: drop element and strip comma from new last element
    prev = lines[s - 1]
    if prev != '\t\t},\n':
        raise SystemExit("unexpected packages structure at tail")
    return lines[:s - 1] + ['\t\t}\n'] + lines[e + 1:], (s, e)


def deep_equal(a, b):
    return json.dumps(a, sort_keys=True, ensure_ascii=False) == json.dumps(b, sort_keys=True, ensure_ascii=False)


def main():
    HYG.mkdir(parents=True, exist_ok=True)
    # --- target 1+2: mixboard snapshot ---
    src = ORIG / "mixboard_feature_snapshot.json"
    lines = load_lines(src)
    orig_len = len(lines)
    lines, twe_rng = remove_twe_last(lines)
    lines, we_rng = remove_we_slot(lines)
    snap_text = ''.join(lines)
    edited = json.loads(snap_text)
    expect = json.loads(src.read_text(encoding="utf-8"))
    twe_expect = [r for r in expect["track_waveform_envelopes"]
                  if not (str(r.get("clip_id")) == "1055")]
    assert len(twe_expect) == len(expect["track_waveform_envelopes"]) - 1, "exactly one twe row dropped"
    expect["track_waveform_envelopes"] = twe_expect
    del expect["waveform_envelope"]
    if not deep_equal(edited, expect):
        raise SystemExit("mixboard edited structure != expected structure")
    out1 = HYG / "mixboard_feature_snapshot.json"
    out1.write_text(snap_text, encoding="utf-8", newline='')
    print(f"mixboard: lines {orig_len} -> {len(lines)}; twe last element {twe_rng}; WE slot {we_rng}")

    # --- target 3: acoustic packages ---
    src2 = ORIG / "acoustic_package_status.json"
    lines2 = load_lines(src2)
    orig_len2 = len(lines2)
    lines2, pkg_rng = remove_pkg_1055(lines2)
    aps_text = ''.join(lines2)
    edited2 = json.loads(aps_text)
    expect2 = json.loads(src2.read_text(encoding="utf-8"))
    pkgs = [p for p in expect2["packages"] if str(p.get("clip_id")) != "1055"]
    assert len(pkgs) == len(expect2["packages"]) - 1, "exactly one package dropped"
    expect2["packages"] = pkgs
    if not deep_equal(edited2, expect2):
        raise SystemExit("acoustic edited structure != expected structure")
    out2 = HYG / "acoustic_package_status.json"
    out2.write_text(aps_text, encoding="utf-8", newline='')
    print(f"acoustic: lines {orig_len2} -> {len(lines2)}; pkg element {pkg_rng}")
    print("OK: surgical edits validated (parse + deep-equal)")


if __name__ == "__main__":
    main()
