#!/usr/bin/env python3
"""RECENTS-HYGIENE-1 cleanup: remove device/smoke fixture entries from kernel recents.

Safety contract:
- Removes ONLY entries exactly matching the classified inventory list
  (forensics/recents_inventory.json entries_before_cleanup[].path).
- Any entry NOT in the inventory list is KEPT and reported (uncertain entries
  are never deleted).
- User real project entries: none present; if any path under
  %APPDATA%/Vit-DAW/projects/ were ever matched for removal the script aborts.
- Pre-write process check is done by the caller (tasklist must be empty for
  VitApp/VitAgent/godot).
- Atomic write: temp file in same dir, then os.replace.
- Idempotent: re-run on an already-clean file is a no-op (0 removed).

Run: python -I cleanup_recents.py <live_json> <inventory_json> <backup_dir> <snapshots_dir>
"""
from __future__ import annotations

import hashlib
import json
import os
import sys
from datetime import datetime

USER_PROJECTS_DIR_MARK = "Vit-DAW\\projects"


def sha256(path: str) -> str:
    with open(path, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


def main() -> int:
    live, inventory_path, backup_dir, snapshots_dir = sys.argv[1:5]
    with open(inventory_path, encoding="utf-8") as f:
        inventory = json.load(f)

    remove_set = {e["path"] for e in inventory["entries_before_cleanup"]}
    for p in remove_set:
        if USER_PROJECTS_DIR_MARK in p:
            print(f"ABORT: inventory entry classified for removal contains user projects dir mark: {p}")
            return 3

    pre_hash = sha256(live)
    with open(live, encoding="utf-8") as f:
        data = json.load(f)
    entries = data.get("recent_projects", [])

    kept, removed, unexpected = [], [], []
    for e in entries:
        if e in remove_set:
            removed.append(e)
        else:
            kept.append(e)
            if USER_PROJECTS_DIR_MARK in e:
                unexpected.append(e)  # user entry present: kept, never touched
    if unexpected:
        print(f"NOTE: {len(unexpected)} user-project entries present and KEPT (not in removal set)")
    if not removed:
        print("NOTHING_TO_REMOVE (already clean or inventory drift) - no write performed")
        print(json.dumps({"pre_hash": pre_hash, "kept": kept, "removed": []}, ensure_ascii=False, indent=1))
        return 0

    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")

    # re-verify pre-cleanup backup still byte-identical to live content
    with open(live, "rb") as f:
        live_bytes = f.read()
    import glob
    backups = sorted(glob.glob(os.path.join(backup_dir, "global_project_config.pre_cleanup_*.json")))
    if not backups:
        print("ABORT: no pre_cleanup backup found")
        return 3
    with open(backups[-1], "rb") as f:
        if f.read() != live_bytes:
            print("ABORT: live file drifted from pre_cleanup backup - manual review required")
            return 3

    out = {"recent_projects": kept}
    # kernel-juce style: json.dumps default separators (', ', ': ') match the
    # observed on-disk format exactly; single line, no trailing newline
    text = json.dumps(out, ensure_ascii=False)
    tmp = os.path.join(os.path.dirname(live), f".global_project_config.tmp_{stamp}")
    with open(tmp, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    os.replace(tmp, live)

    post_hash = sha256(live)
    snap = os.path.join(snapshots_dir, f"global_project_config.post_cleanup_{stamp}.json")
    with open(live, "rb") as f:
        with open(snap, "wb") as g:
            g.write(f.read())

    report = {
        "stamp": stamp,
        "pre_hash": pre_hash,
        "post_hash": post_hash,
        "removed_count": len(removed),
        "removed": removed,
        "kept_count": len(kept),
        "kept": kept,
        "backup_verified_byte_identical": True,
        "snapshot": snap,
    }
    rep_path = os.path.join(snapshots_dir, f"cleanup_report_{stamp}.json")
    with open(rep_path, "w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=1)
    print(json.dumps(report, ensure_ascii=False, indent=1))
    return 0


if __name__ == "__main__":
    sys.exit(main())
