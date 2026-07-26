#!/usr/bin/env python3
"""Live write verification for set_eq_point.

Asks each plugin for the same musical move (3400 Hz, -3 dB) and checks that the
parameters the plugin actually ends up holding match the request. Captures the
normalised value of every parameter it touches beforehand and puts them back
afterwards, so plugin state is unchanged when it finishes.

The case this exists for: FreeEQ8 previously reported success for this request
while writing only a gain, leaving its band parked at 4000 Hz.
"""
from __future__ import annotations

import json
import sys
import urllib.request

BASE = "http://127.0.0.1:7878"
TARGET_HZ = 3400.0
TARGET_DB = -3.0
TARGET_Q = 1.5

# label, track, plugin, expect_freq_write, expected band, expect_q_applied
FIXTURES = [
    ("TDR Nova", "1014", "1020", True, "B3", True),
    ("FreeEQ8", "1021", "1027", True, "B6", True),
    ("ZamEQ2", "1042", "1048", True, "B2", True),
    ("Pro-Q 3", "1035", "1041", True, "B1", True),
    ("Marvel GEQ", "1028", "1034", False, "B12", False),
    ("ZamGEQ31", "1049", "1055", False, "B3165", False),
]


def invoke(tool: str, args: dict, confirmed: bool = False) -> dict:
    payload = json.dumps({"tool": tool, "args": args, "confirmed": confirmed,
                          "source": "eq_write_verify"}).encode()
    req = urllib.request.Request(BASE + "/agent/invoke", data=payload,
                                 headers={"Content-Type": "application/json"}, method="POST")
    return json.loads(urllib.request.urlopen(req, timeout=180).read())


def params(track: str, plugin: str) -> dict[str, dict]:
    res = invoke("plugin.get_parameters",
                 {"track_id": track, "plugin_id": plugin, "include_parameters": True})["result"]
    return {str(r.get("param_id")): r for r in res["parameters"]}


def number(text) -> float | None:
    try:
        return float(str(text).strip().split()[0].replace("+", ""))
    except (TypeError, ValueError, IndexError):
        return None


def run_case(label, track, plugin, expect_freq, expect_band, expect_q) -> list[str]:
    problems = []
    before = params(track, plugin)

    resp = invoke("plugin_grabber.set_eq_point",
                  {"track_id": track, "plugin_id": plugin, "freq_hz": TARGET_HZ,
                   "gain_db": TARGET_DB, "q": TARGET_Q}, confirmed=True)
    if str(resp.get("status")) != "ok":
        print(f"FAIL {label:12s} set_eq_point status={resp.get('status')} {resp.get('error', '')}")
        return [f"{label}: {resp.get('error', 'not ok')}"]

    result = resp.get("result", {})
    writes = result.get("writes", [])
    roles = {w.get("role") for w in writes}
    band = (result.get("selected_band") or {}).get("band")
    model = result.get("eq_model")

    print(f"     {label:12s} model={model:22s} band={band} roles={sorted(roles)}")

    if band != expect_band:
        problems.append(f"{label}: band={band} want {expect_band}")
    if expect_freq and "freq" not in roles:
        problems.append(f"{label}: no frequency write — the band was not retuned")
    if not expect_freq and "freq" in roles:
        problems.append(f"{label}: wrote a frequency on a fixed-frequency EQ")

    skipped = bool((result.get("selected_band") or {}).get("q_skipped"))
    if expect_q and ("q" not in roles or skipped):
        problems.append(f"{label}: Q was requested and this band has one, but it was not applied")
    if not expect_q and not skipped:
        problems.append(f"{label}: this band has no width control; the skip should be reported")
    if skipped:
        print(f"       q     skipped and reported (band has no width control)")

    after = params(track, plugin)
    for write in writes:
        pid = str(write.get("param_id"))
        row = after.get(pid, {})
        got = number(row.get("value_text"))
        role = write.get("role")
        want = {"freq": TARGET_HZ, "gain": TARGET_DB, "q": TARGET_Q}.get(role)
        if want is None:
            continue
        tol = max(abs(want) * 0.06, 1.0)
        verdict = "ok" if got is not None and abs(got - want) <= tol else "MISMATCH"
        print(f"       {role:5s} param {pid}: {row.get('value_text')!r} (want ~{want}) {verdict}")
        if verdict == "MISMATCH":
            problems.append(f"{label}: {role} readback {row.get('value_text')!r} want ~{want}")

    # Put every touched parameter back where it was.
    for write in writes:
        pid = str(write.get("param_id"))
        original = before.get(pid, {}).get("normalized_value")
        if original is None:
            continue
        invoke("plugin.set_parameter",
               {"track_id": track, "plugin_id": plugin, "param_id": pid,
                "value": original}, confirmed=True)
    return problems


def main() -> None:
    print(f"== requesting {TARGET_HZ} Hz / {TARGET_DB} dB on each plugin\n")
    problems = []
    for fixture in FIXTURES:
        problems += run_case(*fixture)
        print()
    if problems:
        print(f"{len(problems)} problem(s):")
        for p in problems:
            print(f"  - {p}")
        sys.exit(1)
    print("all writes landed on the requested frequency and gain")


if __name__ == "__main__":
    main()
