#!/usr/bin/env python3
"""UX-AB-AUDITION-IMPL-1 (card 174) per-round assertion leg.

Reads one round directory produced by aba174_driver.py and checks the
pre-declared assertion faces A-F (card UX-AB-AUDITION-IMPL-1 真栈补证腿 §3):
  A  dynamic chain completed: goal_status_final=completed AND the executed
     response carries the settlement/semantic_dynamic face
     (workflow_data.execution_receipt + semantic_dynamic_audition stamp)
  B  audition.ready in the same round: stamp.mounted=true,
     audition_session_id=audition:dynamic:*; events carry audition.ready
     (ItemID == session id) + trajectory.user_judgment.requested
     (TurnID=dynamic_turn:*)
  C  both render assets on disk: candidate refs resolve to non-empty files,
     recomputed sha256 matches the recorded provenance, before/after
     revisions differ, file names carry the revision provenance
  D  third judgment route: judgment_ok 200 + booked evidence id;
     judgment_409_identity=409; judgment_409_experiment=409; the
     post-judgment events carry the judgment event (+ disclosure if present)
  E  blind face: session blind=true (VIT_DAW_AUDITION_BLIND=1) and candidate
     labels stay A/B
  F  webui-reducer contract fields: audition.ready payload
     schema_version=vit.kernel_audition.v1 + session turn_id/round_id + both
     candidates ready; judgment.requested payload node_kind=user_judgment,
     status=waiting_for_user, details.audition_session_id

Writes <round>/leg_assertions.json. Exit 0 = PASS (all faces hold), 1 = FAIL
(with the §8 classification recorded per failure). Assertions are fixed
upfront; no post-hoc weakening.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional

FAILURES_CLASS = {
    "A_goal_not_completed": "no_dynamic_chain",
    "A_no_executed_response": "no_dynamic_chain",
    "A_stamp_missing_from_responses": "executed_face_capture_gap",
    "B_not_mounted": "no_audition_ready",
    "B_session_prefix": "no_audition_ready",
    "B_no_audition_ready_event": "no_audition_ready",
    "B_no_judgment_requested_event": "no_audition_ready",
    "C_candidate_file_missing": "assets_missing",
    "C_sha256_mismatch": "assets_missing",
    "C_revision_collision": "assets_missing",
    "C_filename_provenance": "assets_missing",
    "D_judgment_not_200": "judgment_route_failed",
    "D_judgment_no_evidence": "judgment_route_failed",
    "D_identity_probe_not_409": "judgment_route_failed",
    "D_experiment_probe_not_409": "experiment_409_regression",
    "D_no_judgment_event": "judgment_route_failed",
    "E_blind_not_enabled": "blind_face_missing",
    "E_labels_not_AB": "blind_face_missing",
    "F_ready_payload_contract": "no_audition_ready",
    "F_judgment_requested_payload_contract": "no_audition_ready",
}


def load_json(path: Path) -> Optional[Any]:
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:  # noqa: BLE001
        return None


def sha256_of(path: Path) -> Optional[str]:
    try:
        digest = hashlib.sha256()
        with open(path, "rb") as handle:
            for chunk in iter(lambda: handle.read(1 << 20), b""):
                digest.update(chunk)
        return digest.hexdigest()
    except Exception:  # noqa: BLE001
        return None


def response_files(round_dir: Path) -> List[Path]:
    names = [p for p in round_dir.glob("*.json")
             if p.name.startswith(("chat_", "hop_", "walkhop_")) and p.name.endswith(".json")]
    return sorted(names)


def find_executed_response(round_dir: Path) -> Dict[str, Any]:
    for path in response_files(round_dir):
        row = load_json(path)
        if not isinstance(row, dict):
            continue
        data = row.get("workflow_data")
        if not isinstance(data, dict):
            continue
        if data.get("execution_receipt") is not None or data.get("status") == "executed":
            stamp = data.get("semantic_dynamic_audition")
            return {
                "file": path.name,
                "workflow_status": data.get("status"),
                "has_execution_receipt": data.get("execution_receipt") is not None,
                "stamp": stamp if isinstance(stamp, dict) else None,
                "receipt": data.get("execution_receipt"),
            }
    return {}


def candidate_rows(session: Dict[str, Any]) -> List[Dict[str, Any]]:
    rows_out: List[Dict[str, Any]] = []
    for row in session.get("candidates") or []:
        if isinstance(row, dict):
            rows_out.append(row)
    return rows_out


def event_rows(events_doc: Optional[Any]) -> List[Dict[str, Any]]:
    if not isinstance(events_doc, dict):
        return []
    rows = events_doc.get("events") or []
    return [r for r in rows if isinstance(r, dict)]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--round-dir", required=True)
    args = parser.parse_args()
    round_dir = Path(args.round_dir).resolve()

    out: Dict[str, Any] = {"round_dir": str(round_dir), "faces": {}, "failures": [], "classification": []}
    summary = load_json(round_dir / "variant_summary.json") or {}
    events_doc = load_json(round_dir / "events.json")
    events_after = load_json(round_dir / "events_after_judgment.json") or events_doc
    audition_doc = load_json(round_dir / "audition_session.json") or {}

    # ---------- A: dynamic chain completed + executed face -------------------
    face_a: Dict[str, Any] = {"goal_status_final": summary.get("goal_status_final")}
    executed = find_executed_response(round_dir)
    face_a["executed_response"] = executed or None
    face_a["settle_reason"] = summary.get("settle_reason")
    receipt = (executed or {}).get("receipt") or {}
    face_a["receipt_fields"] = {
        "receipt_id": receipt.get("receipt_id"),
        "after_revision": receipt.get("after_revision"),
        "settlement_verified": receipt.get("settlement_verified"),
        "ticket_id": receipt.get("ticket_id"),
    }
    a_ok = True
    if summary.get("goal_status_final") != "completed":
        out["failures"].append("A_goal_not_completed")
        a_ok = False
    if not executed or not executed.get("has_execution_receipt"):
        out["failures"].append("A_no_executed_response")
        a_ok = False
    stamp = executed.get("stamp")
    if a_ok and not isinstance(stamp, dict):
        out["failures"].append("A_stamp_missing_from_responses")
        a_ok = False
    out["faces"]["A"] = {"ok": a_ok, **face_a}

    # ---------- B: audition.ready same round, dynamic identity ---------------
    session = (audition_doc.get("session") if isinstance(audition_doc, dict) else None) or {}
    session_id = str(session.get("session_id") or "")
    face_b: Dict[str, Any] = {
        "session_id": session_id,
        "turn_id": session.get("turn_id"),
        "round_id": session.get("round_id"),
        "stamp_file": (executed or {}).get("file"),
    }
    b_ok = True
    stamp = stamp if isinstance(stamp, dict) else ({} if not executed else None)
    if not isinstance(stamp, dict) or stamp.get("mounted") is not True:
        face_b["stamp"] = stamp
        out["failures"].append("B_not_mounted")
        b_ok = False
    if not session_id.startswith("audition:dynamic:"):
        out["failures"].append("B_session_prefix")
        b_ok = False
    ready_events = [e for e in event_rows(events_doc)
                    if e.get("type") == "audition.ready" and str(e.get("item_id") or "") == session_id]
    face_b["audition_ready_event_seqs"] = [e.get("seq") for e in ready_events]
    if not ready_events:
        out["failures"].append("B_no_audition_ready_event")
        b_ok = False
    requested = [e for e in event_rows(events_doc)
                 if e.get("type") == "trajectory.user_judgment.requested"
                 and str((e.get("payload") or {}).get("turn_id") or "").startswith("dynamic_turn:")]
    face_b["judgment_requested_events"] = [
        {"seq": e.get("seq"), "turn_id": (e.get("payload") or {}).get("turn_id"),
         "session": ((e.get("payload") or {}).get("details") or {}).get("audition_session_id")}
        for e in requested
    ]
    requested_for_session = [e for e in requested
                             if str(((e.get("payload") or {}).get("details") or {}).get("audition_session_id") or "") == session_id]
    if not requested_for_session:
        out["failures"].append("B_no_judgment_requested_event")
        b_ok = False
    if isinstance(stamp, dict) and stamp.get("audition_session_id") and session_id:
        face_b["stamp_matches_session"] = str(stamp.get("audition_session_id")) == session_id
        if not face_b["stamp_matches_session"]:
            out["failures"].append("B_not_mounted")
            b_ok = False
    out["faces"]["B"] = {"ok": b_ok, **face_b}

    # ---------- C: both render assets on disk --------------------------------
    rows = candidate_rows(session)
    face_c: Dict[str, Any] = {"candidates": []}
    c_ok = len(rows) == 2
    revisions = []
    for row in rows:
        source_ref = str(row.get("source_ref") or "")
        preview_ref = str(row.get("preview_ref") or "")
        label = str(row.get("label") or "")
        entry: Dict[str, Any] = {"label": label, "source_ref": source_ref,
                                 "preview_ref": preview_ref,
                                 "project_revision": row.get("project_revision"),
                                 "render_revision": row.get("render_revision")}
        path = Path(source_ref) if source_ref else None
        if path is not None and path.exists() and path.stat().st_size > 0:
            entry["file_exists"] = True
            entry["size_bytes"] = path.stat().st_size
            digest = sha256_of(path)
            entry["sha256_recomputed"] = digest
            recorded = preview_ref.split("audio_file:", 1)[1] if preview_ref.startswith("audio_file:") else ""
            entry["sha256_recorded"] = recorded
            entry["sha256_match"] = bool(digest and recorded and digest == recorded)
            if not entry["sha256_match"]:
                out["failures"].append("C_sha256_mismatch")
                c_ok = False
            name = path.name.lower()
            entry["filename_provenance"] = ("before_revision_" in name) or ("after_revision_" in name)
            if not entry["filename_provenance"]:
                out["failures"].append("C_filename_provenance")
                c_ok = False
        else:
            entry["file_exists"] = False
            out["failures"].append("C_candidate_file_missing")
            c_ok = False
        revisions.append(str(row.get("project_revision") or ""))
        face_c["candidates"].append(entry)
    face_c["revisions"] = revisions
    face_c["revisions_distinct"] = len(revisions) == 2 and revisions[0] != revisions[1] and all(revisions)
    if not face_c["revisions_distinct"]:
        out["failures"].append("C_revision_collision")
        c_ok = False
    out["faces"]["C"] = {"ok": c_ok, **face_c}

    # ---------- D: third judgment route --------------------------------------
    face_d: Dict[str, Any] = {}
    d_ok = True
    judgment_ok = load_json(round_dir / "judgment_ok.json") or {}
    face_d["judgment_ok_status"] = judgment_ok.get("http_status")
    outcome = (judgment_ok.get("body") or {}).get("semantic_dynamic_audition") or {}
    face_d["judgment_decision"] = outcome.get("decision")
    face_d["judgment_status"] = outcome.get("status")
    face_d["judgment_evidence_id"] = outcome.get("judgment_evidence_id")
    face_d["blind_disclosure"] = outcome.get("blind_disclosure")
    if judgment_ok.get("http_status") != 200:
        out["failures"].append("D_judgment_not_200")
        d_ok = False
    if not str(outcome.get("judgment_evidence_id") or ""):
        out["failures"].append("D_judgment_no_evidence")
        d_ok = False
    probe_identity = load_json(round_dir / "judgment_409_identity.json") or {}
    face_d["identity_probe_status"] = probe_identity.get("http_status")
    face_d["identity_probe_error"] = (probe_identity.get("body") or {}).get("error")
    if probe_identity.get("http_status") != 409:
        out["failures"].append("D_identity_probe_not_409")
        d_ok = False
    probe_experiment = load_json(round_dir / "judgment_409_experiment.json") or {}
    face_d["experiment_probe_status"] = probe_experiment.get("http_status")
    face_d["experiment_probe_error"] = (probe_experiment.get("body") or {}).get("error")
    if probe_experiment.get("http_status") != 409:
        out["failures"].append("D_experiment_probe_not_409")
        d_ok = False
    judgment_events = [e for e in event_rows(events_after) if e.get("type") == "semantic_dynamic.audition.judgment"]
    face_d["judgment_events"] = [{"seq": e.get("seq"), "status": e.get("status")} for e in judgment_events]
    if not judgment_events:
        out["failures"].append("D_no_judgment_event")
        d_ok = False
    recorded_events = [e for e in event_rows(events_after)
                       if e.get("type") == "trajectory.user_judgment.recorded"
                       and str((e.get("payload") or {}).get("turn_id") or "").startswith("dynamic_turn:")]
    face_d["recorded_trajectory_events"] = [
        {"seq": e.get("seq"), "turn_id": (e.get("payload") or {}).get("turn_id")} for e in recorded_events
    ]
    out["faces"]["D"] = {"ok": d_ok, **face_d}

    # ---------- E: blind face -------------------------------------------------
    face_e: Dict[str, Any] = {"session_blind": session.get("blind")}
    labels = sorted(str(row.get("label") or "") for row in rows)
    face_e["candidate_labels"] = labels
    e_ok = session.get("blind") is True
    if not e_ok:
        out["failures"].append("E_blind_not_enabled")
    if labels != ["A", "B"]:
        out["failures"].append("E_labels_not_AB")
        e_ok = False
    out["faces"]["E"] = {"ok": e_ok, **face_e}

    # ---------- F: webui reducer contract fields ------------------------------
    face_f: Dict[str, Any] = {}
    f_ok = True
    ready_event = ready_events[0] if ready_events else None
    ready_payload = (ready_event or {}).get("payload") or {}
    ready_session = ready_payload.get("session") or {}
    face_f["ready_schema_version"] = ready_payload.get("schema_version")
    face_f["ready_session_turn_id"] = ready_session.get("turn_id")
    face_f["ready_session_round_id"] = ready_session.get("round_id")
    ready_rows = candidate_rows(ready_session)
    face_f["ready_candidates_ready"] = [row.get("status") for row in ready_rows]
    if (ready_payload.get("schema_version") != "vit.kernel_audition.v1"
            or not ready_session.get("turn_id") or not ready_session.get("round_id")
            or len(ready_rows) != 2
            or any(str(row.get("status")).lower() != "ready" for row in ready_rows)):
        out["failures"].append("F_ready_payload_contract")
        f_ok = False
    requested_event = requested_for_session[0] if requested_for_session else None
    requested_payload = (requested_event or {}).get("payload") or {}
    face_f["requested_node_kind"] = requested_payload.get("node_kind")
    face_f["requested_status"] = requested_payload.get("status")
    face_f["requested_session"] = (requested_payload.get("details") or {}).get("audition_session_id")
    if (requested_payload.get("node_kind") != "user_judgment"
            or requested_payload.get("status") != "waiting_for_user"
            or str((requested_payload.get("details") or {}).get("audition_session_id") or "") != session_id):
        out["failures"].append("F_judgment_requested_payload_contract")
        f_ok = False
    out["faces"]["F"] = {"ok": f_ok, **face_f}

    # ---------- verdict --------------------------------------------------------
    out["failures"] = list(dict.fromkeys(out["failures"]))
    out["classification"] = sorted({FAILURES_CLASS[name] for name in out["failures"] if name in FAILURES_CLASS})
    out["conversation_id"] = summary.get("conversation_id")
    out["exit"] = 0 if not out["failures"] else 1
    (round_dir / "leg_assertions.json").write_text(json.dumps(out, ensure_ascii=False, indent=1), encoding="utf-8")
    for face, row in out["faces"].items():
        print(f"[{face}] {'PASS' if row['ok'] else 'FAIL'}")
    if out["failures"]:
        print("failures:", ", ".join(out["failures"]), "| classification:", ", ".join(out["classification"]))
    return out["exit"]


if __name__ == "__main__":
    sys.exit(main())
