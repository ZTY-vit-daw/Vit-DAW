"""VSTI-PARAM-PROBE-1: instrument parameter control probe (deterministic, zero LLM).

Real-stack driver started by dev_agent_smoke.ps1 -VstiParamProbeSmoke
(run with -StartKernel -RestartAgent; no UI). For each configured instrument
(Grand Rhapsody Piano Stereo primary + Surge XT control):

  A. precheck: kernel plugin_list_available must carry both probe subjects
     with is_instrument (empty/missing inventory is an environment hand-over,
     exit 2); pcactl fingerprint (processorattestation.FingerprintPath,
     same source tree build) recorded per plugin path;
  B. kernel ZMQ load leg: add_track -> rack_add_node(plugin_identifier) ->
     wait for a non-empty get_plugin_parameters surface; instance face
     (plugin_instance_ready / plugin_load_state) asserted (seg3a-r2 shape);
  C. inventory leg: full parameter rows (range / load-time default value /
     enum labels via display_probe.discrete_labels) dumped per plugin +
     preset/program/MIDI-channel name scan (card diff-observation material);
  D. kernel roundtrip leg: 3 continuous writable params apply
     (normalized_value) -> kernel reply readback -> independent surface
     readback -> restore -> full-snapshot compare (certification-line
     tolerance 1e-4 on normalized values);
  E. agent HTTP control-face leg: /agent/authority full_project_access
     switch (261 G5 / seg3a precedent) -> plugin.get_parameters (param-guard
     cache prime) -> plugin.set_parameter write + kernel readback + restore
     -> authority restore;
  F. cleanup leg: delete_track (self-contained; retried once).

Card stop condition (get_plugin_parameters empty/error for the instrument ->
param interface unusable for VSTi) maps to exit 4 with raw evidence; the run
then stops without rerun (RUNBOOK section 8 discipline).

Exit codes: 0 PASS / 2 environment interruption / 4 deterministic defect or
card stop condition. No LLM legs in this driver (billing expectation 0 rows).
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

import zmq

EXIT_PASS = 0
EXIT_ENV = 2
EXIT_DEFECT = 4

APPLY_TOLERANCE = 5e-3        # apply/readback tolerance for fresh writes
SNAPSHOT_TOLERANCE = 1e-4     # restore + full-snapshot compare (cert line default)
PARAM_WAIT_SECONDS = 90.0     # Waves shell members can take a while to surface

# Probe subjects (card: Grand Rhapsody Piano Stereo primary + Surge XT control).
# Identifiers are the installed-plugin identity strings the kernel resolves in
# rack_add_node (plugin_identifier wins over shell path ambiguity); plugin_path
# is the pluginsemantics index path fingerprinted by pcactl.
PLUGINS: list[dict[str, str]] = [
    {
        "key": "grand_rhapsody",
        "label": "Grand Rhapsody Piano Stereo",
        "identifier": "VST3-Grand Rhapsody Piano Stereo-695648d-a6dc4e7d",
        "plugin_path": r"C:\Program Files\Common Files\VST3\WaveShell1-VST3 17.1_x64.vst3",
    },
    {
        "key": "surge_xt",
        "label": "Surge XT",
        "identifier": "VST3-Surge XT-1e1bcfb-190e4fbd",
        "plugin_path": r"C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3",
    },
]

# Card diff-observation: instrument parameter face vs effecter families.
DIFF_SCAN_TERMS = ("preset", "program", "patch", "bank", "midi", "channel",
                   "scene", "category", "voice", "tone", "kit")
DIFF_SCAN_TERM_CAP = 12


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    headers = {"Content-Type": "application/json"} if data is not None else {}
    request = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", "replace")
        raise RuntimeError(f"HTTP {exc.code} from {method} {url}: {body[:400]}") from exc


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float, confirmed: bool = False) -> dict[str, Any]:
    return request_json("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool, "args": args, "confirmed": confirmed,
        "source": "vsti_param_probe_smoke",
    }, timeout)


def result_of(reply: dict[str, Any]) -> dict[str, Any]:
    value = reply.get("result") if isinstance(reply.get("result"), dict) else {}
    return value if isinstance(value, dict) else {}


def require_ok(reply: dict[str, Any], label: str) -> dict[str, Any]:
    status = str(reply.get("status", "")).lower()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{label} failed: status={reply.get('status')!r} error={str(reply.get('error'))[:400]}")
    return result_of(reply)


class KernelZmq:
    def __init__(self, url: str, timeout_ms: int = 60000) -> None:
        self.url = url
        self.timeout_ms = timeout_ms
        self.context = zmq.Context.instance()
        self.socket = self.context.socket(zmq.REQ)
        self.socket.setsockopt(zmq.RCVTIMEO, timeout_ms)
        self.socket.setsockopt(zmq.LINGER, 0)
        self.socket.connect(url)

    def command(self, payload: dict[str, Any]) -> dict[str, Any]:
        body = dict(payload)
        body.setdefault("command_timeout_ms", self.timeout_ms)
        self.socket.send_json(body)
        reply = self.socket.recv_json()
        if not isinstance(reply, dict):
            raise RuntimeError("kernel returned non-object JSON")
        return reply

    def close(self) -> None:
        self.socket.close()


def wait_kernel(url: str, seconds: float) -> None:
    deadline = time.time() + seconds
    last = "no attempt"
    probe = None
    while time.time() < deadline:
        try:
            if probe is None:
                probe = KernelZmq(url, timeout_ms=5000)
            reply = probe.command({"cmd": "get_project_state"})
            last = str(reply.get("status", ""))[:120]
            if str(reply.get("status", "")) in {"ok", "success"}:
                return
        except Exception as exc:  # noqa: BLE001 - startup poll.
            last = str(exc)[:120]
        time.sleep(0.5)
    raise RuntimeError(f"kernel ZMQ face not ready at {url}: {last}")


def wait_agent_http(base: str, seconds: float) -> None:
    deadline = time.time() + seconds
    last = "no attempt"
    while time.time() < deadline:
        try:
            body = request_json("GET", base.rstrip("/") + "/health", None, 5.0)
            last = str(body)[:120]
            if body.get("status") in {"ok", "healthy"} or "status" not in body:
                return
        except Exception as exc:  # noqa: BLE001 - startup poll.
            last = str(exc)[:120]
        time.sleep(0.5)
    raise RuntimeError(f"agent HTTP face not ready at {base}: {last}")


def finite_float(value: Any) -> float | None:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return None
    number = float(value)
    return number if number == number and number not in (float("inf"), float("-inf")) else None


def parameter_rows(surface: dict[str, Any]) -> list[dict[str, Any]]:
    rows = surface.get("parameters")
    if not isinstance(rows, list):
        return []
    return [row for row in rows if isinstance(row, dict)]


def snapshot_mismatches(before: dict[str, float], after: dict[str, float],
                        tolerance: float) -> list[str]:
    mismatches = []
    for key in sorted(set(before) | set(after)):
        if key not in before or key not in after:
            mismatches.append(f"{key}:missing")
            continue
        if abs(before[key] - after[key]) > tolerance:
            mismatches.append(f"{key}:{before[key]:.6f}->{after[key]:.6f}")
    return mismatches


def normalized_snapshot(surface: dict[str, Any]) -> dict[str, float]:
    out: dict[str, float] = {}
    for row in parameter_rows(surface):
        param_id = str(row.get("param_id") or row.get("id") or "")
        value = finite_float(row.get("normalized_value"))
        if param_id and value is not None:
            out[param_id] = value
    return out


def pick_roundtrip_targets(surface: dict[str, Any]) -> list[dict[str, Any]]:
    """Three continuous, hosted, writable params (deterministic id order)."""
    candidates: list[dict[str, Any]] = []
    for row in parameter_rows(surface):
        param_id = str(row.get("param_id") or row.get("id") or "")
        if not param_id or row.get("is_discrete") or row.get("is_boolean"):
            continue
        if str(row.get("source") or "") == "tracktion_wrapper":
            continue
        low = finite_float(row.get("min"))
        high = finite_float(row.get("max"))
        normalized = finite_float(row.get("normalized_value"))
        if low is None or high is None or normalized is None or not high > low:
            continue
        candidates.append({
            "param_id": param_id, "name": str(row.get("name") or row.get("raw_param_name") or ""),
            "min": low, "max": high, "load_normalized": normalized,
            "load_value": finite_float(row.get("value")),
            "load_value_text": str(row.get("value_text") or ""),
        })
    candidates.sort(key=lambda item: item["param_id"])
    return candidates[:3]


def write_target_normalized(current: float) -> float:
    return 0.72 if current < 0.5 else 0.28


def surface_for(kernel: KernelZmq, track_id: str, plugin_id: str) -> dict[str, Any]:
    return kernel.command({"cmd": "get_plugin_parameters", "track_id": track_id,
                           "plugin_id": plugin_id, "include_parameters": True})


def wait_parameter_surface(kernel: KernelZmq, track_id: str, plugin_id: str,
                           label: str) -> dict[str, Any]:
    deadline = time.time() + PARAM_WAIT_SECONDS
    surface: dict[str, Any] = {}
    while time.time() < deadline:
        surface = surface_for(kernel, track_id, plugin_id)
        rows = parameter_rows(surface)
        if str(surface.get("status", "")) == "ok" and rows:
            return surface
        time.sleep(1.0)
    raise RuntimeError(
        f"get_plugin_parameters unusable for {label} (track={track_id} plugin={plugin_id}): "
        f"card stop condition (param interface not available for VSTi); last reply: "
        f"{json.dumps(surface, ensure_ascii=False)[:400]}")


def parse_load_state(state_text: str) -> dict[str, str]:
    fields: dict[str, str] = {}
    for token in str(state_text or "").replace(";", " ").split():
        if "=" in token:
            key, _, value = token.partition("=")
            fields[key.strip()] = value.strip()
    return fields


def scan_diff_observations(rows: list[dict[str, Any]]) -> dict[str, Any]:
    hits: dict[str, list[dict[str, Any]]] = {}
    hit_counts: dict[str, int] = {}
    discrete = []
    for row in rows:
        name = str(row.get("name") or row.get("raw_param_name") or "")
        lowered = name.lower()
        matched = [term for term in DIFF_SCAN_TERMS if term in lowered]
        if matched:
            group = "+".join(matched)
            hit_counts[group] = hit_counts.get(group, 0) + 1
            if len(hits.setdefault(group, [])) < DIFF_SCAN_TERM_CAP:
                hits[group].append({
                    "param_id": row.get("param_id") or row.get("id"),
                    "name": name,
                    "is_discrete": bool(row.get("is_discrete")),
                    "num_steps": row.get("num_steps"),
                    "value_text": row.get("value_text"),
                })
        if row.get("is_discrete") and isinstance(row.get("num_steps"), (int, float)):
            probe = row.get("display_probe") if isinstance(row.get("display_probe"), dict) else {}
            labels = probe.get("discrete_labels") or probe.get("all_labels")
            discrete.append({
                "param_id": row.get("param_id") or row.get("id"),
                "name": name,
                "num_steps": row.get("num_steps"),
                "enum_labels_sample": (labels[:8] if isinstance(labels, list) else []),
            })
    return {
        "name_term_hit_counts": hit_counts,
        "name_term_hits": hits,
        "discrete_enum_params": discrete[:40],
        "discrete_enum_param_count": len(discrete),
    }


def pcactl_fingerprint(agent_dir: Path, plugin_path: str, timeout: float) -> dict[str, Any]:
    """processorattestation.FingerprintPath via the same-source pcactl build.

    Read-only: `pcactl fingerprint` only hashes the binary/bundle and prints
    JSON; it never opens the attestation store (admission-deferred ruling).
    Prefers the pcactl.exe built by the ps1 leg from this worktree, falls
    back to `go run` from the same source tree.
    """
    binary = agent_dir / "bin" / "pcactl.exe"
    if binary.is_file():
        command = [str(binary), "fingerprint", "-path", plugin_path]
        cwd = None
    else:
        command = ["go", "run", "./cmd/pcactl", "fingerprint", "-path", plugin_path]
        cwd = str(agent_dir)
    completed = subprocess.run(command, cwd=cwd, capture_output=True, text=True,
                               timeout=timeout, shell=False)
    if completed.returncode != 0:
        raise RuntimeError(
            f"pcactl fingerprint failed for {plugin_path}: exit={completed.returncode} "
            f"stderr={completed.stderr[:300]}")
    try:
        # pcactl prints one pretty-printed (multi-line) JSON object on stdout.
        payload = json.loads(completed.stdout.strip())
    except ValueError as exc:
        raise RuntimeError(f"pcactl fingerprint output not JSON: {completed.stdout[:200]}") from exc
    if not str(payload.get("binary_fingerprint", "")).startswith("sha256:"):
        raise RuntimeError(f"pcactl fingerprint missing sha256 field: {payload}")
    return payload


def authority_get(base: str) -> dict[str, Any]:
    body = request_json("GET", base.rstrip("/") + "/agent/authority", None, 30.0)
    if "authority_mode" not in body:
        raise RuntimeError(f"GET /agent/authority malformed: {str(body)[:300]}")
    return body


def authority_post(base: str, mode: str) -> dict[str, Any]:
    body = request_json("POST", base.rstrip("/") + "/agent/authority",
                        {"authority_mode": mode}, 30.0)
    if str(body.get("status", "")) != "ok" or body.get("authority_mode") != mode:
        raise RuntimeError(f"POST /agent/authority {mode} rejected: {str(body)[:400]}")
    return body


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--repo-root", required=True, type=Path)
    parser.add_argument("--artifacts-dir", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    args = parser.parse_args()

    args.artifacts_dir.mkdir(parents=True, exist_ok=True)
    summary: dict[str, Any] = {
        "card": "VSTI-PARAM-PROBE-1",
        "driver": "vsti_param_probe_smoke.py",
        "plugins": [],
        "gates": [],
        "phases": [],
        "llm_legs": 0,
    }

    def gate(name: str, passed: bool, detail: str) -> bool:
        summary["gates"].append({"gate": name, "pass": bool(passed), "detail": detail})
        print(f"[vsti-param-probe] gate {name}: {'PASS' if passed else 'FAIL'} — {detail}", flush=True)
        return bool(passed)

    def save_summary() -> None:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    def fail(exit_code: int, phase: str, reason: str, extra: dict[str, Any] | None = None) -> int:
        summary["fail_phase"] = phase
        summary["fail_reason"] = reason
        if extra:
            summary.setdefault("phases", []).append({phase: extra})
        save_summary()
        print(f"[vsti-param-probe] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
        return exit_code

    kernel: KernelZmq | None = None
    authority_baseline: dict[str, Any] | None = None
    authority_switched = False
    try:
        wait_kernel(args.kernel_zmq, 60.0)
        wait_agent_http(args.agent_http, 90.0)
        kernel = KernelZmq(args.kernel_zmq)
        summary["phases"].append("stack-ready")

        # ---- phase A: inventory precheck + fingerprints (zero LLM) --------
        listing = kernel.command({"cmd": "plugin_list_available", "limit": 2000})
        if str(listing.get("status", "")) != "ok":
            return fail(EXIT_ENV, "precheck",
                        f"plugin_list_available failed: {json.dumps(listing, ensure_ascii=False)[:400]}")
        rows = [row for row in (listing.get("plugins") or []) if isinstance(row, dict)]
        instrument_rows = [row for row in rows if bool(row.get("is_instrument"))]
        summary["plugin_count"] = len(rows)
        summary["instrument_count"] = len(instrument_rows)
        save_summary()
        if not gate("A.inventory_nonempty", bool(instrument_rows),
                    f"plugin_list_available: {len(rows)} plugins, {len(instrument_rows)} instruments"):
            return fail(EXIT_ENV, "precheck", "is_instrument count == 0 (environment precondition)")

        for plugin in PLUGINS:
            known = any(str(row.get("identifier") or "") == plugin["identifier"] for row in instrument_rows)
            if not gate(f"A.subject_present.{plugin['key']}", known,
                        f"{plugin['label']} identifier present with is_instrument"):
                return fail(EXIT_ENV, "precheck",
                            f"probe subject missing from kernel inventory: {plugin['identifier']}")
        for plugin in PLUGINS:
            try:
                payload = pcactl_fingerprint(args.repo_root / "agent", plugin["plugin_path"], 180.0)
            except Exception as exc:  # noqa: BLE001 - fingerprint failure is env-class.
                return fail(EXIT_ENV, "fingerprint", f"fingerprint {plugin['key']}: {exc}")
            plugin["fingerprint"] = payload["binary_fingerprint"]
            plugin["fingerprint_path"] = payload.get("path", plugin["plugin_path"])
        summary["fingerprints"] = {p["key"]: p["fingerprint"] for p in PLUGINS}
        save_summary()

        # ---- phases B-F per plugin ----------------------------------------
        for plugin in PLUGINS:
            record: dict[str, Any] = {
                "key": plugin["key"], "label": plugin["label"],
                "identifier": plugin["identifier"],
                "binary_fingerprint": plugin["fingerprint"],
            }
            summary["plugins"].append(record)
            track_id = ""
            try:
                added = kernel.command({"cmd": "add_track"})
                if not gate(f"B.add_track.{plugin['key']}", str(added.get("status")) == "ok",
                            f"add_track reply: {str(added)[:200]}"):
                    return fail(EXIT_DEFECT, "load", f"add_track failed: {added}")
                track_id = str(added.get("track_id") or "")
                if not track_id:
                    return fail(EXIT_DEFECT, "load", f"add_track returned no track_id: {added}")

                node = kernel.command({"cmd": "rack_add_node", "track_id": track_id,
                                       "plugin_identifier": plugin["identifier"],
                                       "auto_connect": True, "x": 0.35, "y": 0.35})
                load_state_text = str(node.get("plugin_load_state") or "")
                load_fields = parse_load_state(load_state_text)
                record["load"] = {
                    "rack_add_node_status": node.get("status"),
                    "plugin_id": node.get("plugin_id"),
                    "plugin_name": node.get("plugin_name"),
                    "resolved_identifier": node.get("plugin_identifier"),
                    "zone_id": node.get("zone_id"),
                    "plugin_instance_ready": node.get("plugin_instance_ready"),
                    "plugin_load_state": load_state_text,
                    "graph_last_diff_kind": node.get("graph_last_diff_kind"),
                    "graph_last_diff_summary": node.get("graph_last_diff_summary"),
                }
                if not gate(f"B.rack_add_node.{plugin['key']}", str(node.get("status")) == "ok",
                            f"rack_add_node: {str(node)[:300]}"):
                    return fail(EXIT_DEFECT, "load", f"rack_add_node failed: {str(node)[:400]}")
                plugin_id = str(node.get("plugin_id") or node.get("plugin_item_id") or "")
                if not plugin_id:
                    return fail(EXIT_DEFECT, "load", f"rack_add_node returned no plugin_id: {node}")
                if not gate(f"B.instance_face.{plugin['key']}",
                            node.get("plugin_instance_ready") is not False
                            and load_fields.get("enabled", "true").lower() == "true",
                            f"instance_ready={node.get('plugin_instance_ready')} load_state={load_state_text}"):
                    return fail(EXIT_DEFECT, "load",
                                f"instrument instance not ready: {load_state_text}")

                try:
                    surface = wait_parameter_surface(kernel, track_id, plugin_id, plugin["label"])
                except RuntimeError as exc:
                    gate(f"C.parameters_available.{plugin['key']}", False, str(exc)[:400])
                    return fail(EXIT_DEFECT, "parameters", str(exc))
                gate(f"C.parameters_available.{plugin['key']}", True,
                     f"parameter surface rows={len(parameter_rows(surface))}")

                rows_plain = parameter_rows(surface)
                record["parameter_count"] = len(rows_plain)
                record["surface_signature"] = surface.get("current_param_signature_hash")
                record["recommended_groups"] = surface.get("recommended_groups")
                record["quick_controls"] = surface.get("quick_controls")
                record["template_role"] = surface.get("template_role")
                record["inventory_diff_observations"] = scan_diff_observations(rows_plain)
                inventory_path = args.artifacts_dir / f"{plugin['key']}_parameters.json"
                inventory_path.write_text(json.dumps({
                    "plugin": {k: plugin[k] for k in ("key", "label", "identifier", "plugin_path", "fingerprint")},
                    "track_id": track_id, "plugin_id": plugin_id,
                    "surface": surface,
                }, ensure_ascii=False, indent=2), encoding="utf-8")
                record["inventory_artifact"] = str(inventory_path)

                # ---- phase D: kernel roundtrip (3 continuous params) -------
                targets = pick_roundtrip_targets(surface)
                record["roundtrip_targets"] = targets
                if not gate(f"D.targets_selected.{plugin['key']}", len(targets) >= 2,
                            f"continuous writable targets: {[t['param_id'] for t in targets]}"):
                    return fail(EXIT_DEFECT, "roundtrip",
                                f"fewer than 2 continuous writable params surfaced: {len(targets)}")
                snapshot_before = normalized_snapshot(surface)
                roundtrip_rows: list[dict[str, Any]] = []
                for target in targets:
                    param_id = target["param_id"]
                    want = write_target_normalized(target["load_normalized"])
                    applied = kernel.command({"cmd": "set_plugin_param", "track_id": track_id,
                                              "plugin_id": plugin_id, "param_id": param_id,
                                              "normalized_value": want})
                    reply_norm = finite_float(applied.get("new_normalised_value"))
                    ok_apply = (str(applied.get("status")) == "ok" and reply_norm is not None
                                and abs(reply_norm - want) <= APPLY_TOLERANCE)
                    after = surface_for(kernel, track_id, plugin_id)
                    after_row = next((row for row in parameter_rows(after)
                                      if str(row.get("param_id") or row.get("id")) == param_id), {})
                    surface_norm = finite_float(after_row.get("normalized_value"))
                    ok_surface = surface_norm is not None and abs(surface_norm - want) <= APPLY_TOLERANCE
                    restored = kernel.command({"cmd": "set_plugin_param", "track_id": track_id,
                                               "plugin_id": plugin_id, "param_id": param_id,
                                               "normalized_value": target["load_normalized"]})
                    restore_after = surface_for(kernel, track_id, plugin_id)
                    restore_row = next((row for row in parameter_rows(restore_after)
                                        if str(row.get("param_id") or row.get("id")) == param_id), {})
                    restore_norm = finite_float(restore_row.get("normalized_value"))
                    ok_restore = (str(restored.get("status")) == "ok" and restore_norm is not None
                                  and abs(restore_norm - target["load_normalized"]) <= SNAPSHOT_TOLERANCE)
                    row = {
                        "param_id": param_id, "name": target["name"],
                        "range": [target["min"], target["max"]],
                        "load_default_normalized": target["load_normalized"],
                        "load_default_value_text": target["load_value_text"],
                        "applied_target_normalized": want,
                        "apply_reply": {k: applied.get(k) for k in
                                        ("status", "new_value", "new_normalised_value", "new_value_text",
                                         "value_interpretation")},
                        "apply_readback_normalized": surface_norm,
                        "after_value_text": after_row.get("value_text"),
                        "restore_reply": {k: restored.get(k) for k in
                                          ("status", "new_normalised_value", "new_value_text")},
                        "restore_readback_normalized": restore_norm,
                    }
                    roundtrip_rows.append(row)
                    if not (gate(f"D.apply.{plugin['key']}.{param_id}", ok_apply,
                                 f"apply reply normalized={reply_norm} want={want}")
                            and gate(f"D.readback.{plugin['key']}.{param_id}", ok_surface,
                                     f"surface readback normalized={surface_norm} want={want}")
                            and gate(f"D.restore.{plugin['key']}.{param_id}", ok_restore,
                                     f"restore readback={restore_norm} baseline={target['load_normalized']}")):
                        record["roundtrip"] = roundtrip_rows
                        return fail(EXIT_DEFECT, "roundtrip",
                                    f"roundtrip leg failed at {param_id}: {json.dumps(row, ensure_ascii=False)[:400]}")
                record["roundtrip"] = roundtrip_rows

                final_surface = surface_for(kernel, track_id, plugin_id)
                snapshot_after = normalized_snapshot(final_surface)
                mismatches = snapshot_mismatches(snapshot_before, snapshot_after, SNAPSHOT_TOLERANCE)
                record["full_snapshot_restored"] = not mismatches
                record["full_snapshot_param_count"] = len(snapshot_before)
                if not gate(f"D.full_snapshot.{plugin['key']}", not mismatches,
                            f"snapshot params={len(snapshot_before)} mismatches={mismatches[:10]}"):
                    return fail(EXIT_DEFECT, "roundtrip",
                                f"full snapshot restore failed: {mismatches[:20]}")

                # ---- phase E: agent HTTP control face ---------------------
                if authority_baseline is None:
                    authority_baseline = authority_get(args.agent_http)
                    summary["authority_baseline_mode"] = authority_baseline.get("authority_mode")
                authority_post(args.agent_http, "full_project_access")
                authority_switched = True
                verify = authority_get(args.agent_http)
                if not gate(f"E.authority.{plugin['key']}",
                            str(verify.get("authority_mode")) == "full_project_access",
                            f"authority mode now {verify.get('authority_mode')}"):
                    return fail(EXIT_ENV, "agent_face", "authority switch rejected")
                agent_get = invoke(args.agent_http, "plugin.get_parameters",
                                   {"track_id": track_id, "plugin_id": plugin_id,
                                    "include_parameters": True}, args.timeout_sec)
                agent_rows = parameter_rows(result_of(agent_get))
                if not gate(f"E.agent_get_parameters.{plugin['key']}",
                            str(agent_get.get("status", "")).lower() in {"ok", "success", "completed"}
                            and bool(agent_rows),
                            f"agent-face get rows={len(agent_rows)} status={agent_get.get('status')!r}"):
                    return fail(EXIT_DEFECT, "agent_face",
                                f"plugin.get_parameters over agent face failed: {str(agent_get)[:400]}")

                target = targets[0]
                param_id = target["param_id"]
                want = 0.65 if target["load_normalized"] < 0.5 else 0.35
                range_is_unit = abs(target["min"]) <= 1e-9 and abs(target["max"] - 1.0) <= 1e-9
                before_write = surface_for(kernel, track_id, plugin_id)
                before_row = next((row for row in parameter_rows(before_write)
                                   if str(row.get("param_id") or row.get("id")) == param_id), {})
                before_norm = finite_float(before_row.get("normalized_value"))
                write_reply: dict[str, Any] = {}
                write_invocation_error = ""
                try:
                    write_reply = invoke(args.agent_http, "set_plugin_param",
                                         {"track_id": track_id, "plugin_id": plugin_id,
                                          "param_id": param_id, "value": want},
                                         args.timeout_sec, confirmed=True)
                except RuntimeError as exc:
                    # HTTP 400 rejections (typed-family guards etc.) are
                    # agent-face findings: record and continue so the other
                    # plugin's kernel-face evidence is still collected.
                    write_invocation_error = str(exc)[:600]
                after_write = surface_for(kernel, track_id, plugin_id)
                after_row = next((row for row in parameter_rows(after_write)
                                  if str(row.get("param_id") or row.get("id")) == param_id), {})
                after_norm = finite_float(after_row.get("normalized_value"))
                ok_write_status = (not write_invocation_error
                                   and str(write_reply.get("status", "")).lower() in {"ok", "success", "completed"})
                ok_changed = (before_norm is not None and after_norm is not None
                              and abs(after_norm - before_norm) > APPLY_TOLERANCE)
                ok_contract = (after_norm is not None and abs(after_norm - want) <= APPLY_TOLERANCE) if range_is_unit else None
                restore_reply: dict[str, Any] = {}
                restore_invocation_error = ""
                restore_norm = None
                if ok_write_status:
                    try:
                        restore_reply = invoke(args.agent_http, "set_plugin_param",
                                               {"track_id": track_id, "plugin_id": plugin_id,
                                                "param_id": param_id, "value": target["load_normalized"]},
                                               args.timeout_sec, confirmed=True)
                    except RuntimeError as exc:
                        restore_invocation_error = str(exc)[:400]
                    restore_surface = surface_for(kernel, track_id, plugin_id)
                    restore_row = next((row for row in parameter_rows(restore_surface)
                                        if str(row.get("param_id") or row.get("id")) == param_id), {})
                    restore_norm = finite_float(restore_row.get("normalized_value"))
                ok_agent_restore = (ok_write_status and not restore_invocation_error
                                    and str(restore_reply.get("status", "")).lower() in {"ok", "success", "completed"}
                                    and restore_norm is not None
                                    and abs(restore_norm - target["load_normalized"]) <= SNAPSHOT_TOLERANCE)
                # Param-independence probe for guard-class rejections: retry a
                # second target once so "plugin-level block" vs "param-specific
                # ownership" is evidenced, not assumed.
                guard_retry = None
                if not ok_write_status and len(targets) > 1:
                    second = targets[1]
                    try:
                        guard_retry = invoke(args.agent_http, "set_plugin_param",
                                             {"track_id": track_id, "plugin_id": plugin_id,
                                              "param_id": second["param_id"], "value": 0.6},
                                             args.timeout_sec, confirmed=True)
                    except RuntimeError as exc:
                        guard_retry = {"invocation_error": str(exc)[:600]}
                record["agent_face"] = {
                    "param_id": param_id,
                    "range": [target["min"], target["max"]],
                    "range_is_unit_interval": range_is_unit,
                    "declared_contract": "catalog: value = normalized 0.0-1.0",
                    "before_normalized": before_norm,
                    "written_value": want,
                    "write_status": write_reply.get("status") if not write_invocation_error else "rejected",
                    "write_error": write_reply.get("error") or write_invocation_error,
                    "write_rejection_result": write_reply.get("result") if write_invocation_error else None,
                    "after_kernel_normalized": after_norm,
                    "contract_level_pass": ok_contract,
                    "restore_status": (restore_reply.get("status") if not restore_invocation_error
                                       else ("skipped" if not ok_write_status else "rejected")),
                    "restore_kernel_normalized": restore_norm,
                    "guard_param_independence_retry": guard_retry,
                }
                if not ok_write_status:
                    # Deterministic agent-face finding (e.g. typed-family
                    # fail-closed guard on an instrument): record, keep the
                    # run going for the remaining plugin, exit 4 at the end.
                    summary.setdefault("agent_face_findings", []).append({
                        "plugin": plugin["key"], "param_id": param_id,
                        "error": record["agent_face"]["write_error"],
                    })
                    gate(f"E.write_status.{plugin['key']}", False,
                         f"agent-face set rejected: {record['agent_face']['write_error'][:300]}")
                else:
                    gate(f"E.write_status.{plugin['key']}", True, "agent-face set accepted")
                    if not (gate(f"E.write_effect.{plugin['key']}", ok_changed,
                                 f"kernel readback moved {before_norm} -> {after_norm} (want {want})")
                            and gate(f"E.restore.{plugin['key']}", ok_agent_restore,
                                     f"agent-face restore readback={restore_norm} baseline={target['load_normalized']}")):
                        return fail(EXIT_DEFECT, "agent_face",
                                    f"agent-face control leg failed: {json.dumps(record['agent_face'], ensure_ascii=False)[:400]}")
                    if range_is_unit and not gate(f"E.normalized_contract.{plugin['key']}", bool(ok_contract),
                                                  f"unit-range param readback={after_norm} want={want}"):
                        return fail(EXIT_DEFECT, "agent_face",
                                    "agent-face set_plugin_param violated its declared normalized-value contract")

                post_e_surface = surface_for(kernel, track_id, plugin_id)
                post_e_mismatches = snapshot_mismatches(snapshot_before,
                                                        normalized_snapshot(post_e_surface),
                                                        SNAPSHOT_TOLERANCE)
                if not gate(f"E.full_snapshot.{plugin['key']}", not post_e_mismatches,
                            f"post-agent-leg snapshot mismatches={post_e_mismatches[:10]}"):
                    return fail(EXIT_DEFECT, "agent_face",
                                f"agent-face leg left the surface dirty: {post_e_mismatches[:20]}")

                record["status"] = ("captured_with_agent_face_finding"
                                    if summary.get("agent_face_findings") else "captured")
            finally:
                if track_id:
                    deleted = None
                    for attempt in (1, 2):
                        try:
                            deleted = kernel.command({"cmd": "delete_track", "track_id": track_id})
                        except Exception as exc:  # noqa: BLE001 - cleanup retry.
                            deleted = {"status": "error", "message": str(exc)[:200]}
                        if str(deleted.get("status")) == "ok":
                            break
                        time.sleep(1.0)
                    record["cleanup_delete_track_status"] = str((deleted or {}).get("status"))
            if record.get("cleanup_delete_track_status") != "ok":
                return fail(EXIT_DEFECT, "cleanup",
                            f"delete_track failed for {plugin['key']}: {record.get('cleanup_delete_track_status')}")
            save_summary()

        if summary.get("agent_face_findings"):
            save_summary()
            findings = summary["agent_face_findings"]
            print(f"[vsti-param-probe] FAIL exit=4 phase=agent_face: generic agent-face write "
                  f"rejected for {len(findings)} plugin(s) (deterministic finding, forensic "
                  f"hand-over, kernel-face evidence collected); summary: {args.output}", flush=True)
            summary["fail_phase"] = "agent_face"
            summary["fail_reason"] = (
                "generic set_plugin_param over the agent HTTP face rejected (typed-family "
                "fail-closed guard): VSTi not writable through the agent generic control face; "
                "kernel ZMQ face roundtrip evidence collected in the same summary")
            save_summary()
            return EXIT_DEFECT
        save_summary()
        print(f"[vsti-param-probe] PASS; summary: {args.output}", flush=True)
        return EXIT_PASS
    except Exception as exc:  # noqa: BLE001 - env-class catch for transport faces.
        return fail(EXIT_ENV, "exception", f"unexpected driver exception: {exc}")
    finally:
        if authority_switched and authority_baseline is not None:
            baseline_mode = str(authority_baseline.get("authority_mode") or "")
            try:
                if baseline_mode and baseline_mode != "full_project_access":
                    authority_post(args.agent_http, baseline_mode)
                restored = authority_get(args.agent_http)
                summary["authority_restored_mode"] = restored.get("authority_mode")
            except Exception as exc:  # noqa: BLE001 - best-effort restore, recorded.
                summary["authority_restore_error"] = str(exc)[:200]
        if kernel is not None:
            kernel.close()
        save_summary()


if __name__ == "__main__":
    sys.exit(main())
