#!/usr/bin/env python3
"""Re-test the three disclosed Plugin Alliance round-2 failures after v1.2 fixes."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import compressor_plugin_alliance_regression_v11 as regression


SCRIPT_PATH = Path(__file__).resolve()
REPO_ROOT = SCRIPT_PATH.parent.parent
TRACK_PREFIX = "PA fix v1.2"

CASES: tuple[dict[str, Any], ...] = (
    {
        "id": "neold_u2a",
        "plugin_name": "NEOLD U2A",
        "plugin_path": r"C:\Program Files\Common Files\VST3\Plugin Alliance\NEOLD U2A.vst3",
        "plugin_identifier": "VST3-NEOLD U2A-686850fa-98e5425d",
        "expectation": "compressor",
        "expected_classification": "amount_driven",
    },
    {
        "id": "lindell_mu_66",
        "plugin_name": "Lindell MU-66",
        "plugin_path": r"C:\Program Files\Common Files\VST3\Plugin Alliance\Lindell MU-66.vst3",
        "plugin_identifier": "VST3-Lindell MU-66-80cd83f7-a687ff7f",
        "expectation": "not_compressor",
        "expected_rejection": "unsupported_multiband_compressor",
    },
    {
        "id": "bx_clipper_negative",
        "plugin_name": "bx_clipper",
        "plugin_path": r"C:\Program Files\Common Files\VST3\Plugin Alliance\bx_clipper.vst3",
        "plugin_identifier": "VST3-bx_clipper-dafdf142-13ed8cb9",
        "expectation": "not_compressor",
        "expected_rejection": "unsupported_clipper",
    },
)

POLICY = {
    "snapshot_tolerance": 0.0001,
    "max_apply_controls_per_positive": 2,
    "apply_role_priority": [
        "threshold", "input_drive", "reduction_amount", "low_level_amount",
        "high_level_amount", "ratio", "direction_curve", "attack", "release",
        "makeup_gain", "mix",
    ],
}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()

    output = Path(args.output_dir).resolve()
    if output.exists() and any(output.iterdir()):
        raise RuntimeError(f"output directory must be empty: {output}")
    output.mkdir(parents=True, exist_ok=True)

    for case in CASES:
        if not Path(str(case["plugin_path"])).is_file():
            raise RuntimeError(f"disclosed plug-in is missing: {case['plugin_path']}")

    regression.TRACK_PREFIX = TRACK_PREFIX
    regression.EXPECTED_BOUNDARIES.update({
        "lindell_mu_66": "unsupported_multiband_compressor",
        "bx_clipper_negative": "unsupported_clipper",
    })

    health = regression.blind.health_check(args.agent_http)
    results: list[dict[str, Any]] = []
    for index, case in enumerate(CASES, 1):
        case_id = str(case["id"])
        print(f"[{index}/{len(CASES)}] {case_id}", flush=True)
        resolution = {
            "id": case_id,
            "plugin_identifier": case["plugin_identifier"],
            "resolved_path": case["plugin_path"],
        }
        result = regression.run_case(
            args.agent_http, case, resolution, POLICY, args.timeout_sec,
            output / "cases" / f"{index:02d}_{case_id}",
        )
        expected_classification = str(case.get("expected_classification") or "")
        if result.get("status") == "passed" and expected_classification:
            if result.get("classification") != expected_classification:
                result["status"] = "failed"
                result["error"] = (
                    f"classification={result.get('classification')!r}, "
                    f"expected {expected_classification!r}"
                )
        results.append(result)
        print(f"  {result.get('status')} {result.get('error', '')}", flush=True)

    cleanup = regression.cleanup_audit(args.agent_http, args.timeout_sec)
    passed = [row for row in results if row.get("status") == "passed"]
    summary = {
        "schema_version": "plugin_grabber.compressor_plugin_alliance_fix.v1.2",
        "blind_claim": False,
        "scope": "three disclosed round-2 failures only",
        "sealed_reserve_parameters_read": 0,
        "agent_health": health,
        "agent_sha256": regression.blind.sha256_file(REPO_ROOT / "agent" / "bin" / "VitAgent.exe"),
        "completed_at": regression.blind.utc_now(),
        "verdict": "passed" if len(passed) == len(results) and cleanup["clean"] else "failed",
        "counts": {
            "total": len(results),
            "passed": len(passed),
            "failed": len(results) - len(passed),
        },
        "cleanup_audit": cleanup,
        "results": results,
    }
    regression.blind.write_json(output / "summary.json", summary)
    print(json.dumps({"verdict": summary["verdict"], "counts": summary["counts"]}, ensure_ascii=False))
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
