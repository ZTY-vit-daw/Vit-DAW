#!/usr/bin/env python3
"""PLUGGRABBER-TIMEDYN-COMPAT-1 time-domain compatibility matrix driver.

Real-stack driver (started by pgtdc_compat_verify.ps1 against the
dev_agent_smoke kernel+agent pair; no Godot UI, agent HTTP + kernel ZMQ
faces only, zero LLM):

  1. discovery: plugin_search enumeration of the in-presence time-domain
     targets (FabFilter Pro-R / Timeless 3, Waves WaveShell time-domain
     plugins, ValhallaSupermassive) with honest out-of-scope reasons;
  2. matrix: per case one disposable track -> plugin.load_to_rack ->
     kernel get_plugin_parameters (saved) -> plugin_grabber.inspect_reverb
     AND inspect_delay (both saved; a rejection is recorded as its code,
     never retried away) -> track.delete;
  3. spot: Abbey Road Plates inspect_reverb must resolve with
     plate_damper_family tail evidence at 0.90 confidence and one
     controlled apply_reverb_controls write on the decay binding must land
     (actual_readback + independent parameter readback); FabFilter Pro-R
     takes the same one-write spot with direct_wording tier expectations.

Exit 0 only when every discovered target carries a matrix row (captured or
explicit reason) and both spot writes read back changed values.
"""
from __future__ import annotations

import argparse
import json
import re
import time
from pathlib import Path
from typing import Any

from compressor_compat_matrix_smoke import (
    first_text,
    invoke,
    kernel_command,
    require_ok,
    resolve_identifier,
    result_of,
)

DISCOVERY_QUERIES = [
    # FabFilter time-domain targets
    "Pro-R", "Timeless",
    # Waves shell time-domain families (enumeration + licensing decide)
    "Abbey Road Plates", "Abbey Road Chambers", "Abbey Road Reverb",
    "H-Reverb", "H-Delay", "TrueVerb", "Doubler", "Manny Marroquin Delay",
    "Reverb", "Delay",
    # P3 freebie
    "Supermassive",
]

REVERB_WORDS = ("reverb", "verb", "plates", "chambers", "supermassive")
DELAY_WORDS = ("delay", "timeless", "echo", "doubler")

CASE_ORDER = ["fabfilter_pror", "fabfilter_timeless"]


def vendor_of(name: str, path: str) -> str:
    combined = (name + " " + path).lower()
    if "waveshell" in combined or "waves" in path.lower():
        return "waves"
    if "fabfilter" in combined:
        return "fabfilter"
    if "valhalla" in combined:
        return "valhalla"
    return "other"


def family_of(name: str) -> str:
    lowered = name.lower()
    if any(word in lowered for word in DELAY_WORDS):
        return "delay"
    if any(word in lowered for word in REVERB_WORDS):
        return "reverb"
    return ""


def case_id_for(name: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "_", name.lower()).strip("_")
    return slug or "case"


def discover(timeout: float) -> tuple[list[dict[str, Any]], list[dict[str, Any]], list[str]]:
    rows: dict[str, dict[str, Any]] = {}
    for query in DISCOVERY_QUERIES:
        reply = kernel_command({"cmd": "plugin_search", "query": query, "limit": 64}, timeout)
        batch: list[dict[str, Any]] = []
        for key in ("plugins", "entries", "results"):
            if isinstance(reply.get(key), list):
                batch = [row for row in reply[key] if isinstance(row, dict)]
                break
        for row in batch:
            identifier = first_text(row, "identifier", "plugin_identifier")
            if identifier:
                rows.setdefault(identifier, row)
    targets: list[dict[str, Any]] = []
    out_of_scope: list[dict[str, Any]] = []
    for identifier, row in sorted(rows.items()):
        name = first_text(row, "name", "plugin_name")
        path = first_text(row, "file_or_identifier", "plugin_path", "path")
        vendor = vendor_of(name, path)
        family = family_of(name)
        entry = {"plugin_name": name, "plugin_path": path, "plugin_identifier": identifier,
                 "vendor": vendor, "family": family}
        if vendor == "other":
            out_of_scope.append({**entry, "reason": "non-target vendor"})
        elif family == "":
            out_of_scope.append({**entry, "reason": f"{vendor} non-time-domain name"})
        else:
            targets.append(entry)
    targets.sort(key=lambda case: (
        CASE_ORDER.index(case_id_for(case["plugin_name"]))
        if case_id_for(case["plugin_name"]) in CASE_ORDER else len(CASE_ORDER),
        case["plugin_name"]))
    return targets, out_of_scope, sorted(rows)


def summarize_inspect(response: dict[str, Any], family: str) -> dict[str, Any]:
    status = str(response.get("status", "")).lower()
    result = result_of(response)
    stage_key = f"{family}_stage"
    row: dict[str, Any] = {"tool": f"plugin_grabber.inspect_{family}", "invoke_status": status}
    if status in {"ok", "success", "completed"}:
        stage = result.get(stage_key) if isinstance(result.get(stage_key), dict) else {}
        sections: dict[str, list[str]] = {}
        for section in ("tail", "timing", "character", "output", "feedback"):
            roles = sorted({str(item.get("role")) for item in (stage.get(section) or [])
                            if isinstance(item, dict) and item.get("role")})
            if roles:
                sections[section] = roles
        row.update({
            "resolved": True,
            "classification": result.get("classification"),
            "confidence": result.get("confidence"),
            "tail_evidence": result.get("tail_evidence"),
            "generation": first_text(result.get("control_topology") or {}, "generation"),
            "sections": sections,
        })
    else:
        row.update({
            "resolved": False,
            "rejection_code": first_text(result, "code") or "error",
            "error": first_text(response, "error"),
        })
    return row


def wait_parameters(timeout: float, track_id: str, plugin_id: str) -> dict[str, Any]:
    deadline = time.time() + 60.0
    params: dict[str, Any] = {}
    while time.time() < deadline:
        params = kernel_command({"cmd": "get_plugin_parameters", "track_id": track_id,
                                 "plugin_id": plugin_id, "include_parameters": True}, timeout)
        rows = params.get("parameters")
        if params.get("status") == "ok" and isinstance(rows, list) and rows:
            return params
        time.sleep(0.8)
    return params


def load_case(base: str, case: dict[str, Any], timeout: float) -> tuple[str, str, dict[str, Any]]:
    track = require_ok(invoke(base, "track.add_audio", {
        "name": f"PGTDC matrix {case['plugin_name']}"}, timeout, True), "track.add_audio")
    track_id = first_text(track, "track_id", "id")
    identifier, resolution = resolve_identifier(case, timeout)
    loaded = require_ok(invoke(base, "plugin.load_to_rack", {
        "track_id": track_id,
        "plugin_path": case["plugin_path"],
        "plugin_name": case["plugin_name"],
        "plugin_identifier": identifier,
    }, timeout, True), "plugin.load_to_rack")
    plugin_id = first_text(loaded, "plugin_id", "node_id", "id")
    if not plugin_id:
        raise RuntimeError(f"plugin.load_to_rack returned no plugin id: {loaded}")
    return track_id, plugin_id, resolution


def capture_case(base: str, case: dict[str, Any], timeout: float,
                 output_dir: Path) -> dict[str, Any]:
    case_id = case_id_for(case["plugin_name"])
    track_id = ""
    try:
        track_id, plugin_id, resolution = load_case(base, case, timeout)
        params = wait_parameters(timeout, track_id, plugin_id)
        if not (params.get("status") == "ok" and params.get("parameters")):
            raise RuntimeError(f"parameters unavailable: {str(params)[:300]}")
        (output_dir / f"{case_id}.parameters.json").write_text(json.dumps({
            "case": case, "plugin_resolution": resolution, "parameters": params,
        }, ensure_ascii=False, indent=2), encoding="utf-8")
        inspects = {
            "reverb": summarize_inspect(invoke(base, "plugin_grabber.inspect_reverb", {
                "track_id": track_id, "plugin_id": plugin_id}, timeout), "reverb"),
            "delay": summarize_inspect(invoke(base, "plugin_grabber.inspect_delay", {
                "track_id": track_id, "plugin_id": plugin_id}, timeout), "delay"),
        }
        (output_dir / f"{case_id}.inspects.json").write_text(json.dumps({
            "case": case, "inspects": inspects}, ensure_ascii=False, indent=2), encoding="utf-8")
        expected = case["family"]
        resolved_family = inspects[expected]["resolved"]
        return {"id": case_id, "plugin_name": case["plugin_name"], "plugin_identifier": case["plugin_identifier"],
                "vendor": case["vendor"], "expected_family": expected, "status": "captured",
                "expected_family_resolved": resolved_family, "inspects": inspects}
    finally:
        if track_id:
            try:
                require_ok(invoke(base, "track.delete", {"track_id": track_id}, timeout, True), "track.delete")
            except Exception:
                pass


def parse_number(text: Any) -> float | None:
    if isinstance(text, (int, float)):
        return float(text)
    if isinstance(text, str):
        match = re.search(r"-?\d+(?:\.\d+)?", text)
        if match:
            return float(match.group(0))
    return None


def stage_binding_rows(inspect: dict[str, Any], family: str, section: str) -> list[dict[str, Any]]:
    stage = inspect.get(f"{family}_stage") if isinstance(inspect.get(f"{family}_stage"), dict) else {}
    return [row for row in (stage.get(section) or []) if isinstance(row, dict)]


def spot_case(base: str, case: dict[str, Any], timeout: float, output_dir: Path,
              case_id: str, expect_evidence: str, expect_confidence: float) -> dict[str, Any]:
    track_id = ""
    result: dict[str, Any] = {"case_id": case_id, "plugin_name": case["plugin_name"],
                              "expect_tail_evidence": expect_evidence}
    try:
        track_id, plugin_id, _resolution = load_case(base, case, timeout)
        before = wait_parameters(timeout, track_id, plugin_id)
        inspect = require_ok(invoke(base, "plugin_grabber.inspect_reverb", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout), "inspect_reverb")
        result["inspect"] = {
            "classification": inspect.get("classification"),
            "confidence": inspect.get("confidence"),
            "tail_evidence": inspect.get("tail_evidence"),
            "generation": first_text(inspect.get("control_topology") or {}, "generation"),
        }
        if inspect.get("tail_evidence") != expect_evidence:
            raise RuntimeError(f"tail_evidence={inspect.get('tail_evidence')!r} want {expect_evidence!r}")
        if parse_number(inspect.get("confidence")) != expect_confidence:
            raise RuntimeError(f"confidence={inspect.get('confidence')} want {expect_confidence}")
        tail_rows = stage_binding_rows(inspect, "reverb", "tail")
        decay_rows = [row for row in tail_rows if row.get("role") == "decay"]
        if not decay_rows:
            raise RuntimeError(f"no decay binding in tail: {tail_rows}")
        binding = decay_rows[0]
        result["write_target"] = {"role": "decay", "param_id": binding.get("param_id"),
                                  "name": binding.get("name"), "domain": binding.get("domain")}
        current = parse_number(binding.get("current_physical"))
        if current is None:
            current = parse_number(binding.get("current_text"))
        if current is None:
            current = 0.5
        target = current * 0.55
        if abs(target - current) < 0.05:
            target = current + 0.25
        applied = require_ok(invoke(base, "plugin_grabber.apply_reverb_controls", {
            "track_id": track_id, "plugin_id": plugin_id,
            "controls": [{"control_ref": binding.get("control_ref"), "display_value": target}],
        }, timeout), "apply_reverb_controls")
        if str(applied.get("status")) not in {"exact", "quantized"}:
            raise RuntimeError(f"apply status={applied.get('status')}")
        after = kernel_command({"cmd": "get_plugin_parameters", "track_id": track_id,
                                "plugin_id": plugin_id, "include_parameters": True}, timeout)
        param_id = str(binding.get("param_id"))

        def find_param(payload: dict[str, Any]) -> dict[str, Any]:
            for row in payload.get("parameters") or []:
                if isinstance(row, dict) and str(row.get("id") or row.get("param_id")) == param_id:
                    return row
            return {}

        before_row, after_row = find_param(before), find_param(after)
        result["readback"] = {
            "display_target": target,
            "apply_status": applied.get("status"),
            "actual_readback": (applied.get("controls") or [{}])[0].get("actual_readback"),
            "before": {"value": before_row.get("value"), "value_text": before_row.get("value_text"),
                       "normalized": before_row.get("normalized_value")},
            "after": {"value": after_row.get("value"), "value_text": after_row.get("value_text"),
                      "normalized": after_row.get("normalized_value")},
        }
        (output_dir / f"{case_id}_spot.json").write_text(json.dumps(result, ensure_ascii=False, indent=2),
                                                         encoding="utf-8")
        before_key = (str(before_row.get("value_text", "")), str(before_row.get("normalized_value", "")))
        after_key = (str(after_row.get("value_text", "")), str(after_row.get("normalized_value", "")))
        if before_key == after_key:
            raise RuntimeError(f"parameter readback unchanged: {before_key}")
        result["status"] = "write_readback_changed"
        return result
    finally:
        if track_id:
            try:
                require_ok(invoke(base, "track.delete", {"track_id": track_id}, timeout, True), "track.delete")
            except Exception:
                pass


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", required=True, type=Path)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--skip-spot", action="store_true")
    args = parser.parse_args()

    output_dir = args.output_dir
    output_dir.mkdir(parents=True, exist_ok=True)
    targets, out_of_scope, all_identifiers = discover(args.timeout_sec)
    (output_dir / "discovery.json").write_text(json.dumps({
        "queries": DISCOVERY_QUERIES,
        "unique_identifiers": len(all_identifiers),
        "targets": targets,
        "out_of_scope": out_of_scope,
    }, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"discovery: {len(targets)} targets, {len(out_of_scope)} out-of-scope, "
          f"{len(all_identifiers)} unique identifiers", flush=True)

    matrix: list[dict[str, Any]] = []
    failures: list[dict[str, str]] = []
    for index, case in enumerate(targets, 1):
        case_id = case_id_for(case["plugin_name"])
        print(f"[{index}/{len(targets)}] {case_id}: {case['plugin_name']}", flush=True)
        try:
            row = capture_case(args.agent_http, case, args.timeout_sec, output_dir)
            matrix.append(row)
            expected = row["expected_family"]
            if row["expected_family_resolved"]:
                print(f"  {expected} resolved: {row['inspects'][expected]['classification']} "
                      f"evidence={row['inspects'][expected]['tail_evidence']}", flush=True)
            else:
                print(f"  {expected} rejected: {row['inspects'][expected]['rejection_code']}", flush=True)
        except Exception as error:
            failures.append({"id": case_id, "error": str(error)})
            matrix.append({"id": case_id, "plugin_name": case["plugin_name"],
                           "status": "capture_failed", "error": str(error)})
            print(f"  ERROR: {error}", flush=True)

    spot_results: list[dict[str, Any]] = []
    if not args.skip_spot:
        spot_specs = [
            ("abbey_road_plates", "plate_damper_family", 0.90, "Abbey Road Plates"),
            ("fabfilter_pro_r", "direct_wording", 0.96, "Pro-R"),
        ]
        for case_id, evidence, confidence, needle in spot_specs:
            candidates = [case for case in targets if case_id in case_id_for(case["plugin_name"])]
            if not candidates:
                # fall back to a discovery-wide name needle for shell name quirks
                candidates = [case for case in targets if needle.lower() in case["plugin_name"].lower()]
            if not candidates:
                failures.append({"id": case_id, "error": "spot target not discovered"})
                continue
            print(f"spot {case_id}: {candidates[0]['plugin_name']}", flush=True)
            try:
                spot_results.append(spot_case(args.agent_http, candidates[0], args.timeout_sec,
                                              output_dir, case_id, evidence, confidence))
                print(f"  write readback changed", flush=True)
            except Exception as error:
                failures.append({"id": case_id, "error": str(error)})
                spot_results.append({"case_id": case_id, "status": "spot_failed", "error": str(error)})
                print(f"  SPOT ERROR: {error}", flush=True)

    report = {
        "schema_version": "plugin_grabber.timefx_compat_matrix_report.v1",
        "card": "PLUGGRABBER-TIMEDYN-COMPAT-1",
        "target_count": len(targets),
        "out_of_scope": out_of_scope,
        "matrix": matrix,
        "spot": spot_results,
        "failures": failures,
        "status": "failed" if failures else "ok",
    }
    (output_dir / "matrix_summary.json").write_text(json.dumps(report, ensure_ascii=False, indent=2),
                                                    encoding="utf-8")
    print(f"report: {output_dir / 'matrix_summary.json'}", flush=True)
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
