from pathlib import Path

src = Path(r"D:/Vit_DAW_wt_uxab177/coord/runs/UX-AB-AUDITION-COVERAGE-1/stack_leg/uxab177_driver_base.py")
text = src.read_text(encoding="utf-8")

start = text.index("    # ---- second stage: audition session extraction + judgment probes -------")
end = text.index('    signals["judgment_stage"] = judgment_stage')

new_stage = '''    # ---- second stage: topology-face extraction + probes (card 177) -------
    # The round's assertion faces live in assert_uxab177.py; this stage only
    # captures: every audition:dynamic:* session mounted this round (topo_* =
    # the direct bus legs' bracket, dyn_* = the ticket bracket), the executed
    # command surface (connect/load legs), and a judgment battery against the
    # topo session (identity refusal + correct POST; the answerable-record
    # slot is one-per-conversation by 174 design, so a later ticket mount may
    # have replaced the topo card -- both outcomes recorded verbatim).
    judgment_stage: Dict[str, Any] = {"conversation_id": conversation_id}
    dynamic_sessions: List[Dict[str, Any]] = []
    if events:
        for event in events.get("events") or []:
            if not isinstance(event, dict) or event.get("type") != "audition.ready":
                continue
            payload = event.get("payload") or {}
            session = payload.get("session") or {}
            sid = str(session.get("session_id") or "")
            if sid.startswith("audition:dynamic:"):
                dynamic_sessions.append({"session": session, "event_seq": event.get("seq")})
    topo_rows = [row for row in dynamic_sessions if str((row["session"] or {}).get("session_id") or "").startswith("audition:dynamic:topo_")]
    ticket_rows = [row for row in dynamic_sessions if row not in topo_rows]
    judgment_stage["dynamic_sessions"] = [
        {"session_id": str((row["session"] or {}).get("session_id") or ""), "event_seq": row.get("event_seq")}
        for row in dynamic_sessions
    ]
    judgment_stage["topo_session_found"] = bool(topo_rows)
    judgment_stage["ticket_session_found"] = bool(ticket_rows)
    (out_dir / "audition_sessions_all.json").write_text(json.dumps(dynamic_sessions, ensure_ascii=False, indent=1), encoding="utf-8")

    executed_command_names = sorted(set(executed_commands))
    judgment_stage["executed_commands"] = executed_command_names
    judgment_stage["connect_leg_dispatched"] = "track.send.connect" in executed_command_names
    judgment_stage["load_leg_dispatched"] = "rack_add_node" in executed_command_names or "rack.add_node" in executed_command_names

    if topo_rows:
        session_payload = topo_rows[-1]["session"]
        session_event_seq = topo_rows[-1].get("event_seq")
        (out_dir / "topo_audition_session.json").write_text(json.dumps(topo_rows, ensure_ascii=False, indent=1), encoding="utf-8")
        turn_id = str(session_payload.get("turn_id") or "")
        round_id = str(session_payload.get("round_id") or "")
        session_id = str(session_payload.get("session_id") or "")
        revision = str(session_payload.get("project_revision") or "")
        identity = {
            "conversation_id": conversation_id,
            "turn_id": turn_id,
            "round_id": round_id,
            "audition_session_id": session_id,
            "project_revision": revision,
        }
        judgment_stage["topo_identity"] = identity

        # (1) wrong turn_id against the topo session -> expect 409.
        wrong_turn = dict(identity)
        wrong_turn["turn_id"] = turn_id + "__wrong"
        wrong_turn["heard_difference"] = "yes"
        wrong_turn["preference"] = "b"
        probe1 = post_judgment(base_url, wrong_turn)
        (out_dir / "judgment_409_identity.json").write_text(json.dumps(
            {"request": wrong_turn, "http_status": probe1["http_status"], "body": probe1["body"]},
            ensure_ascii=False, indent=1), encoding="utf-8")
        judgment_stage["probe_wrong_turn"] = {"expect": 409, "got": probe1["http_status"]}

        # (2) correct-identity topo judgment -> expect 200 while the topo card
        # is the conversation's answerable record; 409 with session-mismatch
        # wording if a later ticket mount replaced it (coexistence boundary,
        # both recorded; the assert script accepts 200-or-409 and requires the
        # 409 body to name the mismatch, never a silent fallthrough).
        correct = dict(identity)
        correct["heard_difference"] = "yes"
        correct["preference"] = "b"
        correct["free_text"] = "判定探针：topo 通道选 B，保留这次装载"
        probe2 = post_judgment(base_url, correct)
        (out_dir / "judgment_topo_ok.json").write_text(json.dumps(
            {"request": correct, "http_status": probe2["http_status"], "body": probe2["body"]},
            ensure_ascii=False, indent=1), encoding="utf-8")
        judgment_stage["probe_topo_correct"] = {"expect": "200 or 409(replaced)", "got": probe2["http_status"]}
        outcome = (probe2["body"] or {}).get("semantic_dynamic_audition") or {}
        judgment_stage["topo_judgment_outcome"] = outcome

        # (3) candidate render assets: resolve the topo session's candidate
        # source refs on disk (the payload carries absolute file paths).
        assets = []
        for candidate in session_payload.get("candidates") or []:
            if not isinstance(candidate, dict):
                continue
            ref = str(candidate.get("source_ref") or "")
            row = {"label": str(candidate.get("label") or ""), "source_ref": ref}
            if ref:
                p = Path(ref)
                row["exists"] = p.exists()
                row["size"] = p.stat().st_size if p.exists() else 0
            assets.append(row)
        judgment_stage["topo_candidate_assets"] = assets
        (out_dir / "topo_candidate_assets.json").write_text(json.dumps(assets, ensure_ascii=False, indent=1), encoding="utf-8")

        time.sleep(2.0)
        events_after = pull_events(base_url, conversation_id)
        if events_after:
            (out_dir / "events_after_judgment.json").write_text(
                json.dumps(events_after, ensure_ascii=False, indent=1), encoding="utf-8")
            judgment_stage["events_after_count"] = len(events_after.get("events") or [])

'''
text = text[:start] + new_stage + text[end:]

# trim the session_log summary keys to the ones this driver still produces
text = text.replace(
    '''    for key in ("dynamic_session_found", "probe_wrong_turn", "probe_fake_experiment_session",
                "probe_correct", "goal_after_probes", "goal_after_fallback",
                "experiment_judgment_fallback_status"):''',
    '''    for key in ("topo_session_found", "ticket_session_found", "connect_leg_dispatched",
                "load_leg_dispatched", "probe_wrong_turn", "probe_topo_correct"):''')

# docstring: replace the 174 header text with the 177 face
old_doc = text[text.index('"""'):text.index('"""', 3) + 3]
new_doc = '''#!/usr/bin/env python3
"""UX-AB-AUDITION-COVERAGE-1 (card 177) real-stack reproduction leg, 2026-10-23.

Adapted from aba174_driver.py (card 174; 146/170 driver family) with the
assertion faces swapped to this card's target: the manual-session 场次⑤ form
(bare utterance "给人声加点空间感", full access, cards auto-answered) must,
when the chain dispatches the direct bus-topology legs (track.send.connect
auto_create + rack_add_node -- the forensically-confirmed non-ticket effect
mutation path), mount an equivalent audition session in the
audition:dynamic:topo_* family (the 174 bracket only covered the semantic
ticket's parameter write). Capture only; assertions live in assert_uxab177.py.

Run budget (pre-declared): <=3 rounds; round retries only for env interrupts
or a round that never exercises the direct-legs form (no track.send.connect
in the executed surface). Success 
