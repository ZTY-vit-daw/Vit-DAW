#!/usr/bin/env python3
"""Exercise typed Limiter V1 apply/readback/restore on disclosed positive cases."""
from __future__ import annotations

import argparse
import json
from pathlib import Path

import compressor_compat_matrix_smoke as matrix
import limiter_blind_smoke as blind


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=Path(__file__).with_name("limiter_control_matrix.json"))
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240)
    args = parser.parse_args()
    manifest = blind.load_manifest(args.manifest.resolve())
    selected_ids = {"waves_l1_stereo", "bx_limiter_true_peak", "fabfilter_pro_l_2_regression_after_blind"}
    cases = [row for row in (manifest.get("training_cases") or []) + (manifest.get("regression_cases") or [])
             if row.get("id") in selected_ids]
    results = []
    for index, case in enumerate(cases, 1):
        identifier, resolution = matrix.resolve_identifier(case, args.timeout_sec)
        identity = {"id": case["id"], "plugin_name": case["plugin_name"], "plugin_identifier": identifier,
                    "resolved_path": matrix.first_text(resolution, "file_or_identifier", "plugin_path", "path")}
        result = blind.execute_case(args.agent_http, case, identity, args.timeout_sec,
                                    args.output_dir / "cases" / f"{index:02d}_{case['id']}")
        results.append(result)
        print(json.dumps(result, ensure_ascii=False), flush=True)
    summary = {"schema_version": "plugin_grabber.limiter_known_apply_report.v1", "results": results,
               "verdict": "passed" if len(results) == 3 and all(row.get("status") == "passed" for row in results) else "failed"}
    blind.write_json(args.output_dir / "summary.json", summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
