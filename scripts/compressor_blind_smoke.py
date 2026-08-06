#!/usr/bin/env python3
"""Run one post-freeze identity-free compressor inspect without training edits."""
from __future__ import annotations

import argparse
import json
from pathlib import Path

import compressor_compat_matrix_smoke as matrix


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--plugin-name", required=True)
    parser.add_argument("--plugin-path", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7879")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    args = parser.parse_args()

    case = {
        "id": "post_freeze_blind",
        "plugin_name": args.plugin_name,
        "plugin_path": args.plugin_path,
    }
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    result = matrix.capture_case(
        args.agent_http, case, args.timeout_sec, output_dir)
    report = {
        "schema_version": "plugin_grabber.compressor_blind_report.v1",
        "status": "ok",
        "result": result,
    }
    (output_dir / "summary.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
