#!/usr/bin/env python3
"""FE-START-EMPTY-DRAFT-1 attribution evidence collector (read-only).

Collects deterministic, machine-readable evidence from the agent's ProjectHistory
draft root and the M2-EXPERIENCE-1 run artifacts. Writes JSON bundles under
coord/runs/FE-START-EMPTY-DRAFT-1/evidence/. Never writes into the draft root.

Usage:
    python collect_evidence.py [--draft-root PATH] [--out PATH]

Windows default draft root: %LOCALAPPDATA%\\Vit\\ProjectHistory\\drafts
(history.defaultDraftProjectPath, agent/internal/history/history.go:2260-2288).
"""
import argparse
import hashlib
import json
import os
import sys
from datetime import datetime, timezone

DEFAULT_DRAFT_ROOT = os.path.join(
    os.environ.get("LOCALAPPDATA", os.path.expanduser("~\\AppData\\Local")),
    "Vit", "ProjectHistory", "drafts")

ROUNDS = [
    # label, implicit (bare) draft, explicit (full) draft
    ("round1_2026-10-15_0952", "draft_20261015T015232_041e9c3d", "draft_20261015T015233_4a3e9fef"),
    ("round2_2026-10-15_1237", "draft_20261015T043731_c86c7a3b", "draft_20261015T043732_236b4272"),
    ("round3_2026-10-15_2100", "draft_20261015T130048_1f3dff6a", "draft_20261015T130051_1fdf3d88"),
    ("round4_2026-10-16_1816", "draft_20261016T101633_4494fa5a", "draft_20261016T101634_b87bbe28"),
    ("live5_2026-10-18_1345", "draft_20261018T054528_95b6e3cc", "draft_20261018T054530_2ffafb51"),
]

OLD_DRAFT = "draft_20260905T134132_3866b080"  # 42MB startup-restore target (card item 3)


def iso(ts):
    return datetime.fromtimestamp(ts, tz=timezone.utc).astimezone().isoformat(timespec="seconds")


def sha256(path, limit=None):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1 << 20)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def read_json(path):
    try:
        with open(path, "rb") as f:
            raw = f.read()
        for enc in ("utf-8-sig", "utf-8", "gb18030"):
            try:
                return json.loads(raw.decode(enc))
            except Exception:
                continue
    except OSError:
        return None
    return None


def walk_meta(base, max_files=400):
    out = []
    for root, dirs, files in os.walk(base):
        for name in files:
            p = os.path.join(root, name)
            try:
                st = os.stat(p)
            except OSError:
                continue
            rel = os.path.relpath(p, base)
            row = {"path": rel, "size": st.st_size, "mtime": iso(st.st_mtime)}
            if st.st_size <= 4096:
                try:
                    row["sha256"] = sha256(p)
                except OSError:
                    pass
            out.append(row)
            if len(out) >= max_files:
                return out
    out.sort(key=lambda r: r["path"])
    return out


def summarize_workspace(draft_dir):
    """Extract identity records: .vit_history/<uuid>/workspace.json, sessions."""
    hist = os.path.join(draft_dir, ".vit_history")
    uuids = []
    if os.path.isdir(hist):
        for name in sorted(os.listdir(hist)):
            if not name.startswith("vitproj_"):
                continue
            ws = read_json(os.path.join(hist, name, "workspace.json"))
            entry = {"uuid": name}
            if isinstance(ws, dict):
                entry.update({k: ws.get(k) for k in (
                    "project_path", "project_uuid", "migrated_from_legacy",
                    "legacy_history_root", "legacy_project_state",
                    "migration_completed_at")})
            uuids.append(entry)
    sessions = []
    sroot = os.path.join(hist, ".sessions")
    if os.path.isdir(sroot):
        for uuid in sorted(os.listdir(sroot)):
            d = os.path.join(sroot, uuid)
            if not os.path.isdir(d):
                continue
            for sess in sorted(os.listdir(d)):
                sp = os.path.join(d, sess, "session.json")
                sj = read_json(sp)
                st = os.stat(sp) if os.path.exists(sp) else None
                sessions.append({"uuid": uuid, "session": sess,
                                 "mtime": iso(st.st_mtime) if st else "",
                                 "status": sj.get("status") if isinstance(sj, dict) else None,
                                 "project_path": sj.get("project_path") if isinstance(sj, dict) else None,
                                 "updated_at": sj.get("updated_at") if isinstance(sj, dict) else None})
    return {"uuid_workspaces": uuids, "sessions": sessions}


def summarize_draft(base, name):
    d = os.path.join(base, name)
    if not os.path.isdir(d):
        return {"dir": name, "exists": False}
    files = walk_meta(d)
    has_history = any(f["path"].startswith(".vit_history") for f in files)
    latest = max((f["mtime"] for f in files), default="")
    return {
        "dir": name,
        "exists": True,
        "created_mtime": iso(os.stat(d).st_ctime),
        "dir_mtime": iso(os.stat(d).st_mtime),
        "file_count": len(files),
        "has_vit_history": has_history,
        "has_vit_agent": any(f["path"].startswith(".vit_agent") for f in files),
        "latest_file_mtime": latest,
        "unsaved_vit": next((f for f in files if f["path"].lower() == "unsaved.vit"), None),
        "identity": summarize_workspace(d),
        "files": files,
    }


def summarize_old_draft(base, name):
    d = os.path.join(base, name)
    out = {"dir": name, "exists": os.path.isdir(d)}
    if not out["exists"]:
        return out
    big = []
    runtime_states = []
    for root, dirs, files in os.walk(d):
        for f in files:
            p = os.path.join(root, f)
            rel = os.path.relpath(p, d)
            try:
                st = os.stat(p)
            except OSError:
                continue
            if f.lower() == "agent_runtime_state.json":
                head = ""
                try:
                    with open(p, "rb") as fh:
                        head = fh.read(600).decode("utf-8", "replace")
                except OSError:
                    pass
                runtime_states.append({"path": rel, "size": st.st_size,
                                       "mtime": iso(st.st_mtime),
                                       "head": head})
            if st.st_size > 10 * 1024 * 1024:
                big.append({"path": rel, "size": st.st_size, "mtime": iso(st.st_mtime)})
    out["runtime_states"] = runtime_states
    out["big_files"] = big
    return out


def summarize_run_state(run_dir):
    names = ["agent_state_1003_check1.json", "agent_state_1003_check2.json",
             "agent_state_1005_after_second_stop.json", "agent_state_round4_1820.json",
             "runtime_status_1003.json", "runtime_status_round3_2100.json",
             "runtime_status_round4_1820.json"]
    out = {}
    for n in names:
        j = read_json(os.path.join(run_dir, n))
        if not isinstance(j, dict):
            continue
        ph = j.get("project_history") or {}
        sh = j.get("shadow") or {}
        out[n] = {
            "project_history": {k: ph.get(k) for k in (
                "project_path", "project_uuid", "draft", "initialized", "head",
                "baseline_commit", "commit_count", "worktree_count") if k in ph},
            "shadow": {k: sh.get(k) for k in (
                "initialized", "track_count", "user_track_count", "engine_track_count",
                "internal_track_count", "last_delta_seq", "delta_seq_gaps",
                "nodes_by_uid", "orphan_uid_keys", "state_epoch", "project_uuid",
                "project_path") if k in sh},
        }
    return out


def summarize_events(run_dir):
    out = {}
    for name in ("events_round3_2100.json", "events_round4_1817.json"):
        j = read_json(os.path.join(run_dir, name))
        if not isinstance(j, dict):
            continue
        rows = []
        for e in j.get("events") or []:
            if e.get("seq") in (5, 6, 7, 8) or e.get("type") in (
                    "turn.completed", "trajectory.turn.completed"):
                payload = e.get("payload") or {}
                result = payload.get("result") or {}
                rows.append({
                    "seq": e.get("seq"), "type": e.get("type"),
                    "item_type": e.get("item_type"), "status": e.get("status"),
                    "title": e.get("title"), "body": e.get("body"),
                    "tool": payload.get("tool"),
                    "command_name": payload.get("command_name"),
                    "result": {k: result.get(k) for k in (
                        "status", "track_count", "user_track_count", "tracks",
                        "error", "command_ack") if k in result},
                })
        out[name] = rows
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--draft-root", default=DEFAULT_DRAFT_ROOT)
    ap.add_argument("--run-dir",
                    default=os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(
                        os.path.abspath(__file__)))), "M2-EXPERIENCE-1"))
    ap.add_argument("--out-dir",
                    default=os.path.join(os.path.dirname(os.path.dirname(
                        os.path.abspath(__file__))), "evidence"))
    args = ap.parse_args()
    os.makedirs(args.out_dir, exist_ok=True)

    bundle = {
        "generated_at": datetime.now(tz=timezone.utc).astimezone().isoformat(timespec="seconds"),
        "host": os.environ.get("COMPUTERNAME", ""),
        "draft_root": args.draft_root,
        "rounds": [],
    }
    for label, implicit, explicit in ROUNDS:
        bundle["rounds"].append({
            "label": label,
            "implicit": summarize_draft(args.draft_root, implicit),
            "explicit": summarize_draft(args.draft_root, explicit),
        })
    bundle["old_draft_42mb"] = summarize_old_draft(args.draft_root, OLD_DRAFT)
    bundle["m2_run_state"] = summarize_run_state(args.run_dir)
    bundle["m2_events_tracklist"] = summarize_events(args.run_dir)

    p = os.path.join(args.out_dir, "draft_inventory.json")
    with open(p, "w", encoding="utf-8") as f:
        json.dump(bundle, f, ensure_ascii=False, indent=1)
    print("wrote", p, os.path.getsize(p), "bytes")


if __name__ == "__main__":
    sys.exit(main())
