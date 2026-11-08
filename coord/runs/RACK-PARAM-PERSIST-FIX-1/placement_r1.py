#!/usr/bin/env python3
"""RACK-PARAM-PERSIST-FIX-1 (card 321) placement leg.

Places a single Pro-Q 3 fixture copy (r1) in the persistent user-data domain
by reusing card 320's driver verbatim (read-only reuse; the three copied
modules are byte-identical to the 320 originals, sha256 pinned in RUNBOOK).
Zero LLM: kernel ZMQ face + agent HTTP face only.

Usage:
    python placement_r1.py --out placement_r1.json
"""
from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_REPO = _HERE.parents[2]

for _p in (str(_HERE), str(_REPO / "scripts"),
           str(_REPO / "coord" / "runs" / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "runner_frozen")):
    if _p not in sys.path:
        sys.path.insert(0, _p)

import proq3_free_cases as cases  # noqa: E402
import proq3_free_driver as drv  # noqa: E402


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--agent-http", default="http://127.0.0.1:7878")
    ap.add_argument("--out", required=True, type=Path)
    ap.add_argument("--timeout", type=float, default=30.0)
    args = ap.parse_args()

    projects_dir = Path(r"C:\Users\timoz\AppData\Roaming\Vit-DAW\projects")
    audio_source = Path(cases.AUDIO_SOURCE)

    if not audio_source.is_file():
        print(json.dumps({"status": "error",
                          "error": "audio source missing: " + str(audio_source)}))
        return 4

    spec = drv.resolve_proq3(args.timeout)
    placement = drv.place_project_copy(1, spec, args.agent_http, args.timeout,
                                       projects_dir, audio_source)

    report = {
        "schema_version": "rack_param_persist_placement.v1",
        "card": "RACK-PARAM-PERSIST-FIX-1",
        "placed_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "registry": {"identifier": spec["plugin_identifier"],
                     "plugin_path": spec["plugin_path"]},
        "placement": placement,
    }
    args.out.write_text(json.dumps(report, ensure_ascii=False, indent=2),
                        encoding="utf-8")
    print(json.dumps({"project": placement["project_path"],
                      "fixture": placement["fixture"],
                      "fixture_digest": placement["fixture_digest"],
                      "param_row_count": placement["param_row_count"]},
                     ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
