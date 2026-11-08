#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-PROQ3-FREE-1 (card 320) persistence forensics probe.

Zero-LLM discriminator for the batch-2 device-gate persistence failure
(run proq3_free_20261108_233122: Pro-Q 3 'Band 1 Gain' 0.5->0.7 landed live,
then save_project -> open_project reverted it to 0.5).

Hypothesis from source reading (anchors in the report):
  - .vit save path flushes ONLY track-level state
    (VitHeadlessService.cpp ipcSaveEncryptedProjectToPath ~1316-1318:
    track->flushStateToValueTree() for all tracks; no Edit::flushState(),
    no edit-wide plugin flush);
  - Track::flushStateToValueTree cascades only to the TRACK's own pluginList
    (tracktion_Track.cpp ~49-53 + getAllPlugins = pluginList.getPlugins()
    ~277-280);
  - Pro-Q 3 lives INSIDE a rack ("New Rack" container, run-2/3 forensics):
    rack-internal plugins hang off the RackType's own track list, not the
    host track's pluginList -> their live chunk state is never pulled into
    the ValueTree before the XML snapshot -> reload restores defaults.

Discriminating arms (both zero-LLM, agent invoke face, on placed copy r1):
  A. rack-internal external plugin: Pro-Q 3 param 3 (Band 1 Gain) 0.5 -> 0.7
     -> save -> open -> readback. Expected under hypothesis: REVERTS.
  B. track-level built-in plugin: Volume & Pan plugin row, param "volume"
     normalized -> save -> open -> readback via get_project_state row
     db/pan + plugin row. Expected under hypothesis: PERSISTS.
  A reverts + B persists == save-flush gap for rack-internal plugins
  (product-side; hand-over, no fix here). Both revert == broader reload
  defect (also hand-over). Both persist == unexplained (hand-over raw).

The probe leaves r1 restored to its default state (both arms written back).
Exit 0 = probe completed (it records evidence; it does NOT pass/fail the
card — adjudication is the supervisor's).
"""
from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path
from typing import Any

_HERE = Path(__file__).resolve().parent
_REPO = _HERE.parents[2]
for _p in (str(_HERE), str(_REPO / "scripts"),
           str(_REPO / "coord" / "runs" / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "runner_frozen")):
    if _p not in sys.path:
        sys.path.insert(0, _p)

import plugin_control_eval_driver as frozen  # noqa: E402
from compressor_compat_matrix_smoke import (  # noqa: E402
    first_text, kernel_command, request_json, result_of,
)
import proq3_free_cases as cases  # noqa: E402
import proq3_free_driver as drv  # noqa: E402

invoke = frozen.invoke
param_rows = frozen.param_rows
row_by_id = frozen.row_by_id
_num = frozen._num


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--agent-http", default="http://127.0.0.1:7878")
    ap.add_argument("--project", required=True)
    ap.add_argument("--out", required=True, type=Path)
    ap.add_argument("--timeout", type=float, default=30.0)
    args = ap.parse_args()

    report: dict[str, Any] = {
        "schema_version": "proq3_persistence_forensics.v1",
        "card": cases.CARD,
        "project": args.project,
        "probed_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "anchors": {
            "save_path_vit": "VitApp/Source/Service/VitHeadlessService.cpp "
                             "ipcSaveEncryptedProjectToPath ~1316-1320 (track-level "
                             "flush only, no Edit::flushState / edit-wide plugin flush)",
            "save_path_default_xml": "VitHeadlessService.cpp "
                                     "saveProjectToDefaultXml ~671-675 (same shape)",
            "track_flush": "tracktion_engine .../model/tracks/tracktion_Track.cpp "
                           "~49-53 (flush cascades to the track's own pluginList only)",
            "track_getallplugins": "tracktion_Track.cpp ~277-280 (getAllPlugins == "
                                   "pluginList.getPlugins(); rack-internal plugins "
                                   "are NOT in the host track's list)",
            "set_plugin_param_flush": "VitApp/Source/Service/PluginRackControlService.cpp "
                                      "~2051-2054 (owner-track flush only)",
        },
    }

    spec = drv.resolve_proq3(args.timeout)
    report["registry"] = {"identifier": spec["plugin_identifier"],
                          "plugin_path": spec["plugin_path"]}

    # ---- locate fixture in the placed copy r1 (reload discipline)
    located = drv.open_and_locate(args.project, spec, args.timeout)
    track_row = located["track_row"]
    track_id = located["track_id"]
    plugin_id = located["plugin_id"]
    report["fixture"] = {"track_id": track_id, "plugin_id": plugin_id,
                         "rack_nodes": len(drv.rack_nodes(track_row))}

    # volume plugin row (track-level built-in) for arm B
    volume_rows = [p for p in drv.track_plugins(track_row)
                   if "volume" in str(p.get("name") or "").lower()
                   or "volume" in str(p.get("type") or "").lower()]
    volume_plugin_id = first_text(volume_rows[0], "plugin_id", "plugin_item_id", "id", "item_id") if volume_rows else ""
    report["fixture"]["volume_plugin_id"] = volume_plugin_id
    report["fixture"]["volume_row"] = volume_rows[0] if volume_rows else None

    def proq3_readback() -> dict[str, Any]:
        return drv.agent_get_parameters(args.agent_http,
                                        {"track_id": track_id, "plugin_id": plugin_id},
                                        args.timeout)

    def state_row() -> dict[str, Any]:
        state = kernel_command({"cmd": "get_project_state", "command_timeout_ms": 60000},
                               args.timeout)
        rows = drv.state_user_tracks(state)
        return rows[0] if rows else {}

    # ---- baseline
    before = proq3_readback()
    band_row = row_by_id(before, "3") or {}
    band_before = _num(band_row.get("normalized_value"))
    row0 = state_row()
    vol_before = {"db": row0.get("db"), "pan": row0.get("pan"),
                  "volume_db": row0.get("volume_db")}
    report["baseline"] = {"proq3_param3_normalized": band_before,
                          "volume": vol_before}

    # ---- arm A: Pro-Q 3 (rack-internal) 0.5 -> 0.7
    arm_a: dict[str, Any] = {"param": "Band 1 Gain (id 3)", "target": 0.7}
    resp_a = invoke(args.agent_http, "set_plugin_param", {
        "track_id": track_id, "plugin_id": plugin_id,
        "param_id": "3", "value": 0.7}, args.timeout)
    arm_a["write_status"] = str(resp_a.get("status") or "")
    after_write = proq3_readback()
    arm_a["value_after_write"] = _num((row_by_id(after_write, "3") or {}).get("normalized_value"))
    kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, args.timeout)
    located2 = drv.open_and_locate(args.project, spec, args.timeout)
    after_reopen_a = drv.agent_get_parameters(
        args.agent_http, {"track_id": located2["track_id"], "plugin_id": located2["plugin_id"]},
        args.timeout)
    arm_a["value_after_reopen"] = _num((row_by_id(after_reopen_a, "3") or {}).get("normalized_value"))
    arm_a["persisted"] = (arm_a["value_after_reopen"] is not None
                          and abs(arm_a["value_after_reopen"] - 0.7) <= 0.02)
    report["arm_a_rack_internal_proq3"] = arm_a

    # ---- arm B: Volume & Pan (track-level built-in)
    arm_b: dict[str, Any] = {"plugin_id": volume_plugin_id}
    if volume_plugin_id:
        # freshness gate: the agent face requires a get_plugin_parameters READ
        # of the exact plugin before any write (probe run 1 error in report).
        read_b = invoke(args.agent_http, "get_plugin_parameters", {
            "track_id": track_id, "plugin_id": volume_plugin_id,
            "include_parameters": True}, args.timeout)
        arm_b["read_status"] = str(read_b.get("status") or "")
        arm_b["read_rows"] = len(frozen.param_rows(frozen.result_of(read_b)))
        target_norm = 0.3  # fader position; readback face is the track row db/pan
        resp_b = invoke(args.agent_http, "set_plugin_param", {
            "track_id": track_id, "plugin_id": volume_plugin_id,
            "param_id": "volume", "normalized_value": target_norm}, args.timeout)
        arm_b["write_status"] = str(resp_b.get("status") or "")
        arm_b["write_response_head"] = str(resp_b)[:800]
        row_after_write = state_row()
        arm_b["db_after_write"] = row_after_write.get("db")
        kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, args.timeout)
        drv.open_and_locate(args.project, spec, args.timeout)
        row_after_reopen = state_row()
        arm_b["db_after_reopen"] = row_after_reopen.get("db")
        db_wrote = (arm_b["db_after_write"] is not None
                    and vol_before.get("db") is not None
                    and abs(float(arm_b["db_after_write"]) - float(vol_before["db"])) > 0.01)
        arm_b["write_changed_db"] = db_wrote
        arm_b["persisted"] = (db_wrote and arm_b["db_after_reopen"] is not None
                              and abs(float(arm_b["db_after_reopen"]) - float(arm_b["db_after_write"])) <= 0.01)
    report["arm_b_track_volume"] = arm_b

    # ---- verdict shape (mechanical classification, adjudication=supervisor)
    a_reverted = (arm_a["value_after_write"] == 0.7
                  and arm_a["value_after_reopen"] != 0.7)
    b_ok = bool(arm_b.get("persisted"))
    report["discriminator"] = {
        "a_rack_internal_reverted": a_reverted,
        "b_track_level_persisted": b_ok,
        "shape": ("rack_internal_save_flush_gap" if (a_reverted and b_ok)
                  else "broader_reload_or_both_revert" if not b_ok
                  else "unexplained"),
    }

    # ---- restore r1 to default state (band gain back; volume back)
    try:
        invoke(args.agent_http, "set_plugin_param", {
            "track_id": track_id, "plugin_id": plugin_id,
            "param_id": "3", "value": band_before}, args.timeout)
        if volume_plugin_id and arm_b.get("write_changed_db"):
            invoke(args.agent_http, "set_plugin_param", {
                "track_id": track_id, "plugin_id": volume_plugin_id,
                "param_id": "volume",
                "value": float(vol_before["db"])}, args.timeout)
        kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, args.timeout)
        located3 = drv.open_and_locate(args.project, spec, args.timeout)
        final = drv.agent_get_parameters(
            args.agent_http, {"track_id": located3["track_id"], "plugin_id": located3["plugin_id"]},
            args.timeout)
        report["restored_digest"] = drv.fixture_digest(final)
        report["initial_digest_from_placement"] = "5080116bec1afcfa"  # run3 placement_r1
        report["restore_matches_initial"] = (report["restored_digest"]
                                              == report["initial_digest_from_placement"])
    except Exception as error:  # noqa: BLE001
        report["restore_error"] = str(error)[:400]

    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"discriminator": report["discriminator"],
                      "arm_a": {k: arm_a.get(k) for k in ("value_after_write", "value_after_reopen", "persisted")},
                      "arm_b_persisted": arm_b.get("persisted"),
                      "out": str(args.out)}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
