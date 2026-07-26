#!/usr/bin/env python3
"""Verify detected EQ model and slot roles against already-loaded plugins.

Read-only: calls explain_controls and prints the band summary each plugin now
produces, plus the expected model, so a regression is visible per plugin.
"""
from __future__ import annotations

import json
import sys
import urllib.request

BASE = "http://127.0.0.1:7878"

# track_id, plugin_id, expected eq_model, expectation note
FIXTURES = [
    ("TDR Nova", "1014", "1020", "fixed_slot_adjustable", "declares Hz label"),
    ("FreeEQ8", "1021", "1027", "fixed_slot_adjustable", "no unit label anywhere"),
    ("Marvel GEQ", "1028", "1034", "fixed_freq", "name special case supplies ISO table"),
    ("Pro-Q 3", "1035", "1041", "free_floating", "unit only in value text"),
    ("ZamEQ2", "1042", "1048", "fixed_slot_adjustable", "mixed numeric/letter band index"),
    ("ZamGEQ31", "1049", "1055", "fixed_freq", "frequency from band label"),
]


def invoke(tool: str, args: dict, confirmed: bool = False) -> dict:
    payload = json.dumps({"tool": tool, "args": args, "confirmed": confirmed,
                          "source": "eq_model_verify"}).encode()
    req = urllib.request.Request(BASE + "/agent/invoke", data=payload,
                                 headers={"Content-Type": "application/json"}, method="POST")
    return json.loads(urllib.request.urlopen(req, timeout=120).read())


def main() -> None:
    failures = []
    for label, track, plugin, expected, note in FIXTURES:
        try:
            resp = invoke("plugin_grabber.explain_controls",
                          {"track_id": track, "plugin_id": plugin})
            result = resp.get("result", {})
            summary = result.get("eq_band_summary")
            if not isinstance(summary, dict):
                raise RuntimeError(f"no eq_band_summary (status={resp.get('status')})")
            model = summary.get("eq_model")
            count = summary.get("band_count", summary.get("available_slot_count"))
            mark = "ok  " if model == expected else "FAIL"
            if model != expected:
                failures.append((label, f"model={model!r} want {expected!r}"))
            print(f"{mark} {label:12s} model={model:22s} bands={count}   ({note})")

            rows = summary.get("bands") or summary.get("available_slots") or []
            for row in rows[:2]:
                keys = {k: v for k, v in row.items()
                        if k in ("band", "freq_param_id", "gain_param_id", "q_param_id",
                                 "used_param_id", "current_freq_hz", "fixed_freq_hz")}
                print(f"       {keys}")
            if summary.get("frequency_labels"):
                print(f"       frequency_labels: unknown (declared)")
        except Exception as exc:
            print(f"FAIL {label:12s} {exc}")
            failures.append((label, str(exc)))

    print()
    if failures:
        print(f"{len(failures)} of {len(FIXTURES)} failed:")
        for label, err in failures:
            print(f"  - {label}: {err}")
        sys.exit(1)
    print(f"all {len(FIXTURES)} plugins classified as expected")


if __name__ == "__main__":
    main()
