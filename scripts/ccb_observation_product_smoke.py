from __future__ import annotations

import argparse
import json
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


FORBIDDEN_MUTATIONS = {
    "rack_add_node",
    "rack.load_plugin",
    "plugin.load_to_rack",
    "set_plugin_param",
    "plugin.set_parameter",
    "plugin_grabber_apply_compressor_controls",
    "plugin_grabber.apply_compressor_controls",
    "plugin_grabber_apply_eq_edits",
    "plugin_grabber.apply_eq_edits",
    "mix_apply_tick",
    "mix.apply_tick",
    "set_volume",
    "set_pan",
}

FORBIDDEN_DISCLOSURE = {
    "raw_pcm",
    "waveform_array",
    "spectrogram_tiles",
    "time_segments",
    "observation_path",
    "context_pack_path",
    "board_path",
}


def invoke(
    base: str, tool: str, args: dict[str, Any], timeout: float, confirmed: bool = False
) -> dict[str, Any]:
    payload = json.dumps(
        {"tool": tool, "args": args, "confirmed": confirmed, "source": "ccb_observation_product_smoke"}
    ).encode("utf-8")
    request = urllib.request.Request(
        base.rstrip("/") + "/agent/invoke",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {error.code} from {tool}: {body[:2000]}") from error
    parsed = json.loads(body)
    if str(parsed.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {parsed}")
    result = parsed.get("result", parsed)
    if not isinstance(result, dict):
        raise RuntimeError(f"{tool} returned a non-object result")
    return result


def project_binding(value: dict[str, Any]) -> dict[str, str]:
    project = value.get("project") if isinstance(value.get("project"), dict) else {}
    snapshot = value.get("snapshot") if isinstance(value.get("snapshot"), dict) else {}
    snapshot_project = snapshot.get("project") if isinstance(snapshot.get("project"), dict) else {}

    def first(*values: Any) -> str:
        for item in values:
            text = str(item).strip() if item is not None else ""
            if text and text != "<nil>":
                return text
        return ""

    return {
        "project_uuid": first(value.get("project_uuid"), value.get("project_id"), project.get("project_uuid"), snapshot_project.get("project_uuid")),
        "project_epoch": first(value.get("project_epoch"), project.get("project_epoch"), snapshot_project.get("project_epoch")),
        "project_revision": first(value.get("project_revision"), value.get("revision"), project.get("revision"), snapshot_project.get("revision")),
        "project_state_hash": first(value.get("snapshot_hash"), value.get("project_state_hash"), project.get("snapshot_hash"), snapshot.get("snapshot_hash")),
    }


def resolve_track_id(state: dict[str, Any], preferred: str) -> str:
    rows = state.get("tracks") if isinstance(state.get("tracks"), list) else []
    candidates = [row for row in rows if isinstance(row, dict)]

    def row_id(row: dict[str, Any]) -> str:
        return str(row.get("track_id") or row.get("id") or "").strip()

    for row in candidates:
        if row_id(row) == preferred:
            return preferred
    for row in candidates:
        clips = row.get("clips") or row.get("clip_summaries")
        if isinstance(clips, list) and clips and row_id(row):
            return row_id(row)
    for row in candidates:
        if row_id(row):
            return row_id(row)
    raise RuntimeError(f"fixture project has no user-visible target track: {state}")


def require_bundle(
    result: dict[str, Any], expected_view: str, allow_explicit_omission: bool = False
) -> dict[str, Any]:
    bundle = result.get("bundle")
    if not isinstance(bundle, dict):
        raise RuntimeError(f"CCB result lacks bundle: {result}")
    if bundle.get("schema_version") != "ccb_observation_bundle.v1":
        raise RuntimeError(f"unexpected bundle schema: {bundle}")
    if bundle.get("read_only") is not True or bundle.get("mutation_authority") is not False:
        raise RuntimeError(f"bundle authority boundary failed: {bundle}")
    views = bundle.get("views") if isinstance(bundle.get("views"), dict) else {}
    omissions = bundle.get("omissions") if isinstance(bundle.get("omissions"), dict) else {}
    if expected_view not in views:
        if not allow_explicit_omission or expected_view not in omissions:
            raise RuntimeError(f"bundle did not return required view {expected_view}: {bundle}")
    encoded = json.dumps(bundle, ensure_ascii=False).lower()
    leaked = sorted(key for key in FORBIDDEN_DISCLOSURE if f'"{key}"' in encoded)
    if leaked:
        raise RuntimeError(f"forbidden disclosure keys leaked: {leaked}")
    if int(bundle.get("disclosure_bytes", 0)) > int(bundle.get("max_disclosure_bytes", 0)):
        raise RuntimeError(f"bundle exceeded disclosure budget: {bundle}")
    return bundle


def scan_journal(path: Path) -> list[str]:
    if not path.exists():
        return []
    text = path.read_text(encoding="utf-8", errors="replace").lower()
    return sorted(name for name in FORBIDDEN_MUTATIONS if name.lower() in text)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--track-id", default="1007")
    parser.add_argument("--project-path", type=Path, required=True)
    parser.add_argument("--journal", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--timeout-sec", type=float, default=120.0)
    args = parser.parse_args()

    opened = invoke(
        args.agent_http,
        "project.open",
        {"file_path": str(args.project_path.resolve())},
        args.timeout_sec,
        confirmed=True,
    )
    if str(opened.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"fixture project did not open: {opened}")
    before = invoke(args.agent_http, "project.state", {}, args.timeout_sec)
    before_binding = project_binding(before)
    track_id = resolve_track_id(before, args.track_id)
    catalog_result = invoke(
        args.agent_http,
        "ccb.observation_catalog",
        {"target_kind": "track", "target_id": track_id},
        args.timeout_sec,
    )
    catalog = catalog_result.get("catalog")
    if not isinstance(catalog, dict) or catalog.get("schema_version") != "ccb_observation_catalog.v1":
        raise RuntimeError(f"unexpected CCB catalog: {catalog_result}")
    view_ids = {str(row.get("view_id", "")) for row in catalog.get("views", []) if isinstance(row, dict)}
    required = {"track.basic_energy", "track.time_dynamics", "processor.behavior", "mix.masking_relationship"}
    if not required.issubset(view_ids):
        raise RuntimeError(f"CCB catalog lacks views: {sorted(required - view_ids)}")

    session_id = "ccb_product_" + str(int(time.time() * 1000))
    first_result = invoke(
        args.agent_http,
        "ccb.observation_request",
        {
            "request_id": "ccb-product-1",
            "mix_session_id": session_id,
            "view_ids": ["track.basic_energy"],
            "target_kind": "track",
            "target_id": track_id,
            "max_disclosure_bytes": 8192,
        },
        args.timeout_sec,
    )
    first_bundle = require_bundle(first_result, "track.basic_energy")
    observation_id = str(first_bundle.get("observation_id", "")).strip()
    if not observation_id:
        raise RuntimeError(f"first CCB request lacks observation binding: {first_bundle}")

    second_result = invoke(
        args.agent_http,
        "ccb.observation_request",
        {
            "request_id": "ccb-product-2",
            "mix_session_id": session_id,
            "observation_id": observation_id,
            "view_ids": ["track.time_dynamics", "processor.behavior", "mix.masking_relationship"],
            "max_disclosure_bytes": 8192,
        },
        args.timeout_sec,
    )
    second_bundle = require_bundle(second_result, "track.time_dynamics", allow_explicit_omission=True)
    if second_bundle.get("observation_id") != observation_id:
        raise RuntimeError("repeated CCB request lost observation identity")
    if "mix.masking_relationship" not in (second_bundle.get("omissions") or {}):
        raise RuntimeError("deferred masking view was not explicitly omitted")

    after = invoke(args.agent_http, "project.state", {}, args.timeout_sec)
    after_binding = project_binding(after)
    compared = [key for key in before_binding if before_binding[key] and after_binding[key]]
    changed = {key: [before_binding[key], after_binding[key]] for key in compared if before_binding[key] != after_binding[key]}
    if changed:
        raise RuntimeError(f"project binding changed during read-only CCB smoke: {changed}")
    forbidden_journal_entries = scan_journal(args.journal)
    if forbidden_journal_entries:
        raise RuntimeError(f"mutation commands found in journal: {forbidden_journal_entries}")

    report = {
        "schema_version": "ccb_observation_product_smoke.v1",
        "status": "passed",
        "product_lifecycle": "godot_project",
        "setup_project_opened_before_baseline": str(args.project_path.resolve()),
        "ports": [5555, 5556, 7878, 8787],
        "before_project_binding": before_binding,
        "after_project_binding": after_binding,
        "compared_binding_fields": compared,
        "catalog_view_count": len(view_ids),
        "observation_id": observation_id,
        "target_track_id": track_id,
        "first_bundle_status": first_bundle.get("status"),
        "second_bundle_status": second_bundle.get("status"),
        "second_omissions": second_bundle.get("omissions", {}),
        "mutation_commands_in_journal": forbidden_journal_entries,
        "assertions": {
            "godot_kernel_hub_agent_ready": True,
            "repeat_request_same_observation": True,
            "project_binding_unchanged": True,
            "no_plugin_load": True,
            "no_parameter_write": True,
            "no_daw_mutation": True,
            "no_raw_payload": True,
        },
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
