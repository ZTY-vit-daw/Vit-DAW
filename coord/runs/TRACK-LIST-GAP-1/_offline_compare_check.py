"""Offline validation of the same-frame comparison logic against a recorded dry-run.

Reads the raw per-surface JSON files captured by tracklist_gap_probe.py and re-runs
the summarizers/comparators without any kernel/hub/agent stack. This proves the
analysis path (user-track filtering, cross-surface comparison) before the official
stack run. No network access.
"""

import importlib.util
import json
import sys
from pathlib import Path

PROBE_PATH = Path(__file__).with_name("tracklist_gap_probe.py")
spec = importlib.util.spec_from_file_location("tracklist_gap_probe", PROBE_PATH)
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)

base = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).with_name("run_20261018_145825")
raw_dir = base / "raw"

STAGES = [
    "S0_fresh_baseline",
    "S1_external_new_import",
    "S1b_after_project_state_refresh",
    "S2_external_open_existing",
    "S3_agent_path_mutation",
]


def load(stage: str, suffix: str):
    path = raw_dir / f"{stage}_{suffix}.json"
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


expected = {
    "S0_fresh_baseline": {"kernel_user": 0, "hub_cmd_user": 0, "hub_state": 0, "shadow": 0, "tool": 0},
    "S1_external_new_import": {"kernel_user": 6, "hub_cmd_user": 6, "hub_state": 6, "shadow": 0, "tool": 0},
    "S1b_after_project_state_refresh": {"kernel_user": 6, "hub_cmd_user": 6, "hub_state": 6, "shadow": 6, "tool": 6},
    "S2_external_open_existing": {"kernel_user": 2, "hub_cmd_user": 2, "hub_state": 2, "shadow": 6, "tool": 6},
    "S3_agent_path_mutation": {"kernel_user": 3, "hub_cmd_user": 3, "hub_state": 3, "shadow": 3, "tool": 3},
}

ok = True
for stage_label in STAGES:
    k_list = load(stage_label, "kernel_list_tracks")
    k_state = load(stage_label, "kernel_get_project_state")
    hub_cmd = load(stage_label, "hub_project_tracks_list")
    hub_state = load(stage_label, "hub_state_snapshot")
    agent_state = load(stage_label, "agent_state")
    invoke_track = load(stage_label, "agent_invoke_track_list")
    missing = [name for name, value in (
        ("kernel_list", k_list), ("kernel_state", k_state), ("hub_cmd", hub_cmd),
        ("hub_state", hub_state), ("agent_state", agent_state), ("invoke_track", invoke_track)) if value is None]
    if missing:
        print(f"[stage] {stage_label}: MISSING raw files {missing} (skipped)")
        continue
    stage = {
        "label": stage_label,
        "kernel_list_tracks": probe.summarize_kernel_list(k_list),
        "kernel_get_project_state": probe.summarize_kernel_state(k_state),
        "hub_canonical_project_tracks_list": probe.summarize_hub_command(hub_cmd),
        "hub_state_snapshot": probe.summarize_hub_state(hub_state),
        "agent_shadow": probe.shadow_summary(agent_state),
        "agent_tool_track_list": probe.tool_summary(invoke_track),
    }
    comparison = probe.Probe.compare(stage_label, stage)
    print(f"[stage] {stage_label}")
    for key in ("kernel_user_track_count", "kernel_all_track_count",
                "hub_canonical_user_track_count", "hub_canonical_all_track_count",
                "hub_state_track_count", "shadow_track_count", "tool_track_count"):
        print(f"    {key}: {comparison[key]}")
    for key in ("tool_matches_shadow", "tool_matches_kernel", "hub_matches_kernel",
                "hub_state_matches_kernel", "kernel_hub_state_uuid_match",
                "shadow_uuid_matches_kernel"):
        print(f"    {key}: {comparison[key]}")
    print(f"    kernel_uuid: {comparison['kernel_project_uuid']!r} hub_state_uuid: {comparison['hub_state_project_uuid']!r}")
    exp = expected.get(stage_label)
    if exp:
        got = {
            "kernel_user": comparison["kernel_user_track_count"],
            "hub_cmd_user": comparison["hub_canonical_user_track_count"],
            "hub_state": comparison["hub_state_track_count"],
            "shadow": comparison["shadow_track_count"],
            "tool": comparison["tool_track_count"],
        }
        if got != exp:
            ok = False
            print(f"    COMPARE-MISMATCH expected={exp} got={got}")
        else:
            print(f"    compare: OK ({got})")

print("\noffline compare check: " + ("PASS" if ok else "FAIL"))
sys.exit(0 if ok else 1)
