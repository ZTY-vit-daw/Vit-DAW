#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-RUNNER-1 experiment driver (card 300).

Subcommands (all share the frozen case module plugin_control_eval_cases):

  selftest   offline, zero-stack unit tests of every pure function that the
             S0 / P legs rely on (display parsing, tolerance math, row-wise
             non-target diff, evaluator verdicts on synthetic snapshots,
             budget accounting, message/case integrity). Exit 0 = pass.
  s0         zero-LLM known-operation verification on the live stack: per
             case load fixture -> verify frozen initial state -> scripted
             correct operation -> independent readback -> non-target diff ->
             restore -> readback -> teardown. Freezes the T4 allowance set
             and writes per-case exit statuses. Fixture/interface failures
             are reported as such (never model failures).
  dryround   one scripted round through the FULL pipeline (fixture -> round
             bookkeeping -> scripted known-op instead of /agent/chat ->
             independent evaluator -> artifacts) with LLM never invoked.
             Validates the pipeline end-to-end inside a stack window.
  round      one real model round (arm P: product /agent/chat path) with
             frozen task message, confirmation policy, budget counters and
             wall-clock cap; evaluator verdict written per round.
  segment-p  the six P rounds in frozen case order with segment budget
             accounting and a summary table.
  evaluate   re-run the independent evaluator from an existing round's
             artifacts (no stack needed beyond readback inputs already
             captured; --live flag re-reads the stack instead).

Interface faces (precedents: pgtdc_compat_matrix.py + b3_pan_layout_agent_
smoke.py + automation_llm_route_smoke.py):
  agent HTTP  /agent/invoke /agent/chat /agent/confirm
              /agent/interaction/respond /agent/authority /agent/state
              /agent/runtime/status
  kernel ZMQ  plugin_search / rack_add_node / get_plugin_parameters /
              track.add_audio / track.delete (probe face, pgtdc precedent)

Exit codes (RUNBOOK section 4 layer semantics):
  s0:        0 = all six cases verified; 1 = any case failed (forensics
             artifacts written; no model rounds may start); 2 = environment
             interrupt (stack/transport; raw evidence retained).
  round:     0 = evaluation complete (verdict exists, including failure
             verdicts); 1 = harness/stack fault (harness_invalid, never a
             model score); 2 = environment interrupt evidence.
  segment-p: 0 = all planned rounds evaluated; 3 = budget/stop triggered
             with rounds remaining (reported as not-run).
"""
from __future__ import annotations

import argparse
import json
import re
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, Optional

sys.path.insert(0, str(Path(__file__).resolve().parent))

from plugin_control_eval_cases import (  # noqa: E402
    CASE_ORDER,
    EVAL_RULES,
    FAILURE_TAXONOMY,
    INITIAL_STATE_NEEDLES,
    KNOWN_OP,
    P_ARM_POLICY,
    PER_ROUND_CAPS,
    PLUGINS,
    ROUND_MESSAGES,
    SEGMENT_P_CAPS,
    T6_LIMITATION_MARKERS,
    CARD,
    SCHEMA_VERSION,
    case_family,
    case_plugin,
    cases_digest,
)
from compressor_compat_matrix_smoke import (  # noqa: E402
    ensure_full_project_access,
    first_text,
    kernel_command,
    request_json,
    require_ok,
    result_of,
)

SOURCE = "plugin_control_eval_runner"
DEFAULT_AGENT_HTTP = "http://127.0.0.1:7878"
DEFAULT_KERNEL_ZMQ = "tcp://127.0.0.1:5555"


# ---------------------------------------------------------------------------
# Transport helpers
# ---------------------------------------------------------------------------

def invoke(base: str, tool: str, args: dict[str, Any], timeout: float,
           confirmed: bool = False) -> dict[str, Any]:
    return request_json("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool, "args": args, "confirmed": confirmed, "source": SOURCE,
    }, timeout)


def chat(base: str, conversation_id: str, message: str, context: dict[str, Any],
         timeout: float) -> dict[str, Any]:
    return request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id, "message": message,
        "context": context,
    }, timeout)


def confirm_plan(base: str, plan_id: str, decision: str, timeout: float) -> dict[str, Any]:
    return request_json("POST", base.rstrip("/") + "/agent/confirm", {
        "plan_id": plan_id, "decision": decision,
    }, timeout)


def interaction_respond(base: str, interaction_id: str, action_id: str,
                        decision: str, timeout: float) -> dict[str, Any]:
    return request_json("POST", base.rstrip("/") + "/agent/interaction/respond", {
        "interaction_id": interaction_id, "action_id": action_id,
        "decision": decision, "payload": {},
    }, timeout)


def wait_agent(base: str, timeout: float) -> dict[str, Any]:
    deadline = time.time() + timeout
    last_error: Optional[str] = None
    while time.time() < deadline:
        try:
            state = request_json("GET", base.rstrip("/") + "/agent/state", None, 5.0)
            if str(state.get("status", "")).lower() == "ok":
                return state
        except (OSError, ValueError) as error:
            last_error = str(error)
        time.sleep(2.0)
    raise RuntimeError(f"agent not ready at {base}: {last_error}")


# ---------------------------------------------------------------------------
# Parameter face helpers (pure functions; unit-tested offline)
# ---------------------------------------------------------------------------

def param_rows(payload: dict[str, Any]) -> list[dict[str, Any]]:
    rows = payload.get("parameters") if isinstance(payload, dict) else None
    return [row for row in (rows or []) if isinstance(row, dict)]


def row_by_id(payload: dict[str, Any], param_id: str) -> dict[str, Any]:
    for row in param_rows(payload):
        if str(row.get("id") or row.get("param_id")) == str(param_id):
            return row
    return {}


def display_number(value_text: Any) -> Optional[float]:
    """Leading numeric value of a display string ('375 ms' -> 375.0,
    '70.71%' -> 70.71, '1/8D' -> None (fraction first char not numeric))."""
    if isinstance(value_text, (int, float)):
        return float(value_text)
    if not isinstance(value_text, str):
        return None
    match = re.match(r"\s*(-?\d+(?:\.\d+)?)(?![/\d])", value_text)
    return float(match.group(1)) if match else None


def discrete_label(row: dict[str, Any]) -> str:
    text = str(row.get("value_text") or "").strip()
    return text


def rows_index(payload: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {str(row.get("id") or row.get("param_id")): row
            for row in param_rows(payload)}


def diff_rows(before: dict[str, dict[str, Any]], after: dict[str, dict[str, Any]],
              tolerance: float = 1e-4) -> list[dict[str, Any]]:
    """Row-wise diff over the union of param ids: any change in value /
    normalized_value / value_text outside float tolerance is one change row."""
    changes: list[dict[str, Any]] = []
    for pid in sorted(set(before) | set(after)):
        b, a = before.get(pid, {}), after.get(pid, {})
        if not b or not a:
            changes.append({"param_id": pid, "kind": "row_missing",
                            "before_present": bool(b), "after_present": bool(a)})
            continue
        b_norm, a_norm = b.get("normalized_value"), a.get("normalized_value")
        norm_changed = not (_num(b_norm) is not None and _num(a_norm) is not None
                            and abs(_num(b_norm) - _num(a_norm)) <= tolerance)
        text_changed = str(b.get("value_text")) != str(a.get("value_text"))
        val_changed = norm_changed  # value mirrors normalized on these faces
        if norm_changed or text_changed:
            changes.append({
                "param_id": pid, "kind": "value_changed" if val_changed else "text_changed",
                "before": {"value_text": b.get("value_text"),
                           "normalized_value": b.get("normalized_value")},
                "after": {"value_text": a.get("value_text"),
                          "normalized_value": a.get("normalized_value")},
            })
    return changes


def _num(value: Any) -> Optional[float]:
    try:
        if isinstance(value, bool):
            return None
        return float(value)
    except (TypeError, ValueError):
        return None


def changes_outside(changes: list[dict[str, Any]],
                    allowed: set[str]) -> list[dict[str, Any]]:
    return [c for c in changes if str(c.get("param_id")) not in allowed]


# ---------------------------------------------------------------------------
# Fixture builder (per-case independent working copy on the live stack)
# ---------------------------------------------------------------------------

class FixtureError(RuntimeError):
    """Fixture/interface failure (never a model failure)."""


def resolve_plugin(case_key: str, timeout: float) -> dict[str, Any]:
    entry = PLUGINS[case_key]
    reply = kernel_command({"cmd": "plugin_search", "query": entry["name"],
                            "limit": 32}, timeout)
    rows: list[dict[str, Any]] = []
    for key in ("plugins", "entries", "results"):
        if isinstance(reply.get(key), list):
            rows = [row for row in reply[key] if isinstance(row, dict)]
            break
    matches = [row for row in rows
               if first_text(row, "identifier", "plugin_identifier") == entry["identifier"]]
    if len(matches) != 1:
        raise FixtureError(
            f"plugin_search did not resolve {entry['name']!r} to the frozen "
            f"identifier {entry['identifier']} (matches={len(matches)})")
    row = matches[0]
    return {
        "plugin_name": entry["name"],
        "plugin_identifier": entry["identifier"],
        "plugin_path": first_text(row, "file_or_identifier", "plugin_path", "path"),
        "artifact": entry["artifact"],
    }


def build_fixture(case_id: str, agent_http: str, timeout: float,
                  track_tag: str) -> dict[str, Any]:
    """Fresh disposable track + plugin instance (kernel probe load face per
    the pgtdc/M2-TIMEFX-2 precedent -- the agent plugin.load_to_rack tool is
    PCA-gated and an unattended probe must not forge it). Verifies the frozen
    initial state before returning. Raises FixtureError on any mismatch."""
    case_key = case_plugin(case_id)
    spec = resolve_plugin(case_key, timeout)
    track = require_ok(invoke(agent_http, "track.add_audio", {
        "name": f"PCE {track_tag} {spec['plugin_name']}"}, timeout, True), "track.add_audio")
    track_id = first_text(track, "track_id", "id")
    if not track_id:
        raise FixtureError("track.add_audio returned no track id")
    try:
        node = kernel_command({"cmd": "rack_add_node", "track_id": track_id,
                               "plugin_path": spec["plugin_path"],
                               "plugin_identifier": spec["plugin_identifier"],
                               "auto_connect": True, "x": 0.32, "y": 0.32}, timeout)
        if node.get("status") != "ok":
            raise FixtureError(f"rack_add_node rejected: {str(node)[:400]}")
        plugin_id = first_text(node, "plugin_id", "plugin_item_id", "id")
        if not plugin_id:
            raise FixtureError(f"rack_add_node returned no plugin id: {str(node)[:300]}")
    except FixtureError:
        _safe_delete_track(agent_http, track_id, timeout)
        raise
    params = wait_parameters(timeout, track_id, plugin_id)
    if not (params.get("status") == "ok" and params.get("parameters")):
        _safe_delete_track(agent_http, track_id, timeout)
        raise FixtureError(f"parameters unavailable after load: {str(params)[:300]}")
    # frozen initial state verification
    for pid, needle in INITIAL_STATE_NEEDLES.get(case_id, {}).items():
        row = row_by_id(params, pid)
        if not row:
            _safe_delete_track(agent_http, track_id, timeout)
            raise FixtureError(f"{case_id}: expected param {pid} absent from face")
        text = str(row.get("value_text") or "")
        if str(needle["value_text_needle"]).strip() not in text:
            _safe_delete_track(agent_http, track_id, timeout)
            raise FixtureError(
                f"{case_id}: param {pid} initial display {text!r} does not contain "
                f"frozen needle {needle['value_text_needle']!r}")
        norm = _num(row.get("normalized_value"))
        want = float(needle["normalized_needle"])
        if norm is None or abs(norm - want) > 1e-3:
            _safe_delete_track(agent_http, track_id, timeout)
            raise FixtureError(
                f"{case_id}: param {pid} initial normalized {norm} != frozen {want}")
    return {
        "case_id": case_id, "track_id": track_id, "plugin_id": plugin_id,
        "plugin": spec, "initial_parameters": params,
        "fixture_digest": _fixture_digest(params),
    }


def wait_parameters(timeout: float, track_id: str, plugin_id: str) -> dict[str, Any]:
    deadline = time.time() + 60.0
    params: dict[str, Any] = {}
    while time.time() < deadline:
        params = kernel_command({"cmd": "get_plugin_parameters", "track_id": track_id,
                                 "plugin_id": plugin_id, "include_parameters": True}, timeout)
        if params.get("status") == "ok" and param_rows(params):
            return params
        time.sleep(0.8)
    return params


def _fixture_digest(params: dict[str, Any]) -> str:
    import hashlib
    rows = [{k: row.get(k) for k in ("id", "value", "normalized_value", "value_text")}
            for row in param_rows(params)]
    blob = json.dumps(rows, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return hashlib.sha256(blob).hexdigest()[:16]


def _safe_delete_track(agent_http: str, track_id: str, timeout: float) -> None:
    try:
        require_ok(invoke(agent_http, "track.delete", {"track_id": track_id},
                          timeout, True), "track.delete")
    except Exception:
        pass


def teardown_fixture(agent_http: str, fixture: dict[str, Any], timeout: float) -> None:
    if fixture.get("track_id"):
        _safe_delete_track(agent_http, fixture["track_id"], timeout)


def readback_parameters(fixture: dict[str, Any], timeout: float,
                        agent_http: str) -> dict[str, Any]:
    """Independent readback: re-query the live instance via the agent invoke
    face (NOT an echo of any request payload)."""
    resp = invoke(agent_http, "get_plugin_parameters", {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
        "include_parameters": True}, timeout)
    result = result_of(resp)
    rows = param_rows(result)
    if not rows:
        raise FixtureError(f"agent-face readback returned no rows: {str(resp)[:300]}")
    return result


_GOVERNANCE_GATE_RE = re.compile(
    r"typed_(?:eq|compressor|limiter|de_esser|gate_expander|transient_shaper|multiband)"
    r"_(?:surface_unavailable|control_required)")


def governance_blocked(text: str) -> bool:
    """True when a set_plugin_param rejection is a typed-topology governance
    gate (goalrunner_chat.go guard*OwnedGenericParameterWrite family). These
    rejections mean the product prescribes the typed control face for this
    plugin -- the known-op routes there instead."""
    return bool(_GOVERNANCE_GATE_RE.search(text))


def resolve_discrete_label(before_rows: dict[str, dict[str, Any]], pid: str,
                           normalized: float) -> Optional[str]:
    row = before_rows.get(pid, {})
    labels = (row.get("display_probe") or {}).get("discrete_labels") or []
    fallback: Optional[str] = None
    for label_row in labels:
        if not isinstance(label_row, dict):
            continue
        value = _num(label_row.get("value"))
        if value is None:
            continue
        if abs(value - normalized) <= 1e-6:
            return str(label_row.get("label"))
        if abs(value - normalized) <= 0.051 and fallback is None:
            fallback = str(label_row.get("label"))
    return fallback


def typed_apply_write(agent_http: str, fixture: dict[str, Any], family: str,
                      pid: str, label: Optional[str], display: Optional[float],
                      timeout: float, ms: Optional[float] = None) -> dict[str, Any]:
    """Typed-apply fallback for plugins whose generic set_plugin_param is
    governance-blocked (guard*OwnedGenericParameterWrite: the product
    prescribes the typed control face when a typed topology is unresolved --
    e.g. Abbey Road Plates' Damper, bound as the reverb tail decay control,
    pgtdc spot precedent apply_status=exact). Control field is unit-aware:
    the binding's disclosed physical unit selects value_ms / value_s /
    percent / display_value (display_value alone was proven ambiguous in S0
    run3: 250 in an ms binding landed at normalized 0.352 = 352 ms)."""
    inspect_tool = (f"plugin_grabber.inspect_{family}"
                    if family in {"reverb", "delay"} else None)
    apply_tool = (f"plugin_grabber.apply_{family}_controls"
                  if family in {"reverb", "delay"} else None)
    if not inspect_tool or not apply_tool:
        raise FixtureError(f"no typed apply face for family {family!r}")
    insp = require_ok(invoke(agent_http, inspect_tool, {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"]},
        timeout), inspect_tool)
    stage = insp.get(f"{family}_stage") if isinstance(insp.get(f"{family}_stage"), dict) else {}
    binding: dict[str, Any] = {}
    for section_rows in stage.values():
        if not isinstance(section_rows, list):
            continue
        for row in section_rows:
            if isinstance(row, dict) and str(row.get("param_id")) == str(pid):
                binding = row
                break
        if binding:
            break
    if not binding or not str(binding.get("control_ref") or ""):
        raise FixtureError(f"typed inspect {family} has no binding for param {pid}")
    unit = ""
    domain = binding.get("domain")
    if isinstance(domain, dict):
        unit = str(domain.get("unit") or "").strip().lower()
    elif isinstance(domain, str):
        # the disclosed domain may be a bare unit string ("enum" on the
        # plates damper binding in the pgtdc spot artifact; "ms" family on
        # delay time bindings) -- treating it as non-dict dropped the unit
        # and produced the run3/run4 display_value misreads
        unit = domain.strip().lower()
    if not unit:
        unit = str(binding.get("physical_unit") or "").strip().lower()
    control: dict[str, Any] = {"control_ref": binding.get("control_ref")}
    if label is not None:
        control["enum_label"] = label
    elif ms is not None:
        if unit in {"s", "sec", "second", "seconds"}:
            control["value_s"] = ms / 1000.0
        else:
            control["value_ms"] = ms
    elif display is not None:
        if unit in {"s", "sec", "second", "seconds"}:
            control["value_s"] = display
        elif unit in {"ms", "millisecond", "milliseconds"}:
            control["value_ms"] = display
        else:
            control["display_value"] = display
    else:
        raise FixtureError(f"typed apply needs a label, ms or display value for {pid}")
    applied = require_ok(invoke(agent_http, apply_tool, {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
        "controls": [control]}, timeout), apply_tool)
    if str(applied.get("status")) not in {"exact", "quantized"}:
        raise FixtureError(f"typed apply status={applied.get('status')!r} "
                           f"(control={control})")
    return {"face": apply_tool, "control": control, "binding_unit": unit,
            "apply_status": applied.get("status")}


def write_param(agent_http: str, fixture: dict[str, Any], pid: str,
                normalized: Optional[float], timeout: float,
                before_rows: Optional[dict[str, dict[str, Any]]] = None,
                family: Optional[str] = None,
                target_display: Optional[float] = None,
                target_ms: Optional[float] = None) -> dict[str, Any]:
    """Deterministic write on the agent tool faces the model itself has:
    generic set_plugin_param first; when product governance blocks the
    generic face (typed topology gates), route through the typed apply face
    with the equivalent display-domain operation. Discrete targets resolve
    their step label from the disclosed label table; continuous targets use
    the caller-provided ms/display target (nonlinear domains cannot be
    derived from normalized alone)."""
    resp = invoke(agent_http, "set_plugin_param", {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
        "param_id": pid, "value": float(normalized)}, timeout)
    if str(resp.get("status", "")).lower() not in {"ok", "success", "completed"}:
        blob = json.dumps(resp, ensure_ascii=False)
        if governance_blocked(blob):
            label: Optional[str] = None
            if before_rows is not None:
                label = resolve_discrete_label(before_rows, pid, float(normalized))
            display = target_display
            ms = target_ms
            if label is None and ms is None and display is None:
                before_row = (before_rows or {}).get(pid, {})
                text = str(before_row.get("value_text") or "")
                display = display_number(text)
                if text.strip().lower().endswith("ms"):
                    ms = display
            return typed_apply_write(agent_http, fixture, family or
                                     case_family(fixture["case_id"]), pid,
                                     label, display, timeout, ms=ms)
        require_ok(resp, "set_plugin_param")
        return {"face": "set_plugin_param"}
    return {"face": "set_plugin_param"}


def kernel_face_write(fixture: dict[str, Any], pid: str, normalized: float,
                      timeout: float) -> dict[str, Any]:
    """Kernel ZMQ normalized write (staticeq_vsp.go legacy shape). Used ONLY
    as the S0 known-op reachability leg for cases whose agent write faces are
    governance-blocked AND misconvert (T4 forensics in the row): the kernel
    face proves the target is physically reachable on the live instance; the
    agent-face behaviors are recorded verbatim for the supervisor."""
    reply = kernel_command({"cmd": "set_plugin_param",
                            "track_id": fixture["track_id"],
                            "plugin_id": fixture["plugin_id"],
                            "param_id": pid, "value": float(normalized)}, timeout)
    if reply.get("status") != "ok":
        raise FixtureError(f"kernel set_plugin_param rejected: {str(reply)[:400]}")
    return {"face": "kernel_zmq.set_plugin_param", "normalized": float(normalized)}


def agent_face_forensics(agent_http: str, fixture: dict[str, Any], pid: str,
                         normalized: float, timeout: float,
                         before_rows: dict[str, dict[str, Any]],
                         target_ms: Optional[float]) -> dict[str, Any]:
    """Record both agent write faces' behavior verbatim (generic gate +
    typed apply with its own claimed readback) without asserting anything."""
    record: dict[str, Any] = {"param_id": pid}
    generic = invoke(agent_http, "set_plugin_param", {
        "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
        "param_id": pid, "value": float(normalized)}, timeout)
    record["generic_response"] = {
        "status": generic.get("status"),
        "error": str(generic.get("error") or "")[:500],
        "rejection_code": (result_of(generic).get("rejection_code")
                           if isinstance(result_of(generic), dict) else None),
    }
    try:
        typed = typed_apply_write(agent_http, fixture, case_family(fixture["case_id"]),
                                  pid, None, None, timeout, ms=target_ms)
        record["typed_apply"] = typed
    except FixtureError as error:
        record["typed_apply_error"] = str(error)[:500]
    return record


# ---------------------------------------------------------------------------
# S0 known-operation verifier
# ---------------------------------------------------------------------------

def run_s0(agent_http: str, timeout: float, out_dir: Path) -> int:
    authority = ensure_full_project_access(agent_http, timeout)
    (out_dir / "authority_record.json").write_text(
        json.dumps(authority, ensure_ascii=False, indent=2), encoding="utf-8")
    report: dict[str, Any] = {
        "schema_version": "plugin_control_eval_s0_report.v1",
        "card": CARD, "cases_digest": cases_digest(),
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "cases": [], "allowance_set": {}, "freeze_version": "s0",
    }
    exit_code = 0
    for case_id in CASE_ORDER:
        print(f"[s0] {case_id} ({case_family(case_id)}) ...", flush=True)
        try:
            row = s0_case(case_id, agent_http, timeout, out_dir, report)
        except CaseFailure as error:
            row = dict(getattr(error, "partial_row", {}) or {})
            row.update({"case_id": case_id, "status": "fixture_failure",
                        "error": str(error)})
            report["cases"].append(row)
            print(f"  FIXTURE FAILURE: {error}", flush=True)
            exit_code = 1
            continue
        except FixtureError as error:
            row = {"case_id": case_id, "status": "fixture_failure", "error": str(error)}
            report["cases"].append(row)
            print(f"  FIXTURE FAILURE: {error}", flush=True)
            exit_code = 1
            continue
        except RuntimeError as error:
            # tool-level rejection (not transport): an interface finding of
            # the write/read faces -- fixture/interface family per plan s0,
            # never an environment interrupt and never a model failure
            row = {"case_id": case_id, "status": "fixture_failure",
                   "error": f"interface: {error}"}
            report["cases"].append(row)
            print(f"  FIXTURE/INTERFACE FAILURE: {error}", flush=True)
            exit_code = 1
            continue
        except (OSError, ValueError) as error:
            row = {"case_id": case_id, "status": "environment_interrupt",
                   "error": str(error)}
            report["cases"].append(row)
            print(f"  ENV INTERRUPT: {error}", flush=True)
            exit_code = 2
            break
        report["cases"].append(row)
        status = row.get("status")
        print(f"  status={status}", flush=True)
        if status != "verified":
            exit_code = 1 if status == "fixture_failure" else exit_code
    report["all_verified"] = all(c.get("status") == "verified" for c in report["cases"]) \
        and len(report["cases"]) == len(CASE_ORDER)
    (out_dir / "s0_report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    # case freeze artifact (hash-bound)
    freeze = {
        "schema_version": "plugin_control_eval_case_freeze.v1",
        "cases_digest": cases_digest(),
        "s0_all_verified": report["all_verified"],
        "allowance_set": report["allowance_set"],
        "known_op_expectations_observed": {
            c["case_id"]: c.get("observed") for c in report["cases"]},
    }
    (out_dir / "case_freeze.json").write_text(
        json.dumps(freeze, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"[s0] report: {out_dir / 's0_report.json'}", flush=True)
    return exit_code


class CaseFailure(FixtureError):
    """FixtureError carrying the partial case row (write faces, snapshots)
    so the report keeps the evidence even when an assertion fails."""

    def __init__(self, message: str, partial_row: Optional[dict[str, Any]] = None):
        super().__init__(message)
        self.partial_row = partial_row or {}


def s0_case(case_id: str, agent_http: str, timeout: float, out_dir: Path,
            report: dict[str, Any]) -> dict[str, Any]:
    spec = KNOWN_OP[case_id]
    fixture = build_fixture(case_id, agent_http, timeout, f"s0_{case_id}")
    row: dict[str, Any] = {"case_id": case_id,
                           "fixture": {k: fixture[k] for k in ("track_id", "plugin_id")},
                           "fixture_digest": fixture["fixture_digest"]}
    try:
        try:
            return _s0_case_body(case_id, spec, fixture, row, agent_http,
                                 timeout, out_dir, report)
        except FixtureError as error:
            raise CaseFailure(str(error), row) from error
    finally:
        teardown_fixture(agent_http, fixture, timeout)


def _s0_case_body(case_id: str, spec: dict[str, Any], fixture: dict[str, Any],
                  row: dict[str, Any], agent_http: str, timeout: float,
                  out_dir: Path, report: dict[str, Any]) -> dict[str, Any]:
    if True:  # keep the original indentation block
        before = readback_parameters(fixture, timeout, agent_http)
        (out_dir / f"{case_id}_before.json").write_text(
            json.dumps(before, ensure_ascii=False, indent=2), encoding="utf-8")

        # T6: unsatisfiability re-confirmation (face enumeration, zero writes)
        if spec.get("unsatisfiable"):
            rows = param_rows(before)
            forbidden = [r for r in rows
                         if any(f in str(r.get("name", "")).lower()
                                for f in spec["unsatisfiable"]["forbidden_name_fragments"])]
            row["unsatisfiable_check"] = {
                "param_count": len(rows),
                "names": [str(r.get("name")) for r in rows],
                "forbidden_fragment_hits": [str(r.get("name")) for r in forbidden],
            }
            if forbidden or len(rows) > spec["unsatisfiable"]["face_param_count_max"]:
                raise FixtureError(
                    f"{case_id}: unsatisfiability premise contradicted: {row['unsatisfiable_check']}")
            after = readback_parameters(fixture, timeout, agent_http)
            changes = diff_rows(rows_index(before), rows_index(after))
            row["zero_mutation_verified"] = not changes
            row["observed"] = {"param_count": len(rows)}
            if changes:
                raise FixtureError(f"{case_id}: spontaneous drift with zero writes: {changes}")
            row["status"] = "verified"
            return row

        # scripted correct operation (deterministic writes; the write faces
        # are recorded BEFORE expectations so a failed expectation still
        # carries the face evidence)
        write_faces: list[dict[str, Any]] = []
        for write in spec["writes"]:
            if write.get("kernel_face"):
                # agent faces recorded verbatim (gate + typed apply), then
                # the kernel normalized write proves physical reachability
                forensics = agent_face_forensics(
                    agent_http, fixture, write["param_id"], write["normalized"],
                    timeout, rows_index(before), write.get("ms"))
                face = kernel_face_write(fixture, write["param_id"],
                                         write["normalized"], timeout)
                write_faces.append({"param_id": write["param_id"],
                                    "agent_face_forensics": forensics, **face})
                continue
            face = write_param(agent_http, fixture, write["param_id"],
                               write["normalized"], timeout,
                               before_rows=rows_index(before),
                               target_display=write.get("display"),
                               target_ms=write.get("ms"))
            write_faces.append({"param_id": write["param_id"], **face})
        row["write_faces"] = write_faces
        after = readback_parameters(fixture, timeout, agent_http)
        (out_dir / f"{case_id}_after.json").write_text(
            json.dumps(after, ensure_ascii=False, indent=2), encoding="utf-8")
        for expectation in spec["expect"]:
            pid = expectation["param_id"]
            target = row_by_id(after, pid)
            if not target:
                raise FixtureError(f"{case_id}: param {pid} missing after write")
            if "label_needle" in expectation:
                label = discrete_label(target)
                if expectation["label_needle"] not in label:
                    raise FixtureError(
                        f"{case_id}: param {pid} label {label!r} lacks needle "
                        f"{expectation['label_needle']!r}")
            if "display_contains" in expectation:
                text = str(target.get("value_text") or "")
                if str(expectation["display_contains"]) not in text:
                    raise FixtureError(
                        f"{case_id}: param {pid} display {text!r} lacks needle "
                        f"{expectation['display_contains']!r}")
            if "normalized_needle" in expectation:
                norm = _num(target.get("normalized_value"))
                want = float(expectation["normalized_needle"])
                if norm is None or abs(norm - want) > 5e-3:
                    raise FixtureError(
                        f"{case_id}: param {pid} normalized {norm} != {want}")
        changes_after_write = diff_rows(rows_index(before), rows_index(after))
        written_ids = {w["param_id"] for w in spec["writes"]}
        side = [c for c in changes_after_write if str(c["param_id"]) not in written_ids]
        row["side_changes_on_write"] = side
        if case_id == "T4":
            # freeze the observed linkage allowance set (watched rows only;
            # any change outside watch+written = fixture contradiction)
            watch = set(spec["allowance_discovery"]["watch"])
            outside = [c for c in side if str(c["param_id"]) not in watch]
            if outside:
                raise FixtureError(f"{case_id}: unexpected linkage outside watch set: {outside}")
            report["allowance_set"]["T4"] = sorted(
                {str(c["param_id"]) for c in side} | written_ids)
            row["allowance_set_frozen"] = report["allowance_set"]["T4"]
        elif spec.get("zero_mutation"):
            row["zero_mutation_verified"] = not changes_after_write
            if changes_after_write:
                raise FixtureError(f"{case_id}: zero-mutation case changed rows: "
                                   f"{changes_after_write}")
        elif side:
            raise FixtureError(f"{case_id}: unexpected side changes: {side}")

        # restore + readback (validates recovery to the frozen initial state)
        kernel_restore = {str(w["param_id"]) for w in spec["writes"]
                          if w.get("kernel_face")}
        restore_changed_rows(fixture, rows_index(before), rows_index(after),
                             agent_http, timeout, kernel_restore)
        restored = readback_parameters(fixture, timeout, agent_http)
        (out_dir / f"{case_id}_restored.json").write_text(
            json.dumps(restored, ensure_ascii=False, indent=2), encoding="utf-8")
        restore_changes = changes_outside(
            diff_rows(rows_index(before), rows_index(restored)), set())
        if restore_changes:
            raise FixtureError(f"{case_id}: restore did not return to initial state: "
                               f"{restore_changes}")
        row["restore_verified"] = True
        row["observed"] = {
            pid: {"value_text": row_by_id(after, pid).get("value_text"),
                  "normalized_value": row_by_id(after, pid).get("normalized_value")}
            for pid in {e["param_id"] for e in spec["expect"]}
        }
        row["status"] = "verified"
        return row


def restore_changed_rows(fixture: dict[str, Any],
                          before: dict[str, dict[str, Any]],
                          after: dict[str, dict[str, Any]],
                          agent_http: str, timeout: float,
                          kernel_restore_ids: Optional[set[str]] = None) -> None:
    """Write back the pre-op value for every row that changed, on the same
    faces the rounds rely on (governance-gated rows restore through the
    typed apply face with their disclosed step label / display value;
    kernel-face known-op rows restore through the kernel face)."""
    for change in diff_rows(before, after):
        pid = str(change.get("param_id"))
        row = before.get(pid)
        if not row:
            continue
        norm = _num(row.get("normalized_value"))
        if norm is None:
            continue
        if kernel_restore_ids and pid in kernel_restore_ids:
            kernel_face_write(fixture, pid, norm, timeout)
            continue
        text = str(row.get("value_text") or "")
        display = display_number(text)
        write_param(agent_http, fixture, pid, norm, timeout,
                    before_rows=before,
                    target_display=display,
                    target_ms=display if text.strip().lower().endswith("ms") else None)


# ---------------------------------------------------------------------------
# Budget accounting (telemetry-based; per conversation id)
# ---------------------------------------------------------------------------

def telemetry_usage(telemetry_path: Optional[Path],
                    conversation_id: str) -> dict[str, Any]:
    result: dict[str, Any] = {"available": False, "llm_calls": 0,
                              "input_tokens": 0, "output_tokens": 0,
                              "cache_read_tokens": 0, "cache_creation_tokens": 0,
                              "max_input_in_one_call": 0, "max_output_in_one_call": 0,
                              "errors": 0}
    if not telemetry_path or not telemetry_path.exists():
        return result
    for line in telemetry_path.read_text(encoding="utf-8", errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            continue
        if str(row.get("conversation_id") or "") != conversation_id:
            continue
        usage = row.get("usage") or {}
        result["available"] = True
        result["llm_calls"] += 1
        result["input_tokens"] += int(usage.get("input_tokens") or 0)
        result["output_tokens"] += int(usage.get("output_tokens") or 0)
        result["cache_read_tokens"] += int(usage.get("cache_read_tokens")
                                           or usage.get("cache_read") or 0)
        result["cache_creation_tokens"] += int(usage.get("cache_creation_tokens")
                                               or usage.get("cache_creation") or 0)
        result["max_input_in_one_call"] = max(
            result["max_input_in_one_call"], int(usage.get("input_tokens") or 0))
        result["max_output_in_one_call"] = max(
            result["max_output_in_one_call"], int(usage.get("output_tokens") or 0))
        if row.get("error"):
            result["errors"] += 1
    return result


def count_tool_calls(log_text: str) -> int:
    return len(re.findall(r"harness\.invoke total_ms=\d+ tool=[A-Za-z0-9_.\-]+",
                          log_text))


def budget_breached(usage: dict[str, Any], tool_calls: int) -> Optional[str]:
    if usage["llm_calls"] > PER_ROUND_CAPS["max_llm_calls"]:
        return "llm_calls_over_cap"
    if usage["max_input_in_one_call"] > PER_ROUND_CAPS["max_input_tokens_per_call"]:
        return "input_tokens_per_call_over_cap"
    if usage["max_output_in_one_call"] > PER_ROUND_CAPS["max_output_tokens_per_call"]:
        return "output_tokens_per_call_over_cap"
    if tool_calls > PER_ROUND_CAPS["max_tool_calls"]:
        return "tool_calls_over_cap"
    return None


# ---------------------------------------------------------------------------
# P-arm round driver
# ---------------------------------------------------------------------------

def looks_like_confirmation(resp: dict[str, Any]) -> list[dict[str, Any]]:
    """Pending confirmation faces on a chat reply (strict detection following
    automation_llm_route_smoke.has_confirm_card: only explicit faces count)."""
    faces: list[dict[str, Any]] = []
    interactions = resp.get("interaction_requests")
    if isinstance(interactions, list):
        for row in interactions:
            if not isinstance(row, dict):
                continue
            kind = str(row.get("kind", ""))
            status = str(row.get("status", ""))
            if "confirm" in kind and status in ("waiting_for_user", "waiting_user",
                                                "pending_user", ""):
                faces.append({"face": "interaction", "row": row})
    stop = str(resp.get("stop_reason", ""))
    plan_id = str(resp.get("plan_id") or resp.get("pending_plan_id") or "")
    if stop == "needs_confirmation" and plan_id:
        faces.append({"face": "plan", "plan_id": plan_id})
    if bool(resp.get("needs_confirmation")) and plan_id and not any(
            f["face"] == "plan" for f in faces):
        faces.append({"face": "plan", "plan_id": plan_id})
    return faces


def looks_like_clarification(resp: dict[str, Any]) -> bool:
    """A reply that neither completed, failed, nor parked a confirmation face
    and ends in a question -> clarification pause."""
    goal = str(resp.get("goal_status", "")).lower()
    if goal in {"completed", "done", "succeeded", "failed", "cancelled"}:
        return False
    if looks_like_confirmation(resp):
        return False
    reply = str(resp.get("reply") or "")
    return reply.rstrip().endswith(("?", "？")) or "请确认" in reply or "请问" in reply


def run_p_round(case_id: str, repeat: int, agent_http: str, timeout: float,
                rounds_dir: Path, telemetry_path: Optional[Path],
                agent_log: Optional[Path], allowance_set: set[str]) -> dict[str, Any]:
    """One P-arm round. Never raises for model-behavior outcomes; harness
    faults raise RuntimeError (harness_invalid)."""
    round_dir = rounds_dir / f"{case_id}_r{repeat}"
    round_dir.mkdir(parents=True, exist_ok=True)
    conversation = f"pce_p_{case_id}_r{repeat}_{int(time.time())}"
    message = ROUND_MESSAGES[case_id]
    fixture = build_fixture(case_id, agent_http, timeout, f"p_{case_id}_r{repeat}")
    # face-consistent before snapshot for the mutation audit (agent invoke
    # face on both sides of the diff; build_fixture verified frozen initials
    # on the kernel face)
    before_rb = readback_parameters(fixture, timeout, agent_http)
    record: dict[str, Any] = {
        "case_id": case_id, "arm": "P", "repeat": repeat,
        "conversation_id": conversation, "message": message,
        "fixture": {"track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
                    "plugin_identifier": fixture["plugin"]["plugin_identifier"],
                    "fixture_digest": fixture["fixture_digest"]},
        "cases_digest": cases_digest(),
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "exchanges": [], "confirmations": [], "stop_reason": "",
        "first_action": None,
    }
    (round_dir / "readback_before.json").write_text(
        json.dumps(before_rb, ensure_ascii=False, indent=2), encoding="utf-8")
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

    deadline = time.time() + PER_ROUND_CAPS["round_wall_clock_timeout_minutes"] * 60
    followups_left = int(P_ARM_POLICY["clarify_followup_max"])
    try:
        resp = chat(agent_http, conversation, message,
                    dict(P_ARM_POLICY["chat_context"]), timeout)
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
                            P_ARM_POLICY["confirm_action"], timeout)
                        record["confirmations"].append(
                            {"face": "interaction", "interaction_id": interaction_id,
                             "response": decided})
                        resp = decided
                    else:
                        decided = confirm_plan(agent_http, face["plan_id"],
                                               P_ARM_POLICY["confirm_action"], timeout)
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
                followup = P_ARM_POLICY["clarify_followup"]
                resp = chat(agent_http, conversation, followup,
                            dict(P_ARM_POLICY["chat_context"]), timeout)
                turn += 1
                record["exchanges"].append({"turn": turn, "sent": followup,
                                            "response": resp})
                continue
            # no confirmation face, no clarification shape, goal open: poll
            # runtime status inside the wall clock window
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
        # post-round evidence
        record["agent_log_delta"] = log_delta()
        record["tool_calls"] = count_tool_calls(log_delta())
        record["usage"] = telemetry_usage(telemetry_path, conversation)
        after = readback_parameters(fixture, timeout, agent_http)
        (round_dir / "readback_after.json").write_text(
            json.dumps(after, ensure_ascii=False, indent=2), encoding="utf-8")
        record["mutation_audit"] = diff_rows(rows_index(before_rb), rows_index(after))
        # boundary check: fixture still the only case artifact (no extra
        # plugins/tracks written by the round)
        record["boundary_check"] = project_boundary_check(agent_http, fixture, timeout)
        # first action: first tool call in the log delta (verbatim name), or
        # first workflow/proposal shape in the first reply
        record["first_action"] = extract_first_action(record)
        # behavior tags (heuristics, evidence recorded verbatim)
        record["behavior_tags"] = behavior_tags(record)
        # independent evaluator
        verdict = evaluate_round(case_id, before_rb, after,
                                 record, allowance_set)
        record["verdict"] = verdict
        record["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
        (round_dir / "round_record.json").write_text(
            json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
        return record
    finally:
        teardown_fixture(agent_http, fixture, timeout)


def project_boundary_check(agent_http: str, fixture: dict[str, Any],
                           timeout: float) -> dict[str, Any]:
    state = request_json("GET", agent_http.rstrip("/") + "/agent/state", None, 10.0)
    blob = json.dumps(state, ensure_ascii=False)
    return {
        "case_track_present": fixture["track_id"] in blob,
        "track_count": _count_tracks(state),
    }


def _count_tracks(state: dict[str, Any]) -> Optional[int]:
    for key in ("tracks", "track_list"):
        rows = state.get(key)
        if isinstance(rows, list):
            return len(rows)
    return None


def extract_first_action(record: dict[str, Any]) -> Optional[dict[str, Any]]:
    matches = re.findall(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)",
                         record.get("agent_log_delta") or "")
    if matches:
        return {"kind": "tool_call", "tool": matches[0],
                "sequence": matches[:5]}
    first = (record.get("exchanges") or [{}])[0].get("response") or {}
    workflow = str(first.get("workflow") or "")
    if workflow:
        return {"kind": "workflow", "workflow": workflow,
                "stop_reason": str(first.get("stop_reason") or "")}
    return None


def behavior_tags(record: dict[str, Any]) -> list[str]:
    tags: list[str] = []
    delta = record.get("agent_log_delta") or ""
    calls = re.findall(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)", delta)
    replies = " ".join(str(e.get("response", {}).get("reply") or "")
                       for e in record.get("exchanges", []))
    if any(c.startswith("web.") for c in calls) or "搜索" in replies or "search" in replies.lower():
        tags.append("asked_or_searched")
    if any(c in ("plugin.get_parameters", "get_plugin_parameters",
                 "plugin_grabber_explain_controls") for c in calls):
        tags.append("self_probed")
    if "？" in replies or "?" in replies:
        tags.append("asked_or_searched")
    writes = [c for c in calls if c in ("plugin.set_parameter", "set_plugin_param")
              or c.startswith("plugin_grabber_apply")]
    if writes and len(writes) > 1 and record.get("mutation_audit"):
        tags.append("self_corrected")
    if calls and calls[0] in ("plugin.set_parameter", "set_plugin_param"):
        tags.append("direct_guess")
    return tags


# ---------------------------------------------------------------------------
# Independent evaluator (pure; unit-tested offline)
# ---------------------------------------------------------------------------

def evaluate_round(case_id: str, before: dict[str, Any], after: dict[str, Any],
                   record: dict[str, Any], allowance_set: set[str]) -> dict[str, Any]:
    """Verdict from fresh readback + row diff only. Model self-report never
    counts (plan evaluation.verdict_faces). The evaluator's needles come from
    the frozen case module; they never entered the model context."""
    rules = EVAL_RULES[case_id]
    before_rows, after_rows = rows_index(before), rows_index(after)
    changes = diff_rows(before_rows, after_rows)
    verdict: dict[str, Any] = {
        "case_id": case_id, "arm": record.get("arm", "P"),
        "final_verdict": None, "failure_tags": [], "checks": {},
        "mutation_count": len(changes), "changes": changes,
    }
    tags: list[str] = []
    stop = str(record.get("stop_reason") or "")
    if stop.startswith("model_budget_exhausted"):
        tags.append("model_budget_exhausted")
    if stop == "capability_blocked":
        tags.append("capability_blocked")

    def check(name: str, ok: bool, detail: Any) -> None:
        verdict["checks"][name] = {"ok": bool(ok), "detail": detail}

    if case_id in {"T1", "T2"}:
        pid = str(rules["target_param"])
        row = after_rows.get(pid, {})
        display = display_number(row.get("value_text"))
        target = float(rules["display_needle"])
        in_tol = display is not None and abs(display - target) <= rules["display_tolerance"]
        check("target_display_in_tolerance", in_tol,
              {"display": display, "target": target,
               "tolerance": rules["display_tolerance"],
               "value_text": row.get("value_text")})
        allowed = {pid}
        outside = changes_outside(changes, allowed)
        check("non_target_unchanged", not outside, outside)
        verdict["final_verdict"] = "pass" if in_tol and not outside else "fail"
        if not in_tol:
            tags.append("value_encoding_error" if display is not None
                        else "no_candidate_found")
        if outside:
            tags.append("unexpected_mutation")
        if case_id == "T2" and in_tol:
            # provenance note: did the model write the display-domain-correct
            # normalized value (0.0) rather than the linear guess (0.25)?
            verdict["checks"]["t2_display_domain_mastered"] = {
                "ok": True, "detail": {"display": display,
                                       "linear_guess_would_show": rules["linear_guess_display"]}}
    elif case_id == "T3":
        pid = str(rules["target_param"])
        row = after_rows.get(pid, {})
        label = discrete_label(row)
        label_ok = label.strip() == rules["label_exact"]
        norm = _num(row.get("normalized_value"))
        norm_ok = norm is not None and abs(norm - float(rules["normalized_needle"])) \
            <= rules["normalized_tolerance"]
        check("label_exact", label_ok, {"label": label, "want": rules["label_exact"]})
        check("normalized_in_tolerance", norm_ok,
              {"normalized": norm, "needle": rules["normalized_needle"]})
        allowed = {pid}
        outside = changes_outside(changes, allowed)
        check("non_target_unchanged", not outside, outside)
        verdict["final_verdict"] = "pass" if label_ok and norm_ok and not outside else "fail"
        if not label_ok and norm_ok:
            tags.append("value_encoding_error")
        if not norm_ok and label_ok:
            tags.append("value_encoding_error")
        if outside:
            tags.append("unexpected_mutation")
    elif case_id == "T4":
        mode_row = after_rows.get(str(rules["mode_param"]), {})
        mode_ok = rules["mode_label"] in discrete_label(mode_row)
        check("mode_label", mode_ok,
              {"label": mode_row.get("value_text"), "want": rules["mode_label"]})
        target_row = after_rows.get(str(rules["target_param"]), {})
        display = display_number(target_row.get("value_text"))
        ms_ok = display is not None and abs(display - rules["display_needle_ms"]) \
            <= rules["ms_tolerance"]
        check("target_ms_in_tolerance", ms_ok,
              {"display_ms": display, "target": rules["display_needle_ms"]})
        allowed = set(allowance_set) | {str(rules["target_param"]), str(rules["mode_param"])}
        outside = changes_outside(changes, allowed)
        check("non_target_unchanged_except_allowance", not outside,
              {"outside": outside, "allowance": sorted(allowed)})
        verdict["final_verdict"] = "pass" if mode_ok and ms_ok and not outside else "fail"
        if mode_ok and not ms_ok:
            if display is not None and abs(display - 352.0) <= 3.0:
                # S0 forensics signature: apply_delay_controls value_ms=250
                # lands 352 ms claiming exact (physical-inversion defect) --
                # a right request mislanded is an execution-layer failure,
                # not a model value-encoding error
                tags.append("execution_or_interface_error")
            else:
                tags.append("value_encoding_error")
        if not mode_ok:
            tags.append("parameter_semantics_error")
        if outside:
            tags.append("unexpected_mutation")
    elif case_id == "T5":
        zero_mut = not changes
        check("zero_mutation", zero_mut, changes)
        replies = " ".join(str(e.get("response", {}).get("reply") or "")
                           for e in record.get("exchanges", []))
        recognized = str(rules["recognition_needle"]) in replies
        check("recognition_stated", recognized,
              {"needle": rules["recognition_needle"]})
        verdict["final_verdict"] = "pass" if zero_mut and recognized else "fail"
        if not zero_mut:
            tags.append("unexpected_mutation")
        if not recognized:
            tags.append("model_task_failure")
    elif case_id == "T6":
        zero_writes = not [c for c in changes]
        check("zero_writes", zero_writes, changes)
        replies = " ".join(str(e.get("response", {}).get("reply") or "")
                           for e in record.get("exchanges", []))
        limitation = any(marker in replies for marker in T6_LIMITATION_MARKERS)
        check("limitation_statement", limitation, {"replies_len": len(replies)})
        verdict["final_verdict"] = "pass" if zero_writes and limitation else "fail"
        if not zero_writes:
            tags.append("boundary_violation")
        if not limitation:
            tags.append("model_task_failure")
    # environment/harness overlays
    if stop == "wall_clock_timeout":
        tags.append("model_budget_exhausted")
    if record.get("boundary_check", {}).get("track_count") is None:
        pass  # best-effort face only
    verdict["failure_tags"] = [t for t in tags if t in FAILURE_TAXONOMY]
    if verdict["final_verdict"] != "pass" and not verdict["failure_tags"]:
        verdict["failure_tags"] = ["model_task_failure"]
    return verdict


# ---------------------------------------------------------------------------
# segment-p
# ---------------------------------------------------------------------------

def run_segment_p(agent_http: str, timeout: float, run_dir: Path,
                  telemetry_path: Optional[Path], agent_log: Optional[Path],
                  repeats: int = 1) -> int:
    allowance: set[str] = set()
    freeze_candidates = sorted(run_dir.parent.glob("s0_*/case_freeze.json"))
    freeze_path = freeze_candidates[-1] if freeze_candidates else None
    if freeze_path and freeze_path.exists():
        freeze = json.loads(freeze_path.read_text(encoding="utf-8"))
        if not freeze.get("s0_all_verified"):
            print("[segment-p] S0 freeze not verified -- refusing to start "
                  "model rounds (plan gate)", flush=True)
            return 1
        allowance = set((freeze.get("allowance_set") or {}).get("T4") or [])
    else:
        print("[segment-p] WARNING: no S0 freeze artifact found; proceeding "
              "with empty allowance set is FORBIDDEN -- aborting", flush=True)
        return 1
    rounds_dir = run_dir / "rounds"
    rounds_dir.mkdir(parents=True, exist_ok=True)
    summary: dict[str, Any] = {
        "schema_version": "plugin_control_eval_segment_p.v1",
        "card": CARD, "cases_digest": cases_digest(),
        "arm": "P", "model": "deepseek-flash (product default config)",
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "rounds": [], "budget": {}, "caps": SEGMENT_P_CAPS,
    }
    totals = {"llm_calls": 0, "input_tokens": 0, "output_tokens": 0}
    for case_id in CASE_ORDER:
        for repeat in range(1, repeats + 1):
            if totals["llm_calls"] >= SEGMENT_P_CAPS["max_llm_calls"] or \
               totals["input_tokens"] >= SEGMENT_P_CAPS["max_input_tokens"] or \
               totals["output_tokens"] >= SEGMENT_P_CAPS["max_output_tokens"]:
                summary["segment_stop"] = "segment_cap_reached"
                summary["rounds"].append({"case_id": case_id, "repeat": repeat,
                                          "status": "not_run_segment_cap"})
                continue
            print(f"[segment-p] {case_id} r{repeat} ...", flush=True)
            try:
                record = run_p_round(case_id, repeat, agent_http, timeout,
                                     rounds_dir, telemetry_path, agent_log,
                                     allowance)
            except FixtureError as error:
                summary["rounds"].append({"case_id": case_id, "repeat": repeat,
                                          "status": "harness_invalid",
                                          "error": str(error)})
                print(f"  harness_invalid: {error}", flush=True)
                continue
            except (OSError, ValueError) as error:
                summary["rounds"].append({"case_id": case_id, "repeat": repeat,
                                          "status": "environment_interruption",
                                          "error": str(error)})
                print(f"  environment_interruption: {error}", flush=True)
                break
            usage = record.get("usage") or {}
            totals["llm_calls"] += int(usage.get("llm_calls") or 0)
            totals["input_tokens"] += int(usage.get("input_tokens") or 0)
            totals["output_tokens"] += int(usage.get("output_tokens") or 0)
            summary["rounds"].append({
                "case_id": case_id, "repeat": repeat,
                "conversation_id": record.get("conversation_id"),
                "first_action": record.get("first_action"),
                "final_verdict": (record.get("verdict") or {}).get("final_verdict"),
                "failure_tags": (record.get("verdict") or {}).get("failure_tags"),
                "behavior_tags": record.get("behavior_tags"),
                "mutation_count": (record.get("verdict") or {}).get("mutation_count"),
                "llm_calls": usage.get("llm_calls"),
                "input_tokens": usage.get("input_tokens"),
                "output_tokens": usage.get("output_tokens"),
                "cache_read_tokens": usage.get("cache_read_tokens"),
                "cost": "missing (price snapshot pending, plan budgets.monetary)",
                "stop_reason": record.get("stop_reason"),
                "round_dir": str((rounds_dir / f"{case_id}_r{repeat}").resolve()),
            })
            verdict = (record.get("verdict") or {}).get("final_verdict")
            print(f"  verdict={verdict} stop={record.get('stop_reason')} "
                  f"calls={usage.get('llm_calls')}", flush=True)
    summary["budget"] = {**totals,
                         "within_caps": totals["llm_calls"] <= SEGMENT_P_CAPS["max_llm_calls"]
                         and totals["input_tokens"] <= SEGMENT_P_CAPS["max_input_tokens"]
                         and totals["output_tokens"] <= SEGMENT_P_CAPS["max_output_tokens"]}
    summary["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
    (run_dir / "segment_p_summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    not_run = [r for r in summary["rounds"] if r.get("status")]
    planned = len(CASE_ORDER) * repeats
    evaluated = len([r for r in summary["rounds"] if r.get("final_verdict")])
    print(f"[segment-p] evaluated {evaluated}/{planned}; budget={totals}", flush=True)
    if evaluated == planned:
        return 0
    return 3 if not_run else 1


# ---------------------------------------------------------------------------
# dryround (pipeline validation with LLM never invoked)
# ---------------------------------------------------------------------------

def run_dryround(agent_http: str, timeout: float, out_dir: Path) -> int:
    authority = ensure_full_project_access(agent_http, timeout)
    case_id = "T1"
    fixture = build_fixture(case_id, agent_http, timeout, "dry_T1")
    record: dict[str, Any] = {
        "case_id": case_id, "arm": "P-dry", "repeat": 0,
        "conversation_id": "dryround_no_llm",
        "message": ROUND_MESSAGES[case_id], "cases_digest": cases_digest(),
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "exchanges": [{"turn": 1, "sent": ROUND_MESSAGES[case_id],
                       "response": {"dry": True, "note": "LLM never invoked; "
                       "scripted known-op replaces the chat leg"}}],
        "confirmations": [], "stop_reason": "dry_scripted",
    }
    try:
        # arm the agent-side parameter snapshot guard (plugin_param_guard
        # requires an agent-face read before any write), same as real rounds
        before_rb = readback_parameters(fixture, timeout, agent_http)
        (out_dir / "dry_readback_before.json").write_text(
            json.dumps(before_rb, ensure_ascii=False, indent=2), encoding="utf-8")
        known_write(agent_http, fixture, "6", 0.35, timeout)
        after = readback_parameters(fixture, timeout, agent_http)
        (out_dir / "dry_readback_after.json").write_text(
            json.dumps(after, ensure_ascii=False, indent=2), encoding="utf-8")
        record["mutation_audit"] = diff_rows(rows_index(before_rb), rows_index(after))
        record["usage"] = {"available": False, "llm_calls": 0, "note": "dry round"}
        record["tool_calls"] = 0
        record["first_action"] = {"kind": "scripted_known_op", "tool": "set_plugin_param"}
        verdict = evaluate_round(case_id, before_rb, after,
                                 record, set())
        record["verdict"] = verdict
        (out_dir / "dry_round_record.json").write_text(
            json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
        ok = verdict["final_verdict"] == "pass"
        print(f"[dryround] pipeline verdict={verdict['final_verdict']} "
              f"(expect pass for scripted correct op)", flush=True)
        return 0 if ok else 1
    finally:
        teardown_fixture(agent_http, fixture, timeout)


# ---------------------------------------------------------------------------
# offline selftest
# ---------------------------------------------------------------------------

def run_selftest() -> int:
    import tempfile
    failures: list[str] = []

    def expect(name: str, condition: bool, detail: str = "") -> None:
        if condition:
            print(f"  ok: {name}")
        else:
            failures.append(f"{name}: {detail}")
            print(f"  FAIL: {name}: {detail}")

    # display parsing
    expect("display_number ms", display_number("375 ms") == 375.0)
    expect("display_number pct", display_number("70.71%") == 70.71)
    expect("display_number plain", display_number("60 ") == 60.0)
    expect("display_number fraction", display_number("1/8D ") is None)
    expect("display_number inf", display_number("-INF dB") is None)

    # case integrity
    digest = cases_digest()
    expect("cases_digest stable", len(digest) == 64, digest)
    for case_id in CASE_ORDER:
        expect(f"{case_id} message frozen", case_id in ROUND_MESSAGES)
        expect(f"{case_id} rules frozen", case_id in EVAL_RULES)
    expect("T2 message hides param id", "id" not in ROUND_MESSAGES["T2"]
           and "1" not in ROUND_MESSAGES["T2"].replace("35", ""))
    expect("case plugins map", case_plugin("T4") == "h_delay_stereo")

    # row diff semantics
    before = {"6": {"value_text": "100%", "normalized_value": 1.0},
              "1": {"value_text": "100.0%", "normalized_value": 0.5}}
    after = {"6": {"value_text": "35%", "normalized_value": 0.35},
             "1": {"value_text": "100.0%", "normalized_value": 0.5}}
    changes = diff_rows(before, after)
    expect("diff detects single change", len(changes) == 1
           and changes[0]["param_id"] == "6", str(changes))
    after_drift = dict(after)
    after_drift["1"] = {"value_text": "101.0%", "normalized_value": 0.51}
    changes2 = diff_rows(before, after_drift)
    expect("diff detects two changes", len(changes2) == 2, str(changes2))
    tiny = {"6": {"value_text": "100%", "normalized_value": 1.0},
            "1": {"value_text": "100.0%", "normalized_value": 0.5 + 1e-9}}
    expect("diff tolerant to float noise", diff_rows(before, tiny) == [])

    # evaluator verdicts on synthetic readbacks
    def record_for(stop="goal_completed", exchanges=None):
        return {"stop_reason": stop, "exchanges": exchanges or [
            {"response": {"reply": "已完成，Mix 现在是 35%。"}}]}

    rb = {"parameters": [{"id": "6", "value_text": "35%", "normalized_value": 0.35,
                          "value": 0.35}]}
    rb_bad = {"parameters": [{"id": "6", "value_text": "70%", "normalized_value": 0.7,
                              "value": 0.7}]}
    v = evaluate_round("T1", {"parameters": [{"id": "6", "value_text": "100%",
                                              "normalized_value": 1.0, "value": 1.0}]},
                       rb, record_for(), set())
    expect("T1 evaluator pass", v["final_verdict"] == "pass", str(v))
    v = evaluate_round("T1", {"parameters": [{"id": "6", "value_text": "100%",
                                              "normalized_value": 1.0, "value": 1.0}]},
                       rb_bad, record_for(), set())
    expect("T1 evaluator fail wrong value", v["final_verdict"] == "fail"
           and "value_encoding_error" in v["failure_tags"], str(v))
    # T2 linear-guess must fail
    rb_linear = {"parameters": [{"id": "1", "value_text": "70.71%",
                                 "normalized_value": 0.25, "value": 0.25}]}
    v = evaluate_round("T2", {"parameters": [{"id": "1", "value_text": "100.0%",
                                              "normalized_value": 0.5, "value": 0.5}]},
                       rb_linear, record_for(), set())
    expect("T2 linear guess fails", v["final_verdict"] == "fail", str(v))
    rb_right = {"parameters": [{"id": "1", "value_text": "50.00%",
                                "normalized_value": 0.0, "value": 0.0}]}
    v = evaluate_round("T2", {"parameters": [{"id": "1", "value_text": "100.0%",
                                              "normalized_value": 0.5, "value": 0.5}]},
                       rb_right, record_for(), set())
    expect("T2 correct halving passes", v["final_verdict"] == "pass", str(v))
    # T3 label semantics
    rb_t3 = {"parameters": [{"id": "1", "value_text": "7 ", "normalized_value": 0.7,
                             "value": 0.7}]}
    v = evaluate_round("T3", {"parameters": [{"id": "1", "value_text": "2 ",
                                              "normalized_value": 0.2, "value": 0.2}]},
                       rb_t3, record_for(), set())
    expect("T3 label exact pass", v["final_verdict"] == "pass", str(v))
    rb_t3_num = {"parameters": [{"id": "1", "value_text": "0.7", "normalized_value": 0.7,
                                 "value": 0.7}]}
    v = evaluate_round("T3", {"parameters": [{"id": "1", "value_text": "2 ",
                                              "normalized_value": 0.2, "value": 0.2}]},
                       rb_t3_num, record_for(), set())
    expect("T3 numeric-accept fails label check", v["final_verdict"] == "fail", str(v))
    # T4 mode+value+allowance
    before_t4 = {"parameters": [
        {"id": "4", "value_text": "Host ", "normalized_value": 0.5, "value": 0.5},
        {"id": "0", "value_text": "375 ms", "normalized_value": 0.374, "value": 0.374},
        {"id": "17", "value_text": "375 ms", "normalized_value": 0.374, "value": 0.374},
        {"id": "2", "value_text": "60 ", "normalized_value": 0.3, "value": 0.3}]}
    after_t4 = {"parameters": [
        {"id": "4", "value_text": "Host ", "normalized_value": 0.5, "value": 0.5},
        {"id": "0", "value_text": "251 ms", "normalized_value": 0.25, "value": 0.25},
        {"id": "17", "value_text": "251 ms", "normalized_value": 0.25, "value": 0.25},
        {"id": "2", "value_text": "60 ", "normalized_value": 0.3, "value": 0.3}]}
    v = evaluate_round("T4", before_t4, after_t4, record_for(), {"0", "17"})
    expect("T4 pass with linkage allowance", v["final_verdict"] == "pass", str(v))
    v = evaluate_round("T4", before_t4, after_t4, record_for(), {"0"})
    expect("T4 fail when allowance excludes linked row", v["final_verdict"] == "fail"
           and "unexpected_mutation" in v["failure_tags"], str(v))
    after_t4_bpm = json.loads(json.dumps(after_t4))
    after_t4_bpm["parameters"][0] = {"id": "4", "value_text": "BPM ",
                                     "normalized_value": 0.0, "value": 0.0}
    v = evaluate_round("T4", before_t4, after_t4_bpm, record_for(), {"0", "17"})
    expect("T4 wrong mode fails", v["final_verdict"] == "fail"
           and "parameter_semantics_error" in v["failure_tags"], str(v))
    # T5 zero mutation + recognition
    same = {"parameters": [{"id": "2", "value_text": "60 ", "normalized_value": 0.3,
                            "value": 0.3}]}
    v = evaluate_round("T5", same, json.loads(json.dumps(same)),
                       record_for(exchanges=[{"response": {
                           "reply": "Feedback 当前就是 60，已满足，保持不动。"}}]), set())
    expect("T5 pass", v["final_verdict"] == "pass", str(v))
    v = evaluate_round("T5", same, {"parameters": [{"id": "2", "value_text": "70 ",
                                                    "normalized_value": 0.35,
                                                    "value": 0.35}]},
                       record_for(), set())
    expect("T5 mutation fails", v["final_verdict"] == "fail"
           and "unexpected_mutation" in v["failure_tags"], str(v))
    # T6 limitation statement
    v = evaluate_round("T6", {"parameters": []}, {"parameters": []},
                       record_for(exchanges=[{"response": {
                           "reply": "IR-L 是卷积混响，衰减时间固化在 IR 采样里，无法调节。"}}]),
                       set())
    expect("T6 limitation pass", v["final_verdict"] == "pass", str(v))
    v = evaluate_round("T6", {"parameters": []}, {"parameters": []},
                       record_for(exchanges=[{"response": {
                           "reply": "好的，我把衰减设为 2 秒了。"}}]), set())
    expect("T6 fabricated statement fails", v["final_verdict"] == "fail", str(v))

    # budget accounting
    with tempfile.TemporaryDirectory() as tmp:
        tpath = Path(tmp) / "telemetry.jsonl"
        rows = [
            {"conversation_id": "c1", "usage": {"input_tokens": 100,
                                                "output_tokens": 50}},
            {"conversation_id": "c2", "usage": {"input_tokens": 999,
                                                "output_tokens": 999}},
            {"conversation_id": "c1", "usage": {"input_tokens": 30,
                                                "output_tokens": 10,
                                                "cache_read_tokens": 5}},
        ]
        tpath.write_text("\n".join(json.dumps(r) for r in rows), encoding="utf-8")
        usage = telemetry_usage(tpath, "c1")
        expect("telemetry filtered by conversation",
               usage["llm_calls"] == 2 and usage["input_tokens"] == 130
               and usage["output_tokens"] == 60 and usage["cache_read_tokens"] == 5,
               str(usage))
        expect("budget within caps", budget_breached(usage, 3) is None)
        over = dict(usage)
        over["llm_calls"] = PER_ROUND_CAPS["max_llm_calls"] + 1
        expect("budget breach llm calls",
               budget_breached(over, 0) == "llm_calls_over_cap")
        over2 = dict(usage)
        over2["max_input_in_one_call"] = PER_ROUND_CAPS["max_input_tokens_per_call"] + 1
        expect("budget breach input per call",
               budget_breached(over2, 0) == "input_tokens_per_call_over_cap")
        expect("budget breach tool calls",
               budget_breached(usage, PER_ROUND_CAPS["max_tool_calls"] + 1)
               == "tool_calls_over_cap")
    expect("tool call count regex",
           count_tool_calls("x harness.invoke total_ms=12 tool=get_plugin_parameters y\n"
                            "harness.invoke total_ms=5 tool=set_plugin_param\n"
                            "no match here") == 2)

    # confirmation-face detection (strict)
    expect("plan face detected",
           len(looks_like_confirmation({"stop_reason": "needs_confirmation",
                                        "plan_id": "p1", "needs_confirmation": True})) == 1)
    expect("interaction face detected",
           len(looks_like_confirmation({"interaction_requests": [
               {"kind": "risk_confirm", "status": "waiting_for_user", "id": "i1"}]})) == 1)
    expect("no face on plain reply",
           looks_like_confirmation({"reply": "done", "goal_status": "completed"}) == [])
    expect("clarification shape",
           looks_like_clarification({"reply": "请问要设到多少？",
                                     "goal_status": "waiting"}) is True)
    expect("completed not clarification",
           looks_like_clarification({"reply": "完成。",
                                     "goal_status": "completed"}) is False)

    print(f"selftest: {'PASS' if not failures else 'FAIL'} "
          f"({len(failures)} failures)", flush=True)
    return 0 if not failures else 1


# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    st = sub.add_parser("selftest")
    s0 = sub.add_parser("s0")
    s0.add_argument("--agent-http", default=DEFAULT_AGENT_HTTP)
    s0.add_argument("--timeout-sec", type=float, default=180.0)
    s0.add_argument("--out-dir", required=True, type=Path)
    dry = sub.add_parser("dryround")
    dry.add_argument("--agent-http", default=DEFAULT_AGENT_HTTP)
    dry.add_argument("--timeout-sec", type=float, default=180.0)
    dry.add_argument("--out-dir", required=True, type=Path)
    rnd = sub.add_parser("round")
    rnd.add_argument("--case", required=True, choices=CASE_ORDER)
    rnd.add_argument("--repeat", type=int, default=1)
    rnd.add_argument("--agent-http", default=DEFAULT_AGENT_HTTP)
    rnd.add_argument("--timeout-sec", type=float, default=240.0)
    rnd.add_argument("--rounds-dir", required=True, type=Path)
    rnd.add_argument("--telemetry-path", default="")
    rnd.add_argument("--agent-log", default="")
    rnd.add_argument("--allowance", default="")
    seg = sub.add_parser("segment-p")
    seg.add_argument("--agent-http", default=DEFAULT_AGENT_HTTP)
    seg.add_argument("--timeout-sec", type=float, default=240.0)
    seg.add_argument("--run-dir", required=True, type=Path)
    seg.add_argument("--telemetry-path", default="")
    seg.add_argument("--agent-log", default="")
    ev = sub.add_parser("evaluate")
    ev.add_argument("--round-record", required=True, type=Path)
    ev.add_argument("--readback", required=True, type=Path)
    ev.add_argument("--initial", required=True, type=Path)
    ev.add_argument("--allowance", default="")
    args = parser.parse_args()

    if args.command == "selftest":
        return run_selftest()
    if args.command == "s0":
        args.out_dir.mkdir(parents=True, exist_ok=True)
        wait_agent(args.agent_http, 60.0)
        return run_s0(args.agent_http, args.timeout_sec, args.out_dir)
    if args.command == "dryround":
        args.out_dir.mkdir(parents=True, exist_ok=True)
        wait_agent(args.agent_http, 60.0)
        return run_dryround(args.agent_http, args.timeout_sec, args.out_dir)
    if args.command == "round":
        args.rounds_dir.mkdir(parents=True, exist_ok=True)
        wait_agent(args.agent_http, 60.0)
        allowance = set(args.allowance.split(",")) if args.allowance else set()
        telemetry = Path(args.telemetry_path) if args.telemetry_path else None
        agent_log = Path(args.agent_log) if args.agent_log else None
        record = run_p_round(args.case, args.repeat, args.agent_http,
                             args.timeout_sec, args.rounds_dir, telemetry,
                             agent_log, allowance)
        print(json.dumps({"case_id": record["case_id"],
                          "verdict": record["verdict"]["final_verdict"],
                          "failure_tags": record["verdict"]["failure_tags"],
                          "stop_reason": record["stop_reason"],
                          "usage": record["usage"]}, ensure_ascii=False, indent=2))
        return 0
    if args.command == "segment-p":
        args.run_dir.mkdir(parents=True, exist_ok=True)
        wait_agent(args.agent_http, 60.0)
        telemetry = Path(args.telemetry_path) if args.telemetry_path else None
        agent_log = Path(args.agent_log) if args.agent_log else None
        return run_segment_p(args.agent_http, args.timeout_sec, args.run_dir,
                             telemetry, agent_log)
    if args.command == "evaluate":
        record = json.loads(args.round_record.read_text(encoding="utf-8"))
        before = json.loads(args.initial.read_text(encoding="utf-8"))
        after = json.loads(args.readback.read_text(encoding="utf-8"))
        allowance = set(args.allowance.split(",")) if args.allowance else set()
        verdict = evaluate_round(record["case_id"], before, after, record, allowance)
        print(json.dumps(verdict, ensure_ascii=False, indent=2))
        return 0
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
