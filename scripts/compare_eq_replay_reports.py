#!/usr/bin/env python3
"""Compare two eqtopologyreplay JSON reports without touching plug-ins."""
from __future__ import annotations

import argparse
import csv
import json
from pathlib import Path
from typing import Any


SHAPES = ("bell", "low_shelf", "high_shelf", "low_cut", "high_cut")
ACTIONS = ("upsert", "modify", "disable", "remove", "undo")


def read_report(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise ValueError(f"{path} is not a JSON object")
    return value


def case_map(report: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {
        str(row.get("case_id", "")): row
        for row in report.get("cases", [])
        if isinstance(row, dict) and row.get("case_id")
    }


def capabilities(case: dict[str, Any]) -> dict[tuple[str, str], bool]:
    out: dict[tuple[str, str], bool] = {}
    for row in case.get("shape_capabilities", []):
        if not isinstance(row, dict):
            continue
        shape = str(row.get("shape", ""))
        actions = row.get("actions")
        if not isinstance(actions, dict):
            continue
        for action in ACTIONS:
            out[(shape, action)] = bool(actions.get(action, False))
    return out


def supported_shapes(case: dict[str, Any]) -> str:
    caps = capabilities(case)
    return "|".join(shape for shape in SHAPES if caps.get((shape, "upsert"), False))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", required=True)
    parser.add_argument("--after", required=True)
    parser.add_argument("--csv", required=True)
    parser.add_argument("--markdown", required=True)
    args = parser.parse_args()

    before_path = Path(args.before).resolve()
    after_path = Path(args.after).resolve()
    before_report = read_report(before_path)
    after_report = read_report(after_path)
    before = case_map(before_report)
    after = case_map(after_report)
    ids = sorted(set(before) | set(after))

    rows: list[dict[str, Any]] = []
    regressions: list[str] = []
    expansions: list[str] = []
    for case_id in ids:
        old = before.get(case_id, {})
        new = after.get(case_id, {})
        old_caps = capabilities(old)
        new_caps = capabilities(new)
        lost: list[str] = []
        gained: list[str] = []
        for shape in SHAPES:
            for action in ACTIONS:
                key = (shape, action)
                if old_caps.get(key, False) and not new_caps.get(key, False):
                    lost.append(f"{shape}:{action}")
                if not old_caps.get(key, False) and new_caps.get(key, False):
                    gained.append(f"{shape}:{action}")
        if old.get("recognized") and not new.get("recognized"):
            lost.insert(0, "recognized")
        if old.get("executable") and not new.get("executable"):
            lost.insert(0, "executable")
        if lost:
            regressions.append(f"{case_id}: {', '.join(lost)}")
        if gained:
            expansions.append(f"{case_id}: {', '.join(gained)}")
        rows.append({
            "case_id": case_id,
            "before_recognized": bool(old.get("recognized", False)),
            "after_recognized": bool(new.get("recognized", False)),
            "before_executable": bool(old.get("executable", False)),
            "after_executable": bool(new.get("executable", False)),
            "before_upsert_shapes": supported_shapes(old),
            "after_upsert_shapes": supported_shapes(new),
            "lost_capabilities": "|".join(lost),
            "gained_capabilities": "|".join(gained),
        })

    csv_path = Path(args.csv).resolve()
    csv_path.parent.mkdir(parents=True, exist_ok=True)
    with csv_path.open("w", newline="", encoding="utf-8-sig") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(rows[0]) if rows else ["case_id"])
        writer.writeheader()
        writer.writerows(rows)

    md_path = Path(args.markdown).resolve()
    md_path.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        "# EQ replay before/after comparison",
        "",
        f"- Before: `{before_path}`",
        f"- After: `{after_path}`",
        f"- Cases: {len(ids)}",
        f"- Before recognized/executable: {before_report.get('recognized_count', 0)}/{before_report.get('executable_count', 0)}",
        f"- After recognized/executable: {after_report.get('recognized_count', 0)}/{after_report.get('executable_count', 0)}",
        f"- Regressed cases: {len(regressions)}",
        f"- Expanded cases: {len(expansions)}",
        "",
        "| Case | Before upsert shapes | After upsert shapes | Lost | Gained |",
        "|---|---|---|---|---|",
    ]
    for row in rows:
        lines.append(
            f"| {row['case_id']} | {row['before_upsert_shapes'] or '—'} | "
            f"{row['after_upsert_shapes'] or '—'} | {row['lost_capabilities'] or '—'} | "
            f"{row['gained_capabilities'] or '—'} |"
        )
    md_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"cases={len(ids)} regressions={len(regressions)} expansions={len(expansions)}")
    print(f"csv={csv_path}")
    print(f"markdown={md_path}")
    if regressions:
        for row in regressions:
            print(f"REGRESSION {row}")
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
