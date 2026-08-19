#!/usr/bin/env python3
"""Validate Graph Context v1 model-projection contract fixtures.

This is a Python contract/preflight check for the context governance fix. It
validates the JSON fixtures emitted by the Go unit test
TestDumpContextGraphContractFixtures (regenerate with
DUMP_CONTEXT_FIXTURES=1) against the invariants of
docs/OBSERVATION_PROJECTION_MANIFEST.md and
docs/CONTEXT_GRAPH_V1_DESIGN_DRAFT.md:

- single canonical CCB projection, no forbidden identity/topology fields;
- hot <= 10KB, warm <= 5KB, full request (snapshot + fixed static prompt)
  <= 24KB;
- observation_ledger is delta-only (current round receipts) with folded
  history (count, earliest/latest round, cold ref);
- over-budget degradation writes CompactionMarker and replaces sections with
  resolvable cold refs (facts never silently truncated);
- fail-closed context_overflow is explicit when degradation cannot bring the
  request within budget.

The check is evaluator-side: it parses fixtures and the manifest only, and
never calls the Agent or mutates a DAW project.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

HOT_BUDGET_BYTES = 10 * 1024
WARM_BUDGET_BYTES = 5 * 1024
FULL_REQUEST_BUDGET_BYTES = 24 * 1024
STATIC_PROMPT_BYTES = 6 * 1024  # fixed prompt contribution used by the draft profile

CCB_MODEL_PROJECTION_SCHEMA = "ccb_model_projection.v1"
LEDGER_SCHEMA = "free_state_observation_ledger.v1"

FORBIDDEN_KEY_MARKERS = (
    "plugin", "vendor", "manufacturer", "product", "parameter_id", "param_id",
    "sealed_truth", "target_family", "recommended_family", "processor_type",
    "measurement_key", "source_revision", "clip_revision", "compact_facts",
    "llm_context",
)

PROJECTION_SCHEMAS = (
    "dom.projection.v1",
    "mom.frequency_relationship.v1",
    "mom.static_level_relationship.v1",
    "tim.projection.v0",
    "tom.projection.v0_2",
    "fxm.projection.v0",
    "com.projection.v1",
    "com.source_dynamics.v1",
    "epm.projection.v0",
    "reference_level_model.projection.v0",
)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def walk(value: Any):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def count_schema(value: Any, schema: str) -> int:
    return sum(1 for row in walk(value) if row.get("schema_version") == schema)


def find_forbidden(value: Any) -> list[str]:
    found: list[str] = []
    for row in walk(value):
        for key in row:
            lowered = key.lower()
            if any(marker in lowered for marker in FORBIDDEN_KEY_MARKERS):
                found.append(key)
    return found


def text(value: Any) -> str:
    return "" if value is None else str(value)


def check_round12_fixture(fixture: dict[str, Any], path: Path) -> None:
    require(
        count_schema(fixture, CCB_MODEL_PROJECTION_SCHEMA) == 1,
        f"{path.name}: canonical CCB projection count != 1",
    )
    forbidden = find_forbidden(fixture)
    require(
        not forbidden,
        f"{path.name}: forbidden keys leaked into the model view: {sorted(set(forbidden))}",
    )
    size = fixture.get("context_size") or {}
    hot = int(text(size.get("hot_bytes") or 0))
    warm = int(text(size.get("warm_bytes") or 0))
    require(hot <= HOT_BUDGET_BYTES, f"{path.name}: hot {hot} > {HOT_BUDGET_BYTES}")
    require(warm <= WARM_BUDGET_BYTES, f"{path.name}: warm {warm} > {WARM_BUDGET_BYTES}")
    total = int(text(size.get("total_bytes") or 0))
    require(
        total + STATIC_PROMPT_BYTES <= FULL_REQUEST_BUDGET_BYTES,
        f"{path.name}: full request {total + STATIC_PROMPT_BYTES} > {FULL_REQUEST_BUDGET_BYTES}",
    )
    degradation = fixture.get("context_degradation") or {}
    require(
        text(degradation.get("budget_status")) == "within_budget",
        f"{path.name}: expected within_budget, got {degradation.get('budget_status')}",
    )
    require("context_overflow" not in fixture, f"{path.name}: unexpected context_overflow")
    # active observation views are short digests, never full LLMContext.
    active = fixture.get("active_observation") or {}
    for view_id, view in (active.get("views") or {}).items():
        view_row = view if isinstance(view, dict) else {}
        require(
            "llm_context" not in view_row,
            f"{path.name}: hot view {view_id} inlined llm_context",
        )
        require(
            "compact_facts" not in view_row,
            f"{path.name}: hot view {view_id} inlined compact_facts",
        )
        projection_status = text(view_row.get("projection_status"))
        if projection_status in ("ready", "partial"):
            require(
                text(view_row.get("digest")) != "" or "view_projection" in view_row,
                f"{path.name}: hot view {view_id} has no digest",
            )
    # delta-only ledger: only the current round receipts are visible.
    ledger = fixture.get("observation_ledger") or {}
    require(
        text(ledger.get("schema_version")) == LEDGER_SCHEMA,
        f"{path.name}: ledger schema missing",
    )
    receipts = ledger.get("receipts") or []
    require(len(receipts) == 1, f"{path.name}: ledger receipts = {len(receipts)}, want 1")
    require(
        text(receipts[0].get("receipt_id")) == "receipt-12",
        f"{path.name}: current-round receipt missing",
    )
    history = (ledger.get("history_window") or {}).get("receipts") or {}
    require(text(history.get("total")) == "12", f"{path.name}: history total != 12")
    require(text(history.get("earliest_round")) == "1", f"{path.name}: earliest_round != 1")
    require(text(history.get("latest_round")) == "12", f"{path.name}: latest_round != 12")
    require(text(history.get("cold_ref")) != "", f"{path.name}: history cold_ref missing")
    # goal trace events carry references, never duplicated tool results.
    trace = fixture.get("goal_trace_summary") or {}
    for event in trace.get("recent_events") or []:
        require("tool_result" not in event, f"{path.name}: recent event inlined tool_result")


def check_degraded_fixture(fixture: dict[str, Any], path: Path) -> None:
    degradation = fixture.get("context_degradation") or {}
    require(
        text(degradation.get("budget_status")) == "degraded_within_budget",
        f"{path.name}: expected degraded_within_budget, got {degradation.get('budget_status')}",
    )
    markers = fixture.get("compaction_markers") or []
    require(len(markers) > 0, f"{path.name}: degradation wrote no CompactionMarker")
    for marker in markers:
        require(text(marker.get("action")) == "degraded_to_ref", f"{path.name}: marker action broken")
        require(text(marker.get("cold_ref")) != "", f"{path.name}: marker cold_ref missing")
    state = fixture.get("daw_state_summary") or {}
    require(text(state.get("ref")) != "", f"{path.name}: degraded section lost its cold ref")
    require(
        "context_overflow" not in fixture,
        f"{path.name}: degraded fixture must not fail closed",
    )
    size = fixture.get("context_size") or {}
    require(
        int(text(size.get("hot_bytes") or 0)) <= HOT_BUDGET_BYTES,
        f"{path.name}: degraded hot layer still over budget",
    )


def check_overflow_fixture(fixture: dict[str, Any], path: Path) -> None:
    degradation = fixture.get("context_degradation") or {}
    require(
        text(degradation.get("budget_status")) == "context_overflow",
        f"{path.name}: expected context_overflow, got {degradation.get('budget_status')}",
    )
    overflow = fixture.get("context_overflow") or {}
    require(text(overflow.get("status")) == "overflow", f"{path.name}: overflow status broken")
    require(text(overflow.get("cold_ref")) != "", f"{path.name}: overflow cold_ref missing")
    require("overflowing" in overflow, f"{path.name}: overflow per-section report missing")
    # fail closed means the blocking section is still fully present (facts are
    # never truncated) while the request is refused.
    selection = fixture.get("current_selection") or {}
    require(
        "payload" in selection,
        f"{path.name}: blocking facts were truncated by overflow",
    )


def check_manifest(repo_root: Path) -> None:
    manifest = repo_root / "docs" / "OBSERVATION_PROJECTION_MANIFEST.md"
    require(manifest.is_file(), f"manifest missing: {manifest}")
    content = manifest.read_text(encoding="utf-8")
    for schema in PROJECTION_SCHEMAS:
        require(schema in content, f"manifest does not register {schema}")
    # The manifest states the peer topology and only ever mentions the wrong
    # chain topology inside an explicit negation.
    require("peer" in content and "peer projections" in content, "manifest lost peer-topology language")
    require("DAD -> DOM -> MOM" in content and "不存在" in content, "manifest does not negate the chain topology")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--fixtures",
        type=Path,
        default=Path(__file__).resolve().parents[1] / "temp" / "context_graph_v1",
        help="directory holding the Go-emitted contract fixtures",
    )
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=Path(__file__).resolve().parents[1],
        help="repository root (default: parent of scripts/)",
    )
    args = parser.parse_args()

    def load(name: str) -> dict[str, Any]:
        path = args.fixtures / name
        require(path.is_file(), f"fixture missing: {path}")
        return json.loads(path.read_text(encoding="utf-8"))

    check_manifest(args.repo_root.resolve())
    check_round12_fixture(load("model_projection_round12.json"), args.fixtures / "model_projection_round12.json")
    check_degraded_fixture(load("model_projection_degraded.json"), args.fixtures / "model_projection_degraded.json")
    check_overflow_fixture(load("model_projection_overflow.json"), args.fixtures / "model_projection_overflow.json")
    print(f"context_graph_v1 contract check passed (fixtures={args.fixtures})")


if __name__ == "__main__":
    main()
