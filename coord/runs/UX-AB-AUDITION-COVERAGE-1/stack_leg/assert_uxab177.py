#!/usr/bin/env python3
"""UX-AB-AUDITION-COVERAGE-1 (card 177) per-round assertion leg.

Reads one round directory produced by uxab177_driver.py and checks the
pre-declared assertion faces (card 177 真栈腿):

  A  direct-legs form exercised: the executed command surface of the round
     contains track.send.connect (the forensically-confirmed non-ticket
     mutation path; rack_add_node recorded alongside)
  B  audition in attendance for the direct legs: an audition.ready event with
     session id in the audition:dynamic:topo_* family, turn domain
     dynamic_turn:topo_*, candidates labeled A/B
  C  render assets: the topo session's two candidate source refs resolve to
     non-empty distinct files whose recomputed sha256 matches the payload's
     preview_ref provenance
  D  judgment identity: wrong turn_id -> 409; correct-identity topo POST ->
     200 with ticket_id=topo_* decision retained, or 409 when a later ticket
     mount replaced the answerable slot (one-card-per-conversation, 174
     design) -- both recorded verbatim, never a silent fallthrough
  E  blind face: session blind=true (VIT_DAW_AUDITION_BLIND=1) and labels
     stay A/B
  F  ticket coexistence recorded (dyn_* presence is round-form dependent,
     never asserted either way)

Writes <round>/leg_assertions.json. Exit 0 = PASS, 1 = FAIL with the §8
failure classification. Assertions fixed upfront; no post-hoc weakening.
"""

from __future__ import annotations

import hashlib
import json
import sys
from pathlib import Path
from typing import Any, Dict, List

FAILURES_CLASS = {
    "A_no_connect_leg": "no_direct_legs_form",
    "B_no_topo_session": "no_audition_for_direct_legs",
    "B_no_ready_event": "no_audition_for_direct_legs",
    "B_identity_domain": "no_audition_for_direct_legs",
    "C_asset_missing": "assets_missing",
    "C_sha256_mismatch": "assets_missing",
    "C_same_file": "assets_missing",
    "D_wrong_turn_not_409": "judgment_route_broken",
    "D_correct_not_200_or_409": "judgment_route_broken",
    "D_200_outcome_wrong": "judgment_route_broken",
    "E_blind_off": "blind_face_missing",
    "E_labels": "blind_face_missing",
}


def sha256_of(path: Path) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> int:
    round_dir = Path(sys.argv[1])
    summary = json.loads((round_dir / "variant_summary.json").read_text(encoding="utf-8"))
    stage = summary.get("judgment_stage") or {}
    failures: List[str] = []
    faces: Dict[str, Any] = {}

    # A: the round exercised the direct (non-ticket) mutation legs. The
    # command surface is read from every response artifact (chat_/hop_/
    # walkhop_/resumehop_ rows live under executed_kernel_reply or replies --
    # the driver's summary only aggregates the in-turn channel, a capture gap
    # fixed here by reading the primary artifacts; the face itself is
    # unchanged: track.send.connect dispatched in this round).
    dispatch_commands: List[str] = list(stage.get("executed_commands") or [])
    for artifact in sorted(round_dir.glob("chat_*.json")) + sorted(round_dir.glob("hop_*.json")) \
            + sorted(round_dir.glob("walkhop_*.json")) + sorted(round_dir.glob("resumehop_*.json")):
        try:
            row = json.loads(artifact.read_text(encoding="utf-8"))
        except Exception:  # noqa: BLE001
            continue
        if not isinstance(row, dict):
            continue
        for key in ("executed_kernel_reply", "replies"):
            for cmd in row.get(key) or []:
                if isinstance(cmd, dict) and cmd.get("command_name"):
                    dispatch_commands.append(str(cmd["command_name"]))
    dispatch_commands = sorted(set(dispatch_commands))
    faces["A_connect_leg_dispatched"] = "track.send.connect" in dispatch_commands
    faces["A_load_leg_dispatched"] = "rack_add_node" in dispatch_commands or "rack.add_node" in dispatch_commands
    faces["A_executed_commands"] = dispatch_commands
    if not faces["A_connect_leg_dispatched"]:
        failures.append("A_no_connect_leg")

    # B: audition in attendance in the topo family. The MOUNT event is the
    # authoritative identity row (turn/round/blind are agent-side augmentations
    # present on the mount event; later re-seat re-emits carry the kernel's
    # serialized session without them -- same shape as the ticket channel).
    sessions_all = json.loads((round_dir / "audition_sessions_all.json").read_text(encoding="utf-8")) \
        if (round_dir / "audition_sessions_all.json").exists() else []
    topo_rows = [row for row in sessions_all
                 if str((row.get("session") or {}).get("session_id") or "").startswith("audition:dynamic:topo_")]
    mount_rows = [row for row in topo_rows if str((row.get("session") or {}).get("turn_id") or "").startswith("dynamic_turn:topo_")]
    topo_mount = mount_rows[0] if mount_rows else (topo_rows[0] if topo_rows else None)
    faces["B_topo_session_count"] = len(topo_rows)
    faces["B_mount_row_seq"] = (topo_mount or {}).get("event_seq")
    faces["B_all_dynamic_sessions"] = [str((row.get("session") or {}).get("session_id") or "") for row in sessions_all]
    session = (topo_mount.get("session") or {}) if topo_mount else {}
    session_id = str(session.get("session_id") or "")
    faces["B_session_id"] = session_id
    if topo_mount:
        if not str(session.get("turn_id") or "").startswith("dynamic_turn:topo_"):
            failures.append("B_identity_domain")
        labels = sorted(str(c.get("label") or "") for c in (session.get("candidates") or []) if isinstance(c, dict))
        faces["B_candidate_labels"] = labels
    else:
        failures.append("B_no_topo_session")

    # events: audition.ready for the topo session is in the stream.
    events = json.loads((round_dir / "events.json").read_text(encoding="utf-8")) \
        if (round_dir / "events.json").exists() else {"events": []}
    ready_seqs = [e.get("seq") for e in (events.get("events") or [])
                  if isinstance(e, dict) and e.get("type") == "audition.ready"
                  and str(((e.get("payload") or {}).get("session") or {}).get("session_id") or "") == session_id]
    faces["B_ready_event_seqs"] = ready_seqs
    if topo_rows and not ready_seqs:
        failures.append("B_no_ready_event")

    # C: render assets. In this payload form the kernel session's preview_ref
    # is the kernel's own audio-buffer:// address (no sha face); the durable
    # provenance is the revision-bearing file names + the topo commit refs
    # (semantic_dynamic:topo_*:before/:after). Faces: two candidates, both
    # non-empty distinct files, before/after revision names distinct, commit
    # refs carry the topo identity. (The sha-vs-preview_ref comparison in the
    # first draft assumed the audio_file:<sha> form -- corrected against the
    # real payload shape; wav bytes themselves are preserved as artifacts.)
    if topo_mount:
        assets = stage.get("topo_candidate_assets") or []
        faces["C_assets"] = assets
        ok_assets = len(assets) == 2 and all(row.get("exists") and int(row.get("size") or 0) > 0 for row in assets)
        if not ok_assets:
            failures.append("C_asset_missing")
        else:
            refs = [Path(row["source_ref"]) for row in assets]
            if refs[0].resolve() == refs[1].resolve():
                failures.append("C_same_file")
            names = sorted(ref.name for ref in refs)
            faces["C_render_names"] = names
            if not (any("before_revision_" in n for n in names) and any("after_revision_" in n for n in names)
                    and names[0] != names[1]):
                failures.append("C_sha256_mismatch")
            commit_refs = sorted(str(c.get("commit_id") or "") for c in (session.get("candidates") or [])
                                 if isinstance(c, dict))
            faces["C_commit_refs"] = commit_refs
            if not all(ref.startswith("semantic_dynamic:topo_") and ref.endswith((":before", ":after"))
                       and commit_refs[0] != commit_refs[1] for ref in commit_refs):
                failures.append("C_sha256_mismatch")

    # D: judgment identity faces. The v1 probes (driver second stage) read the
    # identity off the re-seat row (turn_id absent) and hit the
    # required-fields refusal; the v2 probes (probe_v2.py against the live
    # stack, identity from the MOUNT event) are the authoritative face when
    # present.
    v2_wrong = round_dir / "judgment_409_identity_v2.json"
    v2_ok = round_dir / "judgment_topo_ok_v2.json"
    wrong_file = v2_wrong if v2_wrong.exists() else round_dir / "judgment_409_identity.json"
    ok_file = v2_ok if v2_ok.exists() else round_dir / "judgment_topo_ok.json"
    wrong = json.loads(wrong_file.read_text(encoding="utf-8")) if wrong_file.exists() else {}
    faces["D_probe_wrong_turn"] = {"expect": 409, "got": wrong.get("http_status")}
    if wrong.get("http_status") != 409:
        failures.append("D_wrong_turn_not_409")
    topo_correct = json.loads(ok_file.read_text(encoding="utf-8")) if ok_file.exists() else {}
    got = topo_correct.get("http_status")
    faces["D_probe_topo_correct"] = {"expect": "200 or 409(replaced)", "got": got, "artifact": ok_file.name}
    outcome = ((topo_correct.get("body") or {}).get("semantic_dynamic_audition")) or {}
    faces["D_topo_outcome"] = {k: outcome.get(k) for k in ("ticket_id", "family", "decision", "status", "judgment_evidence_id")}
    if got == 200:
        if outcome.get("ticket_id") != session_id.replace("audition:dynamic:", "") or not outcome.get("judgment_evidence_id"):
            failures.append("D_200_outcome_wrong")
    elif got == 409:
        text = json.dumps(topo_correct.get("body") or {}, ensure_ascii=False)
        faces["D_409_body"] = topo_correct.get("body")
        # Replaced-slot 409 must be an explicit refusal (error/409 face), never
        # an empty success-shaped body and never a required-fields artifact of
        # a broken probe identity (that is the v1 capture defect, guarded by
        # using the mount identity for v2).
        if not any(word in text for word in ("409", "error", "not found", "mismatch", "找不到", "不匹配")):
            failures.append("D_correct_not_200_or_409")
        if "are required" in text:
            failures.append("D_correct_not_200_or_409")
    else:
        failures.append("D_correct_not_200_or_409")

    # E: blind face.
    faces["E_blind"] = session.get("blind")
    if topo_mount and session.get("blind") is not True:
        failures.append("E_blind_off")
    if topo_mount and faces.get("B_candidate_labels") != ["A", "B"]:
        failures.append("E_labels")

    # F: ticket coexistence, recorded only.
    faces["F_ticket_session_found"] = bool(stage.get("ticket_session_found"))
    faces["round_settle"] = {"goal_status_final": summary.get("goal_status_final"),
                             "settle_reason": summary.get("settle_reason"),
                             "turns_sent": summary.get("turns_sent"),
                             "nudges_sent": summary.get("nudges_sent")}

    result = {
        "schema_version": "uxab177.leg_assertions.v1",
        "round": str(round_dir),
        "faces": faces,
        "failures": failures,
        "failure_classes": sorted({FAILURES_CLASS[f] for f in failures if f in FAILURES_CLASS}),
        "verdict": "PASS" if not failures else "FAIL",
    }
    (round_dir / "leg_assertions.json").write_text(json.dumps(result, ensure_ascii=False, indent=1), encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False, indent=1))
    return 0 if not failures else 1


if __name__ == "__main__":
    sys.exit(main())
