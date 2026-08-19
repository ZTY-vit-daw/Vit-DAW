#!/usr/bin/env python3
"""Extract per-turn model request budget telemetry from a smoke run.

The Graph Context v1 governance fix makes the model-visible projection carry
context_size/context_degradation (hot/warm bytes, budgets, overflow). The
message_loop also logs `message_loop.llm ... chars=` per model turn. This
script extracts both signals from an agent log and/or response artifacts so a
smoke run can be checked against the v1 budgets:

- hot <= 10 KB
- warm <= 5 KB
- full model request (static + runtime + history) <= 24 KB

It is evaluator-side and never starts an Agent or mutates a project.
"""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path
from typing import Any

HOT_BUDGET = 10 * 1024
WARM_BUDGET = 5 * 1024
FULL_BUDGET = 24 * 1024

LLM_LOG_RE = re.compile(r"message_loop\.llm.*turn=(\d+).*messages=(\d+).*chars=(\d+).*message_chars=(\S+)")


def parse_log(path: Path) -> list[dict[str, Any]]:
    turns: list[dict[str, Any]] = []
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        if "message_loop.llm" not in line:
            continue
        match = LLM_LOG_RE.search(line)
        if not match:
            continue
        turns.append({
            "turn": int(match.group(1)),
            "message_count": int(match.group(2)),
            "chars": int(match.group(3)),
            "per_message_chars": match.group(4),
        })
    return turns


def parse_snapshot_artifacts(root: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for path in sorted(root.rglob("*.json")):
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except Exception:  # noqa: BLE001
            continue
        found: list[dict[str, Any]] = []
        def walk(value: Any) -> None:
            if isinstance(value, dict):
                if "context_size" in value and isinstance(value.get("context_size"), dict):
                    size = value["context_size"]
                    degradation = value.get("context_degradation") or {}
                    rows.append({
                        "source": str(path),
                        "total_bytes": size.get("total_bytes"),
                        "hot_bytes": size.get("hot_bytes"),
                        "warm_bytes": size.get("warm_bytes"),
                        "hot_budget": size.get("hot_budget"),
                        "warm_budget": size.get("warm_budget"),
                        "budget_status": degradation.get("budget_status"),
                        "overflow": value.get("context_overflow") is not None,
                    })
                for child in value.values():
                    walk(child)
            elif isinstance(value, list):
                for child in value:
                    walk(child)
        walk(data)
        if not found:
            continue
        rows.extend(found)
    return rows


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-log", type=Path, help="agent stdout/stderr log file")
    parser.add_argument("--artifact-root", type=Path, help="smoke artifact dir to scan for context_size snapshots")
    parser.add_argument("--fail-on-overflow", action="store_true", help="exit non-zero when a turn violates a budget")
    args = parser.parse_args()
    violations: list[str] = []
    if args.agent_log and args.agent_log.is_file():
        turns = parse_log(args.agent_log)
        print(f"parsed {len(turns)} model turns from {args.agent_log}")
        for turn in turns:
            if turn["chars"] > FULL_BUDGET:
                violation = f"turn {turn['turn']}: full request {turn['chars']} > {FULL_BUDGET}"
                violations.append(violation)
                print(f"  VIOLATION {violation}")
            else:
                print(f"  turn {turn['turn']}: full request {turn['chars']} bytes (<= {FULL_BUDGET})")
    if args.artifact_root and args.artifact_root.is_dir():
        snapshots = parse_snapshot_artifacts(args.artifact_root)
        print(f"found {len(snapshots)} context_size snapshots under {args.artifact_root}")
        for row in snapshots:
            hot = row.get("hot_bytes")
            warm = row.get("warm_bytes")
            problems = []
            if isinstance(hot, int) and hot > HOT_BUDGET:
                problems.append(f"hot {hot} > {HOT_BUDGET}")
            if isinstance(warm, int) and warm > WARM_BUDGET:
                problems.append(f"warm {warm} > {WARM_BUDGET}")
            if row.get("overflow"):
                problems.append("context_overflow present")
            if problems:
                violation = f"{row['source']}: " + "; ".join(problems)
                violations.append(violation)
                print(f"  VIOLATION {violation}")
            else:
                print(f"  ok {row['source']} hot={hot} warm={warm} status={row.get('budget_status')}")
    if violations:
        print(f"context governance check FAILED with {len(violations)} violation(s)")
        return 1 if args.fail_on_overflow else 0
    print("context governance check passed (no budget violations found)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
