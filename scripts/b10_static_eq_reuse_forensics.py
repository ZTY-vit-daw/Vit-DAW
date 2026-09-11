#!/usr/bin/env python3
"""B10 instance-reuse forensics driver: second static_eq round on a track that already carries the whitelisted EQ instance.

Round 1 is a plain run_free_state_d1_smoke.ps1 -PromptFlavor frequency
-ExpectDomain static_eq invocation against a fresh workdir; this driver then
re-issues the same-flavor experiment against the SAME live stack and the SAME
project (free_state_d1_smoke.py --reuse-existing-project), capturing the
shadow plugin graph before/after and the applied receipts of both rounds.

Hard assertions (exit 1 on violation):
  - the round-2 driver completes the D1 contract (exit 0);
  - the shadow plugin graph is instance-count invariant across round 2
    (per-track plugin id sets identical before and after);
  - the round-2 receipt wrote parameters onto a pre-existing instance
    (plugin_instantiated_by_action is false and plugin_id was already present
    before round 2).

The admitted band value comparison is recorded honestly: a second admission
may target a different gain/frequency (value changed) or the same one
(rewritten to the same value); either way the write itself is proven by the
receipt's readback. NOT_EXERCISED (exit 3) covers model-selection outcomes
that never put round 2 on the already-treated track.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import subprocess
import sys
import time
import urllib.request
from pathlib import Path
from typing import Any

import zmq

NOT_EXERCISED_EXIT = 3


def raw_legacy_command(cmd: str, args: dict[str, Any], timeout_ms: int = 15000) -> dict[str, Any]:
    """Send one legacy command to the kernel's raw ZMQ REQ surface.

    The shadow-derived /agent/state and the sanitized get_project_state tool
    both lose the per-track plugin rows in this session shape (delta-applied
    shadow tracks carry plugins: null), so the graph comparison must read the
    same raw surface the agent's reuse discovery consumes.
    """
    def envelope(session_id: str, channel: str, mtype: str, schema: str, payload: dict[str, Any]) -> dict[str, Any]:
        now = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        return {"vsp_version": "1.0", "schema": schema, "message_id": f"msg_b10{time.time_ns()}",
                "session_id": session_id, "client_id": "agent.main", "role": "agent",
                "channel": channel, "type": mtype, "created_at": now,
                "trace_id": f"trace_b10{time.time_ns()}", "payload": payload,
                "command_timeout_ms": timeout_ms}

    ctx = zmq.Context.instance()
    sock = ctx.socket(zmq.REQ)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.LINGER, 0)
    try:
        sock.connect("tcp://127.0.0.1:5555")
        sock.send_json(envelope("session_pending", "session", "session.hello", "vsp.session.hello.v1", {
            "client_name": "B10 forensics", "client_version": "b10-probe", "protocol_min": "1.0", "protocol_max": "1.0",
            "wants": ["command.request"], "transport_bindings": ["legacy.zmq.reqrep"]}))
        session_id = json.loads(sock.recv()).get("session_id")
        sock.send_json(envelope(session_id, "command", "command.request", "vsp.command.request.v1", {
            "command": "legacy.command", "legacy": {"cmd": cmd, "args": args}}))
        reply = json.loads(sock.recv())
    finally:
        sock.close()
    return (reply.get("payload") or {}).get("legacy_reply") or {}


def raw_plugin_graph() -> dict[str, Any]:
    reply = raw_legacy_command("get_project_state", {})
    graph: dict[str, dict[str, Any]] = {}
    for track in rows(reply.get("tracks")):
        track_id = first_text(track.get("track_id"), track.get("id"))
        if not track_id:
            continue
        plugins = []
        for plugin in rows(track.get("plugins")):
            plugins.append({
                "id": first_text(plugin.get("plugin_item_id"), plugin.get("item_id"), plugin.get("plugin_id"), plugin.get("id")),
                "name": first_text(plugin.get("name"), plugin.get("plugin_name")),
                "type": first_text(plugin.get("type")),
            })
        graph[track_id] = {"name": first_text(track.get("track_name"), track.get("name")), "plugins": plugins}
    return graph


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as response:
        parsed = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return parsed


def invoke(base_url: str, tool: str, args: dict[str, Any], timeout: float, confirmed: bool = False) -> dict[str, Any]:
    envelope = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "source": "b10_static_eq_reuse_forensics.setup", "confirmed": confirmed},
        timeout,
    )
    if str(envelope.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {json.dumps(envelope, ensure_ascii=False)[:2000]}")
    result = envelope.get("result", envelope)
    if not isinstance(result, dict):
        raise RuntimeError(f"{tool} returned no object result")
    return result


def save_json(path: Path, payload: Any) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")


def first_text(*values: Any) -> str:
    for value in values:
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""


def dicts(value: Any):
    if isinstance(value, dict):
        yield value
        for item in value.values():
            yield from dicts(item)
    elif isinstance(value, list):
        for item in value:
            yield from dicts(item)


def rows(value: Any) -> list[dict[str, Any]]:
    return [item for item in (value if isinstance(value, list) else []) if isinstance(item, dict)]


def find_static_eq_loop(responses: list[dict[str, Any]]) -> dict[str, Any] | None:
    found = None
    found_updated = ""
    for response in responses:
        for item in dicts(response):
            if item.get("schema_version") == "free_state_reasoning_loop.v1" and isinstance(item.get("experiment"), dict):
                admission = item["experiment"].get("admission")
                typed = admission.get("typed_action") if isinstance(admission, dict) and isinstance(admission.get("typed_action"), dict) else {}
                if first_text(typed.get("action_domain")).lower() == "static_eq":
                    updated = first_text(item.get("updated_at"))
                    if found is None or updated >= found_updated:
                        found = item
                        found_updated = updated
    return found


def loop_intervention_receipt(loop: dict[str, Any] | None) -> dict[str, Any]:
    if loop is None:
        return {}
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    admission = experiment.get("admission") if isinstance(experiment.get("admission"), dict) else {}
    typed = admission.get("typed_action") if isinstance(admission.get("typed_action"), dict) else {}
    target = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
    for round_row in rows(experiment.get("rounds")):
        for intervention in rows(round_row.get("interventions")):
            receipt = intervention.get("receipt") if isinstance(intervention.get("receipt"), dict) else {}
            return {
                "track_id": first_text(target.get("id"), target.get("track_id")),
                "gain_db": typed.get("gain_db"),
                "frequency_hz": typed.get("frequency_hz"),
                "plugin_id": first_text(receipt.get("plugin_id")),
                "param_id": first_text(receipt.get("param_id")),
                "plugin_instantiated_by_action": receipt.get("plugin_instantiated_by_action"),
                "actual_readback_value": receipt.get("actual_readback_value"),
                "requested_target_value": receipt.get("requested_target_value"),
                "before_revision": first_text(receipt.get("before_revision")),
                "after_revision": first_text(receipt.get("after_revision"), receipt.get("applied_revision")),
            }
    return {}


def plugin_graph_diff(before: dict[str, Any], after: dict[str, Any]) -> list[str]:
    diff: list[str] = []
    for track_id in sorted(set(before) | set(after)):
        before_ids = sorted(p["id"] for p in before.get(track_id, {}).get("plugins", []))
        after_ids = sorted(p["id"] for p in after.get(track_id, {}).get("plugins", []))
        if before_ids != after_ids:
            diff.append(f"track {track_id}: {before_ids} -> {after_ids}")
    return diff


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--public-manifest", required=True)
    parser.add_argument("--public-case-id", default="spv1_p01")
    parser.add_argument("--project-workdir", required=True)
    parser.add_argument("--timeout-sec", type=float, default=600)
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--prepare-track-id", default="",
                        help="bed setup: before the reuse round, instantiate the whitelisted static_eq plugin on this "
                             "track through the same kernel command the execution port issues, establishing the "
                             "'track already carries EQ' precondition without leaving a half-settled first-experiment "
                             "conversation in the bed (its workspace recovery destroys the precondition)")
    parser.add_argument("--round1-report", default="",
                        help="alternative precondition source: an earlier applied static_eq run's d1_smoke_report.json")
    args = parser.parse_args()

    out_dir = Path(args.out_dir).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    report: dict[str, Any] = {
        "schema_version": "vit.b10_static_eq_reuse_forensics.v1",
        "started_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "agent_http": args.agent_http,
        "round1_report": args.round1_report,
        "project_workdir": str(Path(args.project_workdir).resolve()),
        "prepared_track_id": args.prepare_track_id,
    }

    track_id = ""
    existing_plugin_id = ""
    observed_param_id = ""
    if args.prepare_track_id:
        track_id = args.prepare_track_id
        # Bind the workdir project live BEFORE preparing the instance: the
        # driver then skips its reopen (live binding already matches), which
        # both preserves the prepared instance and leaves the closure state
        # fresh for the round's first conversation (a second conversation on
        # an already-experimented project finds the closure frontier consumed
        # and the admission gate rejects every proposal).
        candidates = sorted(Path(args.project_workdir).resolve().glob("*.vit"))
        require(candidates, f"workdir carries no .vit project: {args.project_workdir}")
        project_path = str(candidates[0])
        open_reply = invoke(args.agent_http, "project.open", {"file_path": project_path, "project_path": project_path}, min(args.timeout_sec, 120.0), confirmed=True)
        report["prepared_project_open"] = {"project_path": project_path, "reply_status": open_reply.get("status")}
        whitelist_path = Path.home() / ".vit" / "free_state_experiment_plugins.json"
        whitelist = json.loads(whitelist_path.read_text(encoding="utf-8"))
        plugin_path = str((whitelist.get("static_eq") or {}).get("plugin_path") or "")
        require(plugin_path, f"static_eq whitelist entry has no plugin_path: {whitelist_path}")
        instantiate_reply = raw_legacy_command("instantiate_plugin", {"track_id": track_id, "plugin_path": plugin_path})
        require(first_text(instantiate_reply.get("status")).lower() == "ok",
                "bed-setup instantiate_plugin failed: " + json.dumps(instantiate_reply, ensure_ascii=False)[:400])
        existing_plugin_id = first_text(instantiate_reply.get("plugin_id"), instantiate_reply.get("plugin_item_id"))
        require(existing_plugin_id, "bed-setup instantiate_plugin returned no plugin_id")
        report["prepared_instance"] = {
            "track_id": track_id, "plugin_id": existing_plugin_id,
            "plugin_name": first_text(instantiate_reply.get("plugin_name")),
            "note": "precondition established by the same instantiate_plugin command the StaticEQVSPPort Apply issues",
        }
        save_json(out_dir / "00_prepare_instantiate_reply.json", instantiate_reply)
    else:
        require(args.round1_report, "either --prepare-track-id or --round1-report is required")
        round1 = json.loads(Path(args.round1_report).read_text(encoding="utf-8"))
        round1_receipt = loop_intervention_receipt(find_static_eq_loop(round1.get("responses") or []))
        report["round1_receipt"] = round1_receipt
        require(round1_receipt.get("plugin_id") and round1_receipt.get("track_id"),
                "round-1 report carries no applied static_eq receipt with plugin_id/track: " + json.dumps(round1_receipt, ensure_ascii=False))
        track_id = str(round1_receipt["track_id"])
        existing_plugin_id = str(round1_receipt["plugin_id"])
        observed_param_id = str(round1_receipt.get("param_id") or "")

    # Pre-round-2 snapshot: the raw kernel plugin graph (the same surface the
    # agent's reuse discovery reads) plus the agent state for supplementary
    # evidence; the EQ instance's current parameter surface comes through the
    # governed read command.
    state_before = request_json("GET", args.agent_http.rstrip("/") + "/agent/state", None, 30.0)
    save_json(out_dir / "01_agent_state_before_round2.json", state_before)
    graph_before = raw_plugin_graph()
    save_json(out_dir / "01b_raw_plugin_graph_before_round2.json", graph_before)
    report["plugin_graph_before_round2"] = graph_before
    require(track_id in graph_before, f"target track {track_id} missing from the raw plugin graph")
    require(any(p["id"] == existing_plugin_id for p in graph_before.get(track_id, {}).get("plugins", [])),
            f"pre-existing instance {existing_plugin_id} is not on track {track_id} in the raw graph")
    eq_param_read_before = invoke(args.agent_http, "get_plugin_parameters", {
        "track_id": track_id, "plugin_id": existing_plugin_id, "include_parameters": True,
    }, min(args.timeout_sec, 60.0))
    save_json(out_dir / "02_eq_params_before_round2.json", eq_param_read_before)
    param_before = {}
    for row in rows(eq_param_read_before.get("parameters")):
        pid = first_text(row.get("param_id"), row.get("id"))
        if pid:
            param_before[pid] = {"value_text": first_text(row.get("value_text")), "normalized_value": row.get("normalized_value")}
    report["eq_param_values_before_round2"] = {
        "observed_param": observed_param_id or "round2_admission_band",
        "value_text": param_before.get(observed_param_id, {}).get("value_text", "") if observed_param_id else "",
        "param_count": len(param_before),
    }

    # Round 2: same live stack, same project, new conversation, same flavor.
    round2_report_path = out_dir / "round2_d1_smoke_report.json"
    driver = [
        sys.executable, str(Path(__file__).resolve().parent / "free_state_d1_smoke.py"),
        "--public-manifest", args.public_manifest, "--public-case-id", args.public_case_id,
        "--agent-http", args.agent_http, "--timeout-sec", str(args.timeout_sec),
        "--project-workdir", str(Path(args.project_workdir).resolve()),
        "--output", str(round2_report_path),
        "--reuse-existing-project", "--prompt-flavor", "frequency", "--expect-domain", "static_eq",
    ]
    report["round2_command"] = " ".join(driver)
    completed = subprocess.run(driver, capture_output=True, text=True, encoding="utf-8", errors="replace")
    report["round2_exit_code"] = completed.returncode
    (out_dir / "round2_driver_stdout.log").write_text(completed.stdout, encoding="utf-8")
    (out_dir / "round2_driver_stderr.log").write_text(completed.stderr, encoding="utf-8")
    if completed.returncode == NOT_EXERCISED_EXIT:
        report["status"] = "not_exercised"
        report["reason"] = "round-2 model run did not autonomously select static_eq; see round2_d1_smoke_report.json"
        report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
        save_json(out_dir / "b10_report.json", report)
        print("B10 NOT_EXERCISED: " + report["reason"])
        return NOT_EXERCISED_EXIT
    require(completed.returncode == 0,
            f"round-2 driver failed with exit {completed.returncode}; stderr tail: {completed.stderr[-2000:]}")

    round2 = json.loads(round2_report_path.read_text(encoding="utf-8"))
    round2_receipt = loop_intervention_receipt(find_static_eq_loop(round2.get("responses") or []))
    report["round2_receipt"] = round2_receipt
    require(round2_receipt.get("plugin_id"), "round-2 applied receipt carries no plugin_id")

    if str(round2_receipt.get("track_id")) != track_id:
        report["status"] = "not_exercised"
        report["reason"] = (f"round-2 selected track {round2_receipt.get('track_id')} while the treated track is {track_id}; "
                            "the reuse path was never exercised (a fresh track legitimately instantiates)")
        report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
        save_json(out_dir / "b10_report.json", report)
        print("B10 NOT_EXERCISED: " + report["reason"])
        return NOT_EXERCISED_EXIT

    state_after = request_json("GET", args.agent_http.rstrip("/") + "/agent/state", None, 30.0)
    save_json(out_dir / "03_agent_state_after_round2.json", state_after)
    graph_after = raw_plugin_graph()
    save_json(out_dir / "03b_raw_plugin_graph_after_round2.json", graph_after)
    report["plugin_graph_after_round2"] = graph_after
    graph_diff = plugin_graph_diff(graph_before, graph_after)
    report["plugin_graph_diff"] = graph_diff
    require(not graph_diff, "shadow plugin graph changed across round 2 (instance stacking): " + "; ".join(graph_diff))

    pre_ids = {p["id"] for p in graph_before.get(track_id, {}).get("plugins", [])}
    require(round2_receipt.get("plugin_instantiated_by_action") is False,
            "round-2 receipt claims it instantiated a plugin: " + json.dumps(round2_receipt, ensure_ascii=False))
    require(str(round2_receipt.get("plugin_id")) in pre_ids,
            f"round-2 wrote to plugin {round2_receipt.get('plugin_id')} which was not present before round 2 ({sorted(pre_ids)})")

    # Parameter observation (recorded, not gate): the round-2 write either
    # moved the band value off the pre-write read or rewrote the same value;
    # the receipt's readback proves the write onto the reused instance either
    # way. The before-value comes from the pre-round parameter surface of the
    # same reused instance (02 snapshot), keyed by the round-2 admission's
    # resolved band param.
    round2_param_id = str(round2_receipt.get("param_id") or "")
    before_surface = param_before.get(round2_param_id, {})
    report["parameter_observation"] = {
        "before_value_text": before_surface.get("value_text", ""),
        "before_normalized_value": before_surface.get("normalized_value"),
        "round2_requested_target_value": round2_receipt.get("requested_target_value"),
        "round2_actual_readback_value": round2_receipt.get("actual_readback_value"),
        "round2_param_id": round2_param_id,
        "reused_plugin_id": round2_receipt.get("plugin_id"),
        "note": "value comparison is observational; instance-count invariance and zero-instantiate are the hard gates",
    }

    report["status"] = "pass"
    report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
    save_json(out_dir / "b10_report.json", report)
    print(f"B10 PASS: reuse verified on track {track_id}; report={out_dir / 'b10_report.json'}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
