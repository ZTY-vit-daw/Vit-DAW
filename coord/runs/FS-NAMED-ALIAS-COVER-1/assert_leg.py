#!/usr/bin/env python3
"""FS-NAMED-ALIAS-COVER-1 (card 169) chat-leg assertion checker.

Reads the driver artifacts (softguide_exp.py --arm B1 --variants A2,A4) and
asserts the card acceptance face: the 159 A2/A4 utterances now resolve a
non-empty user_named_target through the real stack, with the resolved id
cross-checked against the track inventory by name. Also reads the optional
③ disclosure probe artifact when present. Exit 0 = all assertions pass.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

EXPECT_NAME = {"A2": "piano", "A4": "guitar"}


def load_json(path: Path):
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def track_names(project_state) -> dict:
    rows = ((project_state.get("result") or {}).get("tracks")) or []
    return {str(t.get("track_id") or t.get("id")): str(t.get("name") or t.get("track_name") or "") for t in rows}


def main() -> int:
    run_dir = Path(sys.argv[1]).resolve()
    arm_dir = run_dir / "driver" / "arm"
    failures = []
    resolved = {}
    for variant, want_name in EXPECT_NAME.items():
        chat = load_json(arm_dir / variant / "chat_1.json")
        loop = ((chat.get("workflow_data") or {}).get("free_state_reasoning_loop") or {})
        unt = loop.get("user_named_target") or ""
        if not unt:
            failures.append(f"{variant}: user_named_target empty (159 regression form)")
            continue
        names = track_names(load_json(arm_dir / variant / "project_state_after.json"))
        got_name = names.get(str(unt), "?")
        resolved[variant] = {"user_named_target": unt, "track_name": got_name}
        if want_name not in got_name.lower():
            failures.append(f"{variant}: resolved {unt}={got_name!r}, expected a {want_name!r} track")
        print(f"[{variant}] user_named_target={unt} name={got_name!r} (expect {want_name!r})")

    probe_path = run_dir / "driver" / "disclosure_probe.json"
    if probe_path.exists():
        probe = load_json(probe_path)
        line = "指名轨『弦乐』未在轨集中识别，按自动选择处理。"
        reply = str(probe.get("reply") or "")
        wf = (probe.get("workflow_data") or {})
        marker = wf.get("user_named_target_unresolved")
        print(f"[probe] reply-carries-line={line in reply} workflow-marker={'yes' if marker else 'no'}")
        if line not in reply:
            failures.append("probe: disclosure line missing from reply")
    else:
        print("[probe] artifact absent (optional face, non-blocking)")

    (run_dir / "driver" / "leg_assertions.json").write_text(
        json.dumps({"resolved": resolved, "failures": failures, "exit": 1 if failures else 0},
                   ensure_ascii=False, indent=1), encoding="utf-8")
    if failures:
        for f in failures:
            print("FAIL:", f)
        return 1
    print("LEG ASSERTIONS PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
