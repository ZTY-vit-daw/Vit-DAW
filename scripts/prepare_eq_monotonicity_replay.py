#!/usr/bin/env python3
"""Convert historical EQ compatibility captures into offline replay pages.

This is a research-only adapter.  It does not call a plug-in, mutate a project,
or change the production recognizer.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise ValueError(f"{path} is not a JSON object")
    return value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture-dir", required=True)
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()

    capture_dir = Path(args.capture_dir).resolve()
    output_dir = Path(args.output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    manifest: list[dict[str, Any]] = []
    for path in sorted(capture_dir.glob("*.json")):
        if path.name == "summary.json" or path.name.endswith(".plugin_search.json"):
            continue
        capture = read_json(path)
        envelope = capture.get("parameters")
        rows = envelope.get("parameters") if isinstance(envelope, dict) else None
        if not isinstance(rows, list):
            continue
        case = capture.get("case") if isinstance(capture.get("case"), dict) else {}
        case_id = str(case.get("id") or path.stem).strip()
        case_dir = output_dir / case_id
        case_dir.mkdir(parents=True, exist_ok=True)
        page = {
            "parameters": rows,
            "parameter_page": {"offset": 0, "limit": len(rows), "total": len(rows), "complete": True},
            "template_role": envelope.get("template_role"),
            "plugin_class": envelope.get("plugin_class"),
            "plugin_identity": envelope.get("plugin_identity"),
        }
        (case_dir / "page_0000.json").write_text(
            json.dumps(page, ensure_ascii=False, indent=2), encoding="utf-8")
        smoke = capture.get("smoke") if isinstance(capture.get("smoke"), dict) else {}
        result = smoke.get("result") if isinstance(smoke.get("result"), dict) else {}
        manifest.append({
            "case_id": case_id,
            "plugin_name": case.get("plugin_name"),
            "parameter_count": len(rows),
            "old_smoke_status": smoke.get("status"),
            "old_requested": result.get("requested"),
            "old_selected_band": result.get("selected_band"),
            "old_writes": result.get("writes"),
            "old_actual_readback": result.get("actual_readback"),
        })
    (output_dir / "manifest.json").write_text(
        json.dumps({"schema_version": "eq.monotonicity.replay_input.v1", "cases": manifest},
                   ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"prepared={len(manifest)} output={output_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
