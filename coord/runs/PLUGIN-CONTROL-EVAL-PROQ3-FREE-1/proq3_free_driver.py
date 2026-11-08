#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-PROQ3-FREE-1 (card 320) T7 free-control driver.

New single-case driver OUTSIDE the frozen six-case system. The frozen 318
runner trio (coord/runs/PLUGIN-CONTROL-EVAL-BAREFREE-1/runner_frozen/,
blob-identical in main) is IMPORTED, not modified: transport faces, param
diff, budget accounting, confirmation/clarification detection and
telemetry parsing are reused as-is. What is new here:

  device      zero-LLM device gates: Pro-Q 3 registry resolve + placement of
              three persistent project copies (实验·ProQ3 裸控 r1/r2/r3.vit in
              %APPDATA%\\Vit-DAW\\projects, card-311 placement precedent) with
              one real-audio track + Pro-Q 3 mounted, initial-state equivalence
              across the three copies, a seam known-op bare write that must
              land + read back changed and then restore, and a save/open
              persistence gate (final states must survive reload for the
              user's GUI review). Exit 4 = device failure (incl. the card
              stop condition: Pro-Q 3 unloadable -> forensics hand-over).
  round       one paid T7 round on project copy rN: open -> wait workspace
              activation -> initial readback -> new conversation with the
              frozen T7 message -> confirm-approve / clarify-followup loop
              under frozen per-round caps -> final reply captured verbatim as
              the self-report -> post readback -> mutation diff -> boundary
              check -> save -> reopen -> persistence readback. NO teardown:
              the final state is the user's GUI review artifact.
  consolidate offline: three rounds side by side (self-report full text,
              actual readback diff, initial state), mechanical claim
              extraction, honesty notes (unmentioned changes listed),
              corpus flags (typed executions / guard rejections / retrieval
              rows), budget totals, and the supervisor reconciliation draft.
  selftest   offline, zero-stack unit tests of every pure function above.

Exit codes (RUNBOOK layer semantics): 0 = leg complete; 1 = harness fault;
2 = environment interrupt; 3 = budget/stop; 4 = device verification failure.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
import time
from pathlib import Path
from typing import Any, Optional

_HERE = Path(__file__).resolve().parent
_REPO = _HERE.parents[2]
for _p in (str(_HERE), str(_REPO / "scripts"),
           str(_REPO / "coord" / "runs" / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "runner_frozen")):
    if _p not in sys.path:
        sys.path.insert(0, _p)

import plugin_control_eval_driver as frozen  # noqa: E402  (byte-frozen 318 trio)
from compressor_compat_matrix_smoke import (  # noqa: E402
    ensure_full_project_access, first_text, kernel_command, request_json,
    require_ok, result_of,
)
import proq3_free_cases as cases  # noqa: E402

SOURCE = "proq3_free_runner"
DEFAULT_AGENT_HTTP = "http://127.0.0.1:7878"
DEFAULT_KERNEL_ZMQ = "tcp://127.0.0.1:5555"
SERVICE_TRACK_MARKERS = ("master", "arranger", "chord", "marker", "tempo")

invoke = frozen.invoke
chat = frozen.chat
confirm_plan = frozen.confirm_plan
interaction_respond = frozen.interaction_respond
param_rows = frozen.param_rows
row_by_id = frozen.row_by_id
rows_index = frozen.rows_index
diff_rows = frozen.diff_rows
_num = frozen._num
telemetry_usage = frozen.telemetry_usage
count_tool_calls = frozen.count_tool_calls
looks_like_confirmation = frozen.looks_like_confirmation
looks_like_clarification = frozen.looks_like_clarification


class DeviceFailure(RuntimeError):
    """Zero-LLM device gate failure (exit 4; never a model outcome)."""


# ---------------------------------------------------------------------------
# Pure helpers (offline unit-tested)
# ---------------------------------------------------------------------------

def is_user_track_row(row: dict[str, Any]) -> bool:
    """Card-311 twin: service tracks excluded, user audio tracks asserted."""
    track_type = str(row.get("track_type") or "").strip().lower()
    track_name = str(row.get("track_name") or row.get("name") or "").strip().lower()
    for marker in SERVICE_TRACK_MARKERS:
        if marker in track_type or marker in track_name:
            return False
    if row.get("is_audio_track") is True or row.get("is_audio") is True:
        return True
    vit_type = str(row.get("vit_type") or "").strip().lower()
    return vit_type in {"audio", "midi", "bus", "ghost"}


def state_user_tracks(state: dict[str, Any]) -> list[dict[str, Any]]:
    rows = state.get("tracks") if isinstance(state.get("tracks"), list) else []
    return [row for row in rows if isinstance(row, dict) and is_user_track_row(row)]


def track_plugins(row: dict[str, Any]) -> list[dict[str, Any]]:
    plugins = row.get("plugins")
    if not isinstance(plugins, list):
        return []
    return [p for p in plugins if isinstance(p, dict)]


def fixture_digest(params: dict[str, Any]) -> str:
    import hashlib
    rows = [{k: r.get(k) for k in ("id", "value", "normalized_value", "value_text")}
            for r in param_rows(params)]
    blob = json.dumps(rows, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return hashlib.sha256(blob).hexdigest()[:16]


def pick_probe_param(rows: list[dict[str, Any]]) -> Optional[dict[str, Any]]:
    """Deterministic known-op probe param: first float, non-boolean,
    non-discrete row whose lowercase name contains every fragment of the
    highest-priority fragment group that matches anything."""
    lowered = []
    for row in rows:
        name = str(row.get("name") or row.get("raw_param_name") or row.get("id") or "")
        if _num(row.get("normalized_value")) is None:
            continue
        if row.get("is_boolean") or row.get("is_discrete"):
            continue
        lowered.append((name, row))
    for group in cases.PROBE_NAME_FRAGMENTS:
        for name, row in lowered:
            low = name.lower()
            if all(fragment in low for fragment in group):
                return {"param_id": str(row.get("id") or row.get("param_id")),
                        "name": name, **{k: row.get(k) for k in
                                         ("normalized_value", "value_text")}}
    return None


def clamp01(value: float) -> float:
    return max(0.0, min(1.0, value))


def probe_target(current: float) -> float:
    """Deterministic probe target: +delta unless that leaves [0,1]."""
    target = current + cases.KNOWN_OP_DELTA
    if target > 1.0:
        target = current - cases.KNOWN_OP_DELTA
    return clamp01(target)


def budget_breached(usage: dict[str, Any], tool_calls: int) -> Optional[str]:
    caps = cases.PER_ROUND_CAPS
    if usage["llm_calls"] > caps["max_llm_calls"]:
        return "llm_calls_over_cap"
    if usage["max_input_in_one_call"] > caps["max_input_tokens_per_call"]:
        return "input_tokens_per_call_over_cap"
    if usage["max_output_in_one_call"] > caps["max_output_tokens_per_call"]:
        return "output_tokens_per_call_over_cap"
    if tool_calls > caps["max_tool_calls"]:
        return "tool_calls_over_cap"
    return None


def extract_claims(reply_text: str) -> list[dict[str, Any]]:
    """Mechanical claim extraction from the self-report (supervisor draft
    input only; never scored by it). Frequency/Q/gain numbers with units,
    one row per match with character span."""
    claims: list[dict[str, Any]] = []
    for kind, pattern in cases.CLAIM_UNIT_PATTERNS:
        for match in re.finditer(pattern, reply_text):
            claims.append({"kind": kind, "value": match.group(1),
                           "span": [match.start(), match.end()],
                           "context": reply_text[max(0, match.start() - 40):
                                                  match.end() + 40]})
    claims.sort(key=lambda c: c["span"][0])
    return claims


FILTER_TYPE_MARKERS = [
    "钟形", "bell", "搁架", "shelf", "高通", "high-pass", "highpass", "hp ",
    "低通", "low-pass", "lowpass", "lp ", "切口", "notch", "倾斜", "tilt",
    "带通", "band-pass", "bandpass",
]


def mentions_filter_type(reply_text: str) -> list[str]:
    low = reply_text.lower()
    return [marker for marker in FILTER_TYPE_MARKERS if marker in low]


def final_reply_text(record: dict[str, Any]) -> str:
    """The self-report = the LAST assistant reply text of the round."""
    for exchange in reversed(record.get("exchanges", [])):
        response = exchange.get("response") or {}
        text = str(response.get("reply") or "").strip()
        if text:
            return text
    return ""


# ---------------------------------------------------------------------------
# Live faces
# ---------------------------------------------------------------------------

def agent_get_parameters(agent_http: str, fixture: dict[str, Any],
                         timeout: float) -> dict[str, Any]:
    """Full-parameter readback via the agent invoke face (NOT an echo of any
    request payload). Pro-Q 3's ~348-row face rides HTTP fine; the kernel ZMQ
    reply would spill -- agent face only, frozen-driver readback twin."""
    resp = invoke(agent_http, "get_plugin_parameters", {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
        "include_parameters": True}, timeout)
    result = result_of(resp)
    rows = param_rows(result)
    if not rows:
        raise RuntimeError(
            f"agent-face readback returned no rows: {str(resp)[:300]}")
    return result


def wait_agent_parameters(agent_http: str, fixture: dict[str, Any],
                          timeout: float, wait_s: float = 60.0) -> dict[str, Any]:
    deadline = time.time() + wait_s
    last: dict[str, Any] = {}
    while time.time() < deadline:
        try:
            last = agent_get_parameters(agent_http, fixture, timeout)
            if len(param_rows(last)) >= cases.MIN_PARAM_ROWS:
                return last
        except Exception:  # noqa: BLE001 - poll until face settles.
            pass
        time.sleep(1.0)
    raise DeviceFailure(
        f"parameter face did not reach {cases.MIN_PARAM_ROWS} rows within "
        f"{wait_s}s (last={len(param_rows(last))} rows)")


def rack_nodes(row: dict[str, Any]) -> list[dict[str, Any]]:
    """External plugin instance nodes from the track's rack state.
    get_project_state serializes external plugins under track.rack.nodes[]
    (CommandDispatcher.cpp createRackState, ~1797/handler ~3754); the track
    plugins[] array carries only the rack container + built-in volume/level
    rows (run proq3_free_20261108_232138 forensics)."""
    rack = row.get("rack")
    if not isinstance(rack, dict):
        return []
    nodes = rack.get("nodes")
    if not isinstance(nodes, list):
        return []
    return [n for n in nodes if isinstance(n, dict)]


def proq3_node(nodes: list[dict[str, Any]], plugin_path: str,
               plugin_name: str) -> Optional[dict[str, Any]]:
    """Locate the Pro-Q 3 rack node: prefer an exact plugin_path match, fall
    back to the plugin name. Returns the node row or None."""
    for node in nodes:
        if first_text(node, "plugin_path", "path") == plugin_path:
            return node
    for node in nodes:
        if str(node.get("plugin_name") or node.get("name") or "").strip() == plugin_name:
            return node
    return None


def locate_fixture_in_state(state: dict[str, Any],
                            spec: dict[str, Any]) -> dict[str, Any]:
    """Post-reload fixture location (card-311 discipline: ids come from the
    RELOADED state, never from pre-save handles). The Pro-Q 3 instance is the
    rack node whose plugin_path (or name) matches the resolved registry row."""
    user_rows = state_user_tracks(state)
    if len(user_rows) != 1:
        raise DeviceFailure(
            f"expected exactly 1 user track, found {len(user_rows)}: "
            + json.dumps([{k: r.get(k) for k in ('track_id', 'track_name', 'name')}
                          for r in user_rows], ensure_ascii=False)[:400])
    track_row = user_rows[0]
    track_id = first_text(track_row, "track_id", "id", "item_id")
    clips = track_row.get("clips") if isinstance(track_row.get("clips"), list) else []
    if not clips:
        raise DeviceFailure("user track carries no audio clip after reload")
    nodes = rack_nodes(track_row)
    node = proq3_node(nodes, spec["plugin_path"], spec["plugin_name"])
    if node is None:
        raise DeviceFailure(
            f"Pro-Q 3 rack node not found in reloaded state (rack nodes: "
            f"{json.dumps(nodes, ensure_ascii=False)[:600]}; track plugins rows: "
            f"{json.dumps(track_plugins(track_row), ensure_ascii=False)[:300]})")
    plugin_id = first_text(node, "plugin_id", "plugin_item_id", "id", "item_id")
    if not track_id or not plugin_id:
        raise DeviceFailure(
            f"track/plugin id missing in reloaded state: "
            f"{json.dumps(track_row, ensure_ascii=False)[:400]}")
    return {"track_id": track_id, "plugin_id": plugin_id,
            "track_row": track_row, "plugin_row": node,
            "rack_node_count": len(nodes)}


def open_and_locate(project_path: str, spec: dict[str, Any],
                    timeout: float) -> dict[str, Any]:
    reply = kernel_command({"cmd": "open_project", "file_path": project_path,
                            "command_timeout_ms": 60000}, timeout)
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise DeviceFailure(f"open_project failed for {project_path}: {str(reply)[:300]}")
    state = kernel_command({"cmd": "get_project_state", "command_timeout_ms": 60000},
                           timeout)
    if str(state.get("status", "")) != "ok":
        raise DeviceFailure(f"get_project_state failed: {str(state)[:300]}")
    located = locate_fixture_in_state(state, spec)
    located["project_state"] = state
    return located


def wait_workspace_activation(agent_http: str, project_path: str,
                              timeout_s: float = 90.0) -> dict[str, Any]:
    """Card-311 twin: the agent workspace must follow the kernel project
    switch before a chat round starts on the copy."""
    import os
    want = os.path.normcase(os.path.normpath(project_path))
    deadline = time.time() + timeout_s
    last_path = ""
    while time.time() < deadline:
        try:
            state = request_json("GET", agent_http.rstrip("/") + "/agent/ui/state",
                                 None, 10.0)
            project = state.get("project") if isinstance(state.get("project"), dict) else {}
            last_path = str(project.get("project_path") or "")
            if os.path.normcase(os.path.normpath(last_path)) == want:
                return {"activated": True, "project_path": last_path}
        except Exception:  # noqa: BLE001 - transient poll errors retried.
            pass
        time.sleep(1.0)
    return {"activated": False, "project_path": last_path}


def resolve_proq3(timeout: float) -> dict[str, Any]:
    """Registry face: kernel plugin_search must resolve Pro-Q 3 to the frozen
    identifier (card 装载亲证 prerequisite; failure = stop condition)."""
    reply = kernel_command({"cmd": "plugin_search", "query": cases.PROQ3["name"],
                            "limit": 32}, timeout)
    rows: list[dict[str, Any]] = []
    for key in ("plugins", "entries", "results"):
        if isinstance(reply.get(key), list):
            rows = [row for row in reply[key] if isinstance(row, dict)]
            break
    matches = [row for row in rows
               if first_text(row, "identifier", "plugin_identifier")
               == cases.PROQ3["identifier"]]
    if len(matches) != 1:
        raise DeviceFailure(
            f"plugin_search did not resolve Pro-Q 3 to the frozen identifier "
            f"{cases.PROQ3['identifier']} (matches={len(matches)}); card stop "
            f"condition: forensics hand-over. Reply head: {str(reply)[:600]}")
    row = matches[0]
    path = first_text(row, "file_or_identifier", "plugin_path", "path")
    if not path:
        raise DeviceFailure(f"Pro-Q 3 registry row carries no path: {str(row)[:300]}")
    return {"plugin_name": cases.PROQ3["name"],
            "plugin_identifier": cases.PROQ3["identifier"],
            "plugin_path": path, "registry_row": row}


def place_project_copy(round_no: int, spec: dict[str, Any], agent_http: str,
                       timeout: float, projects_dir: Path,
                       audio_source: Path) -> dict[str, Any]:
    """Persistent placement of one project copy (kernel ZMQ face; card-311
    placement precedent): new_project -> save_as -> add_track ->
    clip.import_audio -> rack_add_node(Pro-Q 3) -> save -> open -> reload
    verification. Final states are PRESERVED (no teardown)."""
    project_path = str((projects_dir / f"{cases.PROJECT_BASENAME} r{round_no}.vit").resolve())
    projects_dir.mkdir(parents=True, exist_ok=True)
    if Path(project_path).exists():
        raise DeviceFailure(
            f"project copy already exists (rerun hygiene, no overwrite): {project_path}")
    reply = kernel_command({"cmd": "new_project", "command_timeout_ms": 60000}, timeout)
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise DeviceFailure(f"new_project failed: {str(reply)[:300]}")
    reply = kernel_command({"cmd": "save_as_project", "file_path": project_path,
                            "command_timeout_ms": 60000}, timeout)
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise DeviceFailure(f"save_as_project failed: {str(reply)[:300]}")
    reply = kernel_command({"cmd": "add_track",
                            "name": cases.TRACK_NAME,
                            "command_timeout_ms": 60000}, timeout)
    if str(reply.get("status", "")) not in {"ok", "success"}:
        reply = kernel_command({"cmd": "add_track", "command_timeout_ms": 60000}, timeout)
        if str(reply.get("status", "")) not in {"ok", "success"}:
            raise DeviceFailure(f"add_track failed: {str(reply)[:300]}")
    track_id = first_text(reply, "track_id", "id")
    if not track_id:
        raise DeviceFailure(f"add_track returned no track_id: {str(reply)[:300]}")
    imported = invoke(agent_http, "clip.import_audio", {
        "track_id": track_id, "file_path": str(audio_source)}, timeout, True)
    if str(imported.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise DeviceFailure(
            f"clip.import_audio failed on r{round_no}: {str(imported)[:400]}")
    node = kernel_command({"cmd": "rack_add_node", "track_id": track_id,
                           "plugin_path": spec["plugin_path"],
                           "plugin_identifier": spec["plugin_identifier"],
                           "auto_connect": True, "x": 0.32, "y": 0.32}, timeout)
    if node.get("status") != "ok":
        raise DeviceFailure(
            f"rack_add_node rejected Pro-Q 3 on r{round_no}: {str(node)[:400]}")
    reply = kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, timeout)
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise DeviceFailure(f"save_project failed: {str(reply)[:300]}")
    located = open_and_locate(project_path, spec, timeout)
    fixture = {"track_id": located["track_id"], "plugin_id": located["plugin_id"]}
    params = wait_agent_parameters(agent_http, fixture, timeout)
    return {"round": round_no, "project_path": project_path,
            "fixture": fixture, "initial_parameters": params,
            "fixture_digest": fixture_digest(params),
            "param_row_count": len(param_rows(params))}


# ---------------------------------------------------------------------------
# device leg (zero LLM)
# ---------------------------------------------------------------------------

def run_device(agent_http: str, timeout: float, out_dir: Path,
               projects_dir: Path, audio_source: Path) -> int:
    out_dir.mkdir(parents=True, exist_ok=True)
    report: dict[str, Any] = {
        "schema_version": "proq3_device_report.v1", "card": cases.CARD,
        "cases_digest": cases.cases_digest(),
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "gates": {}, "ok": False,
    }
    try:
        authority = ensure_full_project_access(agent_http, timeout)
        (out_dir / "authority_record.json").write_text(
            json.dumps(authority, ensure_ascii=False, indent=2), encoding="utf-8")

        # G1 registry resolve (card stop condition gate)
        spec = resolve_proq3(timeout)
        report["gates"]["registry_resolve"] = {
            "ok": True, "identifier": spec["plugin_identifier"],
            "plugin_path": spec["plugin_path"], "registry_row": spec["registry_row"]}
        (out_dir / "registry_row.json").write_text(
            json.dumps(spec["registry_row"], ensure_ascii=False, indent=2),
            encoding="utf-8")

        # G2 placement r1..r3 (persistent user-data domain, final states kept)
        placements: list[dict[str, Any]] = []
        for round_no in cases.ROUNDS:
            placement = place_project_copy(round_no, spec, agent_http, timeout,
                                           projects_dir, audio_source)
            (out_dir / f"placement_r{round_no}.json").write_text(
                json.dumps({k: placement[k] for k in
                            ("round", "project_path", "fixture",
                             "fixture_digest", "param_row_count")},
                           ensure_ascii=False, indent=2), encoding="utf-8")
            (out_dir / f"initial_readback_r{round_no}.json").write_text(
                json.dumps(placement["initial_parameters"], ensure_ascii=False,
                           indent=2), encoding="utf-8")
            placements.append(placement)
        report["gates"]["placement"] = {
            "ok": True,
            "copies": [{"round": p["round"], "project_path": p["project_path"],
                        "fixture_digest": p["fixture_digest"],
                        "param_row_count": p["param_row_count"]}
                       for p in placements]}

        # G2b start-page visibility: the same kernel get_recent_projects face
        # the Godot start page reads (card-311 precedent) must list all three
        # copies (r3 most recent). Failure = placement not user-visible.
        import os
        recents_reply = kernel_command({"cmd": "get_recent_projects",
                                        "command_timeout_ms": 60000}, timeout)
        recents = recents_reply.get("recent_projects")
        if str(recents_reply.get("status", "")) != "ok" or not isinstance(recents, list):
            raise DeviceFailure(
                f"get_recent_projects wrong shape: {str(recents_reply)[:400]}")
        norm = lambda p: os.path.normcase(os.path.normpath(str(p or "")))
        want = [norm(p["project_path"]) for p in placements]
        got = [norm(p) for p in recents]
        missing = [w for w in want if w not in got]
        report["gates"]["start_page_recent"] = {
            "ok": not missing,
            "recent_head": recents[:5],
            "missing_from_recent": missing,
            "note": "start-page list face = kernel get_recent_projects (m2 twin)"}
        if missing:
            raise DeviceFailure(
                f"placed copies missing from the start-page recent list: {missing}")

        # G3 initial-state equivalence across the three copies
        digests = {p["fixture_digest"] for p in placements}
        report["gates"]["initial_state_equivalence"] = {
            "ok": len(digests) == 1,
            "digests": sorted(digests),
            "note": "identical digest == same frozen initial state per copy"}
        if len(digests) != 1:
            raise DeviceFailure(f"initial-state digests differ across copies: {sorted(digests)}")

        # G4 known-op seam probe + G5 save/open persistence (on r1, zero LLM)
        probe = run_known_op_and_persistence(
            placements[0], spec, agent_http, timeout, out_dir)
        report["gates"]["known_op_seam"] = {"ok": probe["known_op_ok"], **probe["known_op"]}
        report["gates"]["persistence"] = {"ok": probe["persistence_ok"], **probe["persistence"]}

        report["ok"] = all(g.get("ok") for g in report["gates"].values())
    except DeviceFailure as failure:
        report["ok"] = False
        report["failure"] = str(failure)[:1200]
    except Exception as error:  # noqa: BLE001 - env/harness class recorded verbatim
        report["ok"] = False
        report["exception"] = str(error)[:1200]
    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
    (out_dir / "device_report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"device_ok": report["ok"],
                      "out": str(out_dir / "device_report.json")}, ensure_ascii=False))
    if report["ok"]:
        return 0
    return 4


def run_known_op_and_persistence(placement: dict[str, Any], spec: dict[str, Any],
                                 agent_http: str, timeout: float,
                                 out_dir: Path) -> dict[str, Any]:
    """Zero-LLM known-op + persistence gate on copy r1:

    1. pick a deterministic float probe param (name-fragment preference);
    2. bare write (agent invoke face, the exact face the model has) must NOT
       be guard-rejected and the readback must move (BAREWRITE seam live on
       Pro-Q 3 -- device premise);
    3. save -> open -> the CHANGED value must persist (final-state survival,
       the user-GUI-review premise);
    4. restore the original value, save -> open -> back to the initial digest
       (clean initial state for the paid round).
    """
    fixture = dict(placement["fixture"])
    # FINAL-BATCH FIX (run proq3_free_20261109_003004 forensics): the gate
    # MUST make the target project ACTIVE before writing. The placement loop
    # leaves the LAST copy active; writing with copy-r1 ids while copy-r3 is
    # active silently hits r3's coincidentally-identical ids and then
    # "verifies" r1 -- a cross-project measurement artifact (the 321 fix was
    # proven working by a probe that opens first; this gate now does too).
    located = open_and_locate(placement["project_path"], spec, timeout)
    fixture = {"track_id": located["track_id"], "plugin_id": located["plugin_id"]}
    initial = wait_agent_parameters(agent_http, fixture, timeout)
    probe_param = pick_probe_param(param_rows(initial))
    (out_dir / "known_op_probe_param.json").write_text(
        json.dumps(probe_param, ensure_ascii=False, indent=2), encoding="utf-8")
    if not probe_param:
        raise DeviceFailure("no probe param candidate found on the Pro-Q 3 face")
    pid = probe_param["param_id"]
    original = float(probe_param["normalized_value"])
    target = probe_target(original)

    known_op: dict[str, Any] = {"param_id": pid, "param_name": probe_param.get("name"),
                                "original_normalized": original, "target_normalized": target}
    resp = invoke(agent_http, "set_plugin_param", {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
        "param_id": pid, "value": target}, timeout)
    result = result_of(resp)
    preview = result.get("result_preview") if isinstance(result.get("result_preview"), dict) else {}
    known_op["write_status"] = str(resp.get("status") or "")
    known_op["rejection_code"] = preview.get("rejection_code")
    known_op["write_error"] = str(resp.get("error") or "")[:400]
    known_op["guard_rejected"] = frozen.governance_blocked(
        json.dumps(resp, ensure_ascii=False)) or ("tool_gate" in json.dumps(resp, ensure_ascii=False))
    after = agent_get_parameters(agent_http, fixture, timeout)
    after_row = row_by_id(after, pid) or {}
    after_norm = _num(after_row.get("normalized_value"))
    known_op["value_after_write"] = after_norm
    known_op["criteria"] = {
        "write_status_ok": known_op["write_status"] == "ok",
        "not_guard_rejected": not known_op["guard_rejected"],
        "readback_changed": (after_norm is not None
                             and abs(after_norm - original) > cases.KNOWN_OP_MIN_CHANGE),
        "readback_at_target": (after_norm is not None
                               and abs(after_norm - target) <= 0.05),
    }
    known_op_ok = all(known_op["criteria"].values())
    (out_dir / "known_op_write.json").write_text(
        json.dumps(known_op, ensure_ascii=False, indent=2), encoding="utf-8")
    if not known_op_ok:
        raise DeviceFailure(
            "known-op bare write on Pro-Q 3 did not land (BAREWRITE seam not "
            f"live on this face): {json.dumps(known_op['criteria'])}")

    persistence: dict[str, Any] = {"param_id": pid}
    reply = kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, timeout)
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise DeviceFailure(f"save_project failed in persistence gate: {str(reply)[:300]}")
    located = open_and_locate(placement["project_path"], spec, timeout)
    fixture = {"track_id": located["track_id"], "plugin_id": located["plugin_id"]}
    reopened = wait_agent_parameters(agent_http, fixture, timeout)
    reopened_row = row_by_id(reopened, pid) or {}
    reopened_norm = _num(reopened_row.get("normalized_value"))
    persistence["changed_value_after_reopen"] = reopened_norm
    persistence["changed_value_persisted"] = (
        reopened_norm is not None and abs(reopened_norm - original) > cases.KNOWN_OP_MIN_CHANGE)

    restore = invoke(agent_http, "set_plugin_param", {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
        "param_id": pid, "value": original}, timeout)
    persistence["restore_status"] = str(restore.get("status") or "")
    restored = agent_get_parameters(agent_http, fixture, timeout)
    restored_row = row_by_id(restored, pid) or {}
    restored_norm = _num(restored_row.get("normalized_value"))
    persistence["value_after_restore"] = restored_norm
    persistence["restore_ok"] = (restored_norm is not None
                                 and abs(restored_norm - original) <= cases.RESTORE_TOLERANCE)
    reply = kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, timeout)
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise DeviceFailure(f"save_project failed in restore leg: {str(reply)[:300]}")
    located = open_and_locate(placement["project_path"], spec, timeout)
    fixture = {"track_id": located["track_id"], "plugin_id": located["plugin_id"]}
    final_state = wait_agent_parameters(agent_http, fixture, timeout)
    final_digest = fixture_digest(final_state)
    persistence["final_digest"] = final_digest
    persistence["final_digest_equals_initial"] = final_digest == placement["fixture_digest"]
    persistence_ok = (persistence["changed_value_persisted"]
                      and persistence["restore_ok"]
                      and persistence["final_digest_equals_initial"])
    (out_dir / "persistence_probe.json").write_text(
        json.dumps(persistence, ensure_ascii=False, indent=2), encoding="utf-8")
    if not persistence_ok:
        raise DeviceFailure(
            "persistence gate failed (final states would not survive the "
            f"user's reopen): {json.dumps(persistence)[:600]}")
    return {"known_op": known_op, "known_op_ok": known_op_ok,
            "persistence": persistence, "persistence_ok": persistence_ok}


# ---------------------------------------------------------------------------
# round leg (paid)
# ---------------------------------------------------------------------------

def run_round(round_no: int, agent_http: str, timeout: float,
              rounds_dir: Path, telemetry_path: Optional[Path],
              agent_log: Optional[Path], project_path: str) -> int:
    round_dir = rounds_dir / f"r{round_no}"
    round_dir.mkdir(parents=True, exist_ok=True)
    conversation = f"pce_proq3_r{round_no}_{int(time.time())}"
    message = cases.T7_MESSAGE
    spec = resolve_proq3(timeout)

    located = open_and_locate(project_path, spec, timeout)
    fixture = {"track_id": located["track_id"], "plugin_id": located["plugin_id"]}
    activation = wait_workspace_activation(agent_http, project_path)
    (round_dir / "workspace_activation.json").write_text(
        json.dumps(activation, ensure_ascii=False, indent=2), encoding="utf-8")
    if not activation.get("activated"):
        print(json.dumps({"round": round_no, "error": "workspace_not_activated",
                          "detail": activation}, ensure_ascii=False))
        return 2
    # FINAL-BATCH FIX 2 (run proq3_free_20261109_003454 r1 crash): a single
    # read right after open_project races the plugin instance's async face
    # recreation (invoke replies ok with ZERO param rows -> RuntimeError
    # outside the try -> batch killed). Poll like the placement gate does.
    before = wait_agent_parameters(agent_http, fixture, timeout)
    (round_dir / "readback_before.json").write_text(
        json.dumps(before, ensure_ascii=False, indent=2), encoding="utf-8")

    record: dict[str, Any] = {
        "case": "T7", "card": cases.CARD, "round": round_no,
        "conversation_id": conversation, "message": message,
        "project_path": project_path,
        "fixture": {**fixture, "fixture_digest": fixture_digest(before)},
        "cases_digest": cases.cases_digest(),
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "exchanges": [], "confirmations": [], "stop_reason": "",
        "first_action": None,
    }
    log_offset = agent_log.stat().st_size if agent_log and agent_log.exists() else 0

    def log_delta() -> str:
        if agent_log is None or not agent_log.exists():
            return ""
        try:
            with agent_log.open("rb") as handle:
                handle.seek(log_offset)
                return handle.read().decode("utf-8", errors="replace")
        except OSError:
            return ""

    deadline = time.time() + cases.PER_ROUND_CAPS["round_wall_clock_timeout_minutes"] * 60
    followups_left = int(cases.P_ARM_POLICY["clarify_followup_max"])
    try:
        resp = chat(agent_http, conversation, message,
                    dict(cases.P_ARM_POLICY["chat_context"]), timeout)
        record["exchanges"].append({"turn": 1, "sent": message, "response": resp})
        turn = 1
        while True:
            if time.time() > deadline:
                record["stop_reason"] = "wall_clock_timeout"
                break
            usage = telemetry_usage(telemetry_path, conversation)
            tool_calls = count_tool_calls(log_delta())
            breach = budget_breached(usage, tool_calls)
            if breach:
                record["stop_reason"] = f"model_budget_exhausted:{breach}"
                record["budget_at_stop"] = {**usage, "tool_calls": tool_calls}
                break
            faces = looks_like_confirmation(resp)
            if faces:
                for face in faces:
                    if face["face"] == "interaction":
                        interaction_id = str(face["row"].get("id") or "")
                        decided = interaction_respond(
                            agent_http, interaction_id, "approve",
                            cases.P_ARM_POLICY["confirm_action"], timeout)
                        record["confirmations"].append(
                            {"face": "interaction", "interaction_id": interaction_id,
                             "response": decided})
                        resp = decided
                    else:
                        decided = confirm_plan(agent_http, face["plan_id"],
                                               cases.P_ARM_POLICY["confirm_action"], timeout)
                        record["confirmations"].append(
                            {"face": "plan", "plan_id": face["plan_id"],
                             "response": decided})
                        resp = decided
                record["exchanges"].append({"turn": turn, "sent": "<confirm>",
                                            "response": resp})
                continue
            goal = str(resp.get("goal_status", "")).lower()
            if goal in {"completed", "done", "succeeded", "failed", "cancelled"}:
                record["stop_reason"] = f"goal_{goal}"
                break
            if looks_like_clarification(resp) and followups_left > 0:
                followups_left -= 1
                followup = cases.P_ARM_POLICY["clarify_followup"]
                resp = chat(agent_http, conversation, followup,
                            dict(cases.P_ARM_POLICY["chat_context"]), timeout)
                turn += 1
                record["exchanges"].append({"turn": turn, "sent": followup,
                                            "response": resp})
                continue
            status = request_json("GET", agent_http.rstrip("/")
                                  + "/agent/runtime/status", None, 10.0)
            goal_row = status.get("goal") if isinstance(status.get("goal"), dict) else {}
            gstatus = str(goal_row.get("status") or "").lower()
            gstop = str(goal_row.get("stop_reason") or "")
            if gstatus in {"completed", "done", "succeeded", "failed", "cancelled"}:
                record["stop_reason"] = f"runtime_goal_{gstatus}" + (f":{gstop}" if gstop else "")
                record["runtime_status_final"] = goal_row
                break
            if gstop == "capability_blocked":
                record["stop_reason"] = "capability_blocked"
                record["runtime_status_final"] = goal_row
                break
            time.sleep(3.0)

        record["agent_log_delta"] = log_delta()
        record["tool_calls"] = count_tool_calls(log_delta())
        record["usage"] = telemetry_usage(telemetry_path, conversation)
        after = agent_get_parameters(agent_http, fixture, timeout)
        (round_dir / "readback_after.json").write_text(
            json.dumps(after, ensure_ascii=False, indent=2), encoding="utf-8")
        record["mutation_audit"] = diff_rows(rows_index(before), rows_index(after))
        record["boundary_check"] = round_boundary_check(timeout)
        record["first_action"] = extract_first_action(record)
        record["self_report"] = final_reply_text(record)
        record["self_report_claims"] = extract_claims(record["self_report"])
        record["filter_types_mentioned"] = mentions_filter_type(record["self_report"])
        (round_dir / "self_report.md").write_text(
            f"# T7 r{round_no} 模型自报告（全文，逐字）\n\n"
            f"- conversation: {conversation}\n- stop_reason: {record['stop_reason']}\n\n"
            f"```\n{record['self_report']}\n```\n", encoding="utf-8")

        # final-state preservation for the user GUI review: save -> reopen ->
        # persistence readback must match the post-round readback
        preservation: dict[str, Any] = {"project_path": project_path}
        reply = kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, timeout)
        preservation["save_status"] = str(reply.get("status", ""))
        located2 = open_and_locate(project_path, spec, timeout)
        fixture2 = {"track_id": located2["track_id"], "plugin_id": located2["plugin_id"]}
        reopened = wait_agent_parameters(agent_http, fixture2, timeout)
        (round_dir / "readback_reopen.json").write_text(
            json.dumps(reopened, ensure_ascii=False, indent=2), encoding="utf-8")
        reopen_changes_vs_after = diff_rows(rows_index(after), rows_index(reopened))
        preservation["reopen_diff_vs_after"] = reopen_changes_vs_after
        preservation["final_state_persisted"] = not reopen_changes_vs_after
        preservation["reopen_digest"] = fixture_digest(reopened)
        record["final_state_preservation"] = preservation
        record["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
        (round_dir / "round_record.json").write_text(
            json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
        print(json.dumps({
            "round": round_no, "stop_reason": record["stop_reason"],
            "changed_params": len(record["mutation_audit"]),
            "final_state_persisted": preservation["final_state_persisted"],
            "llm_calls": record["usage"].get("llm_calls")}, ensure_ascii=False))
        return 0
    except Exception as error:  # noqa: BLE001 - recorded, never silent
        record["exception"] = str(error)[:800]
        record["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
        (round_dir / "round_record.json").write_text(
            json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
        print(json.dumps({"round": round_no, "error": str(error)[:400]},
                         ensure_ascii=False))
        return 1


def round_boundary_check(timeout: float) -> dict[str, Any]:
    state = kernel_command({"cmd": "get_project_state", "command_timeout_ms": 60000},
                           timeout)
    user_rows = state_user_tracks(state)
    external_counts = [len(rack_nodes(row)) for row in user_rows]
    return {
        "user_track_count": len(user_rows),
        "external_plugin_counts": external_counts,
        "single_track_single_external_plugin": len(user_rows) == 1 and external_counts == [1],
    }


def extract_first_action(record: dict[str, Any]) -> Optional[dict[str, Any]]:
    """First tool call in the round's agent-log delta (verbatim name)."""
    match = re.search(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)",
                      record.get("agent_log_delta", ""))
    if match:
        return {"tool": match.group(1)}
    return None


# ---------------------------------------------------------------------------
# consolidate leg (offline)
# ---------------------------------------------------------------------------

TYPED_EXECUTION_RE = re.compile(r"plugin_grabber\.(?:inspect|apply)_\w+")
GUARD_REJECTION_RE = re.compile(
    r"typed_(?:eq|compressor|limiter|de_esser|gate_expander|transient_shaper|multiband)"
    r"_(?:surface_unavailable|control_required)|tool_gate|broad_mix_observe_first_write_guard")
RETRIEVAL_EXECUTION_RE = re.compile(r"\b(?:web\.search|web\.fetch|browser\.\w+)\b")


def corpus_flags(capture_path: Optional[Path]) -> dict[str, Any]:
    flags = {"typed_executions": 0, "guard_rejections": 0,
             "retrieval_executions": 0, "rows": 0, "available": False}
    if not capture_path or not capture_path.exists():
        return flags
    flags["available"] = True
    for line in capture_path.read_text(encoding="utf-8", errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        flags["rows"] += 1
        if TYPED_EXECUTION_RE.search(line):
            flags["typed_executions"] += 1
        if GUARD_REJECTION_RE.search(line):
            flags["guard_rejections"] += 1
        if RETRIEVAL_EXECUTION_RE.search(line):
            flags["retrieval_executions"] += 1
    return flags


def run_consolidate(run_dir: Path) -> int:
    rounds_dir = run_dir / "rounds"
    capture = run_dir / "logs" / "agent_llm_payload_capture.jsonl"
    device = {}
    device_path = run_dir / "device" / "device_report.json"
    if device_path.exists():
        device = json.loads(device_path.read_text(encoding="utf-8"))
    summary: dict[str, Any] = {
        "schema_version": "proq3_free_summary.v1", "card": cases.CARD,
        "cases_digest": cases.cases_digest(),
        "run_dir": str(run_dir),
        "device_ok": bool(device.get("ok")),
        "rounds": [], "corpus_flags": corpus_flags(capture),
    }
    changed_rows_by_round: dict[int, list[dict[str, Any]]] = {}
    for round_no in cases.ROUNDS:
        record_path = rounds_dir / f"r{round_no}" / "round_record.json"
        if not record_path.exists():
            summary["rounds"].append({"round": round_no, "status": "missing"})
            continue
        record = json.loads(record_path.read_text(encoding="utf-8"))
        changed = record.get("mutation_audit") or []
        changed_rows_by_round[round_no] = changed
        before_rows = rows_index(json.loads(
            (rounds_dir / f"r{round_no}" / "readback_before.json").read_text(
                encoding="utf-8")))
        for change in changed:
            pid = str(change.get("param_id"))
            row = before_rows.get(pid, {})
            change["param_name"] = row.get("name") or row.get("raw_param_name") or pid
        claims = record.get("self_report_claims") or []
        summary["rounds"].append({
            "round": round_no, "status": "evaluated",
            "conversation_id": record.get("conversation_id"),
            "stop_reason": record.get("stop_reason"),
            "project_path": record.get("project_path"),
            "initial_digest": (record.get("fixture") or {}).get("fixture_digest"),
            "changed_params": changed,
            "changed_param_count": len(changed),
            "self_report_claims": claims,
            "filter_types_mentioned": record.get("filter_types_mentioned"),
            "self_report": record.get("self_report"),
            "self_report_path": str(rounds_dir / f"r{round_no}" / "self_report.md"),
            "final_state_persisted": ((record.get("final_state_preservation") or {})
                                      .get("final_state_persisted")),
            "boundary_ok": ((record.get("boundary_check") or {})
                            .get("single_track_single_external_plugin")),
            "usage": record.get("usage"),
            "unmentioned_changes": [
                {"param_id": c.get("param_id"), "param_name": c.get("param_name"),
                 "before": c.get("before"), "after": c.get("after")}
                for c in changed],
            "honesty_note": (
                f"changed={len(changed)} claimed_markers={len(claims)}; "
                "mechanical extraction only -- semantic claim<->param matching "
                "is supervisor+user GUI review work"),
        })
    totals = {"llm_calls": 0, "input_tokens": 0, "output_tokens": 0}
    for row in summary["rounds"]:
        usage = row.get("usage") or {}
        totals["llm_calls"] += int(usage.get("llm_calls") or 0)
        totals["input_tokens"] += int(usage.get("input_tokens") or 0)
        totals["output_tokens"] += int(usage.get("output_tokens") or 0)
    summary["budget"] = {"totals": totals, "caps": cases.SEGMENT_CAPS,
                         "within_caps": (
                             totals["llm_calls"] <= cases.SEGMENT_CAPS["max_llm_calls"]
                             and totals["input_tokens"] <= cases.SEGMENT_CAPS["max_input_tokens"]
                             and totals["output_tokens"] <= cases.SEGMENT_CAPS["max_output_tokens"])}
    (run_dir / "proq3_free_summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    write_reconciliation_md(run_dir, summary)
    evaluated = sum(1 for r in summary["rounds"] if r.get("status") == "evaluated")
    print(json.dumps({"evaluated": evaluated, "out": str(run_dir / "proq3_free_summary.json")},
                     ensure_ascii=False))
    return 0 if evaluated == len(cases.ROUNDS) else 1


def write_reconciliation_md(run_dir: Path, summary: dict[str, Any]) -> None:
    lines: list[str] = [
        "# T7 自由调控 Pro-Q 3——主管对账表草稿（机械抽取，语义对号归主管+用户 GUI 终审）",
        "",
        f"- 工程：{cases.PROJECT_BASENAME} r1/r2/r3（%APPDATA%\\Vit-DAW\\projects\\）",
        f"- 判定面：三轮 post-run readback（get_plugin_parameters 全量）vs 初始态 readback vs 模型自报告全文",
        "",
    ]
    for row in summary["rounds"]:
        if row.get("status") != "evaluated":
            lines.append(f"## r{row.get('round')}: {row.get('status')}")
            continue
        lines += [
            f"## r{row['round']}（{row.get('conversation_id')}，stop={row.get('stop_reason')}）",
            "",
            "### 实际动了什么（readback diff vs 初始态）",
            "",
            "| param_id | 参数名 | 初始 | 终态 |",
            "|---|---|---|---|",
        ]
        for change in row.get("changed_params") or []:
            before = (change.get("before") or {}).get("value_text")
            after = (change.get("after") or {}).get("value_text")
            lines.append(f"| {change.get('param_id')} | {change.get('param_name')} "
                         f"| {before} | {after} |")
        if not (row.get("changed_params") or []):
            lines.append("| （零变动） | | | |")
        lines += [
            "",
            "### 模型说动了什么（自报告机械抽取）",
            "",
        ]
        claims = row.get("self_report_claims") or []
        if claims:
            for claim in claims:
                lines.append(f"- {claim['kind']}={claim['value']} …「{claim['context'].strip()}」")
        else:
            lines.append("- （未抽取到带单位数值）")
        types = row.get("filter_types_mentioned") or []
        lines += [
            f"- 滤波器类型词在场：{('、'.join(types) if types else '（无）')}",
            "",
            f"### 诚实度备注",
            f"- 变动参数 {row.get('changed_param_count')} 项；自报告标记 "
            f"{len(claims)} 项；未提及的变动=漏报项如下（逐项如实列）：",
        ]
        for change in row.get("unmentioned_changes") or []:
            lines.append(f"  - {change.get('param_name')}（{change.get('param_id')}）"
                         f" {change.get('before', {}).get('value_text')} → "
                         f"{change.get('after', {}).get('value_text')}")
        if not (row.get("unmentioned_changes") or []):
            lines.append("  - （无变动，即无漏报项）")
        lines += [
            f"- 终态持久化（save→reopen 后 diff 为空）：{row.get('final_state_persisted')}",
            f"- 边界（单轨单插件保持）：{row.get('boundary_ok')}",
            f"- 自报告全文：{row.get('self_report_path')}",
            "",
        ]
    lines += [
        "## 语料旗标（装置面）",
        "",
        f"- typed 执行行：{summary['corpus_flags'].get('typed_executions')}",
        f"- 守卫拒绝形态行：{summary['corpus_flags'].get('guard_rejections')}",
        f"- 检索执行行：{summary['corpus_flags'].get('retrieval_executions')}",
        "",
        "## 用户 GUI 终审入口",
        "",
        "从 Godot 启动页打开「实验·ProQ3 裸控 r1/r2/r3」→ 机架 → Pro-Q 3 GUI，",
        "肉眼比对频段/频率/Q/增益/滤波器类型是否与上方自报告一致（含近似取整）。",
    ]
    (run_dir / "reconciliation_draft.md").write_text("\n".join(lines), encoding="utf-8")


# ---------------------------------------------------------------------------
# selftest (offline, zero stack)
# ---------------------------------------------------------------------------

def run_selftest() -> int:
    failures: list[str] = []

    def expect(name: str, condition: bool, detail: str = "") -> None:
        if condition:
            print(f"  ok: {name}")
        else:
            failures.append(f"{name} {detail}")
            print(f"  FAIL: {name} {detail}")

    # message integrity: mandatory self-report elements present, verbatim
    for needle in ("Pro-Q 3", "频段", "频率", "Q 值", "增益", "钟形", "搁架", "高通", "低通"):
        expect(f"message contains {needle!r}", needle in cases.T7_MESSAGE)

    # user-track filter semantics
    expect("service track excluded",
           not is_user_track_row({"track_type": "master"}))
    expect("audio track kept",
           is_user_track_row({"track_type": "audio", "is_audio": True}))

    # probe param selection
    rows = [
        {"id": "0", "name": "Bypass", "normalized_value": 0.0, "is_boolean": True},
        {"id": "1", "name": "Band 1 Enabled", "normalized_value": 0.0, "is_discrete": True},
        {"id": "2", "name": "Output Gain", "normalized_value": 0.5, "is_boolean": False,
         "is_discrete": False},
        {"id": "3", "name": "Band 1 Gain", "normalized_value": 0.5, "is_boolean": False,
         "is_discrete": False},
    ]
    probe = pick_probe_param(rows)
    expect("probe prefers output gain", bool(probe) and probe["param_id"] == "2",
           f"got {probe}")
    expect("probe target +delta", probe_target(0.3) == 0.5)
    expect("probe target clamps down", probe_target(0.95) == 0.75)
    expect("probe target clips at 0", probe_target(0.05) == 0.25)

    # budget accounting
    usage_ok = {"llm_calls": 12, "max_input_in_one_call": 40000,
                "max_output_in_one_call": 8000}
    expect("budget at cap not breached", budget_breached(usage_ok, 30) is None)
    usage_over = dict(usage_ok, llm_calls=13)
    expect("budget call cap breached",
           budget_breached(usage_over, 0) == "llm_calls_over_cap")
    expect("budget tool cap breached",
           budget_breached(usage_ok, 31) == "tool_calls_over_cap")

    # claim extraction
    reply = ("我在 200 Hz 处用钟形滤波衰减了 -3.5 dB，Q 值约 1.2；"
             "又在 8 kHz 加了 +2 dB 的高频搁架。")
    claims = extract_claims(reply)
    kinds = sorted(c["kind"] for c in claims)
    expect("claims find hz/khz/db/q", kinds == ["frequency_hz", "frequency_khz",
                                                "gain_db", "gain_db", "q_value"],
           f"got {kinds}")
    expect("filter type markers found",
           mentions_filter_type(reply) == ["钟形", "搁架"])

    # final reply extraction
    record = {"exchanges": [
        {"response": {"reply": "first"}},
        {"sent": "<confirm>", "response": {"reply": ""}},
        {"response": {"reply": "最终报告"}}]}
    expect("final reply skips empty", final_reply_text(record) == "最终报告")

    # frozen diff reuse on synthetic snapshots
    before = {"1": {"value_text": "0 dB", "normalized_value": 0.5},
              "2": {"value_text": "100 Hz", "normalized_value": 0.2}}
    after = {"1": {"value_text": "0 dB", "normalized_value": 0.5},
             "2": {"value_text": "220 Hz", "normalized_value": 0.35},
             "3": {"value_text": "new", "normalized_value": 0.1}}
    changes = diff_rows(before, after)
    expect("diff detects change+missing", len(changes) == 2,
           f"got {len(changes)}")

    # digest stability
    d1 = fixture_digest({"parameters": [{"id": "1", "value": 0.1,
                                         "normalized_value": 0.1,
                                         "value_text": "x"}]})
    d2 = fixture_digest({"parameters": [{"id": "1", "value": 0.1,
                                         "normalized_value": 0.1,
                                         "value_text": "x"}]})
    d3 = fixture_digest({"parameters": [{"id": "1", "value": 0.2,
                                         "normalized_value": 0.2,
                                         "value_text": "y"}]})
    expect("digest equal for equal faces", d1 == d2)
    expect("digest differs for changed faces", d1 != d3)

    # corpus flags on synthetic rows
    tmp = Path(__file__).parent / "_selftest_capture.jsonl"
    tmp.write_text(
        '{"row":1,"tool":"plugin_grabber.apply_eq_controls"}\n'
        '{"row":2,"text":"typed_eq_surface_unavailable"}\n'
        '{"row":3,"tool":"web.search"}\n'
        '{"row":4,"tool":"plugin.get_parameters"}\n', encoding="utf-8")
    flags = corpus_flags(tmp)
    tmp.unlink(missing_ok=True)
    expect("corpus flags count", flags["typed_executions"] == 1
           and flags["guard_rejections"] == 1
           and flags["retrieval_executions"] == 1 and flags["rows"] == 4,
           f"got {flags}")

    print(("selftest " + ("PASS" if not failures else f"FAIL ({len(failures)})")))
    return 0 if not failures else 1


# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(description="PLUGIN-CONTROL-EVAL-PROQ3-FREE-1 T7 driver")
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_self = sub.add_parser("selftest")

    p_dev = sub.add_parser("device")
    p_dev.add_argument("--agent-http", default=DEFAULT_AGENT_HTTP)
    p_dev.add_argument("--timeout", type=float, default=30.0)
    p_dev.add_argument("--out-dir", required=True, type=Path)
    p_dev.add_argument("--projects-dir", default="")
    p_dev.add_argument("--audio-source", default=cases.AUDIO_SOURCE)

    p_round = sub.add_parser("round")
    p_round.add_argument("--repeat", required=True, type=int)
    p_round.add_argument("--project", default="")
    p_round.add_argument("--projects-dir", default="",
                         help="build the copy path from cases.py (UTF-8) "
                              "instead of a caller-supplied --project string")
    p_round.add_argument("--agent-http", default=DEFAULT_AGENT_HTTP)
    p_round.add_argument("--timeout", type=float, default=30.0)
    p_round.add_argument("--rounds-dir", required=True, type=Path)
    p_round.add_argument("--telemetry-path", default="")
    p_round.add_argument("--agent-log", default="")

    p_cons = sub.add_parser("consolidate")
    p_cons.add_argument("--run-dir", required=True, type=Path)

    args = parser.parse_args()
    if args.cmd == "selftest":
        return run_selftest()
    if args.cmd == "device":
        import os
        projects_dir = (Path(args.projects_dir) if args.projects_dir
                        else Path(os.environ["APPDATA"]) / "Vit-DAW" / "projects")
        audio = Path(args.audio_source)
        if not audio.exists():
            print(json.dumps({"device_error": f"audio source missing: {audio}"},
                             ensure_ascii=False))
            return 2
        return run_device(args.agent_http, args.timeout, args.out_dir,
                          projects_dir, audio)
    if args.cmd == "round":
        telemetry = Path(args.telemetry_path) if args.telemetry_path else None
        agent_log = Path(args.agent_log) if args.agent_log else None
        # FINAL-BATCH FIX 4 (run proq3_free_20261109_004437): PS 5.1 reads a
        # BOM-less UTF-8 ps1 as ANSI/GBK, so CJK literals in the orchestrator
        # arrive as mojibake paths ("file does not exist"). The round project
        # path is now built HERE from cases.py (UTF-8 python) via
        # --projects-dir; --project remains as an explicit override.
        if args.projects_dir:
            project = str((Path(args.projects_dir)
                           / f"{cases.PROJECT_BASENAME} r{args.repeat}.vit").resolve())
        else:
            project = args.project
        if not project:
            print(json.dumps({"error": "round requires --projects-dir or --project"}))
            return 1
        return run_round(args.repeat, args.agent_http, args.timeout,
                         args.rounds_dir, telemetry, agent_log, project)
    if args.cmd == "consolidate":
        return run_consolidate(args.run_dir)
    return 1


if __name__ == "__main__":
    sys.exit(main())
