import argparse
import json
import shutil
import sys
import time
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Any, Dict, List


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def send_command(req_socket: Any, payload: Dict[str, Any]) -> Dict[str, Any]:
    req_socket.send_string(json.dumps(payload, ensure_ascii=False))
    raw = req_socket.recv_string()
    try:
        reply = json.loads(raw)
    except json.JSONDecodeError:
        return {"status": "error", "raw_reply": raw}
    if isinstance(reply, dict):
        return reply
    return {"status": "error", "raw_reply": reply}


def require_ok(label: str, reply: Dict[str, Any]) -> None:
    if str(reply.get("status", "")).lower() != "ok":
        raise RuntimeError(f"{label} did not return ok: {json.dumps(reply, ensure_ascii=False)[:2000]}")


def compact_reply(reply: Dict[str, Any]) -> Dict[str, Any]:
    out: Dict[str, Any] = {"status": reply.get("status"), "command": reply.get("command")}
    for key in ("message", "safe_to_apply", "audio_clip_count", "changes_audio_device_sample_rate"):
        if key in reply:
            out[key] = reply[key]
    if isinstance(reply.get("audio_settings"), dict):
        settings = reply["audio_settings"]
        out["audio_settings"] = {
            key: settings.get(key)
            for key in (
                "schema_version",
                "sample_rate_hz",
                "record_bit_depth",
                "record_file_type",
                "pcm_format",
                "import_sample_rate_policy",
                "import_bit_depth_policy",
                "media_copy_policy",
                "channel_import_policy",
                "render_default_sample_rate_hz",
                "render_default_bit_depth",
                "render_default_file_type",
                "dither_policy",
                "metadata_state",
                "defaulted_this_call",
                "migration_state",
                "defaulted_origin",
            )
            if key in settings
        }
        if isinstance(settings.get("capabilities"), dict):
            out["audio_settings"]["capabilities"] = settings["capabilities"]
    warnings = reply.get("warnings")
    if isinstance(warnings, list):
        out["warnings"] = warnings[:8]
    if isinstance(reply.get("summary"), dict):
        out["summary"] = reply["summary"]
    if isinstance(reply.get("audio_settings_snapshot"), dict):
        out["audio_settings_snapshot"] = reply["audio_settings_snapshot"]
    if isinstance(reply.get("import_plan"), dict):
        plan = reply["import_plan"]
        out["import_plan"] = {
            key: plan.get(key)
            for key in (
                "intended_mode",
                "target_policy",
                "start_time_seconds",
                "tracks_to_create",
                "copy_reference_strategy",
                "requires_user_confirmation",
            )
            if key in plan
        }
        for source_key, target_key in (
            ("track_plan", "track_plan_preview"),
            ("sample_rate_mismatches", "sample_rate_mismatch_examples"),
            ("bit_depth_or_format_mismatches", "bit_depth_or_format_mismatch_examples"),
            ("unreadable_files", "unreadable_file_examples"),
        ):
            value = plan.get(source_key)
            if isinstance(value, list):
                out["import_plan"][target_key] = value[:8]
    files = reply.get("files")
    if isinstance(files, list):
        out["file_preview"] = files[:8]
    return out


def audio_setting(reply: Dict[str, Any], key: str, fallback: Any = None) -> Any:
    settings = reply.get("audio_settings")
    if isinstance(settings, dict):
        return settings.get(key, fallback)
    return fallback


def recommended_presets(reply: Dict[str, Any]) -> List[Dict[str, Any]]:
    presets = audio_setting(reply, "recommended_presets", [])
    if isinstance(presets, list):
        return [item for item in presets if isinstance(item, dict)]
    return []


def write_legacy_project_without_audio_settings(template_path: Path, legacy_path: Path) -> None:
    # The kernel saves .vit as a VIT1 app-bound encrypted container, so the
    # legacy fixture must be derived from a plain-XML project template instead
    # of stripping sections out of the saved container.
    shutil.copyfile(template_path, legacy_path)
    tree = ET.parse(legacy_path)
    root = tree.getroot()
    removed = 0
    for child in list(root):
        if child.tag == "VIT_AUDIO_SETTINGS":
            root.remove(child)
            removed += 1
    if removed <= 0:
        raise RuntimeError("could not create legacy fixture: VIT_AUDIO_SETTINGS was not present in legacy template")
    tree.write(legacy_path, encoding="utf-8", xml_declaration=True)


def run_probe(args: argparse.Namespace) -> int:
    import zmq

    output_path = Path(args.output).resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    project_path = Path(args.project_path).resolve()
    training_folder = Path(args.training_folder).resolve()
    if not training_folder.is_dir():
        raise RuntimeError(f"training folder does not exist: {training_folder}")
    legacy_template = Path(args.legacy_template).resolve()
    if not legacy_template.is_file():
        raise RuntimeError(f"legacy plain-XML project template does not exist: {legacy_template}")

    context = zmq.Context()
    req = context.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.SNDTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(args.req_url)

    commands: List[Dict[str, Any]] = []
    replies: Dict[str, Dict[str, Any]] = {}

    def call(label: str, payload: Dict[str, Any], ok: bool = True) -> Dict[str, Any]:
        commands.append(payload)
        reply = send_command(req, payload)
        replies[label] = reply
        if ok:
            require_ok(label, reply)
        return reply

    try:
        ping = call("ping", {"cmd": "ping"})
        new_project = call("new_project", {"cmd": "new_project"})
        default_settings = call("default_get_audio_settings", {"cmd": "project.get_audio_settings"})

        if int(audio_setting(default_settings, "sample_rate_hz", 0)) != 48000:
            raise RuntimeError("default sample_rate_hz is not 48000")
        if int(audio_setting(default_settings, "record_bit_depth", 0)) != 24:
            raise RuntimeError("default record_bit_depth is not 24")
        if str(audio_setting(default_settings, "record_file_type", "")) != "WAV/BWF":
            raise RuntimeError("default record_file_type is not WAV/BWF")
        cd_export_presets = [
            preset for preset in recommended_presets(default_settings)
            if str(preset.get("preset_id") or "") == "cd_export"
        ]
        if not cd_export_presets:
            raise RuntimeError("CD Export recommended preset is missing")
        cd_export = cd_export_presets[0]
        if int(cd_export.get("sample_rate_hz") or 0) != 44100 or int(cd_export.get("bit_depth") or 0) != 16:
            raise RuntimeError("CD Export preset is not 44.1 kHz / 16-bit")
        if bool(cd_export.get("default_project_working_spec")):
            raise RuntimeError("CD Export preset must not be the default project working spec")

        mismatch_settings = {
            "sample_rate_hz": args.mismatch_sample_rate,
            "record_bit_depth": 16,
            "record_file_type": "WAV",
            "pcm_format": "int16",
            "import_sample_rate_policy": "ask",
            "import_bit_depth_policy": "ask",
            "media_copy_policy": "ask",
            "channel_import_policy": "preserve_interleaved",
            "render_default_sample_rate_hz": 44100,
            "render_default_bit_depth": 16,
            "render_default_file_type": "WAV",
            "dither_policy": "ask",
        }
        validate = call("validate_mismatch_settings", {
            "cmd": "project.validate_audio_settings_change",
            "audio_settings": mismatch_settings,
        })
        set_reply = call("set_mismatch_settings", {
            "cmd": "project.set_audio_settings",
            "audio_settings": mismatch_settings,
        })
        warnings = set_reply.get("warnings")
        if not isinstance(warnings, list):
            warnings = []
        warning_codes = [
            str(item.get("code"))
            for item in warnings
            if isinstance(item, dict) and item.get("code")
        ]
        warning_values = json.dumps(warnings, ensure_ascii=False)
        mismatch_warning_present = (
            "project_sample_rate_differs_from_audio_device" in warning_codes
            or "audio_device_sample_rate_hz" in warning_values
        )

        save_as = call("save_as_project", {"cmd": "save_as_project", "file_path": str(project_path)})
        with open(project_path, "rb") as saved_project_file:
            saved_container_magic = saved_project_file.read(4).decode("ascii", errors="replace")
        if saved_container_magic != "VIT1":
            raise RuntimeError(
                f"saved .vit is not a VIT1 app-bound encrypted container (magic={saved_container_magic!r})"
            )
        reopen = call("open_saved_project", {"cmd": "open_project", "file_path": str(project_path)})
        reopened_settings = call("reopened_get_audio_settings", {"cmd": "project.get_audio_settings"})
        if int(audio_setting(reopened_settings, "sample_rate_hz", 0)) != args.mismatch_sample_rate:
            raise RuntimeError("saved/reopened sample_rate_hz did not persist")
        if int(audio_setting(reopened_settings, "record_bit_depth", 0)) != 16:
            raise RuntimeError("saved/reopened record_bit_depth did not persist")

        legacy_project_path = project_path.with_name(project_path.stem + "_legacy_no_audio_settings" + project_path.suffix)
        write_legacy_project_without_audio_settings(legacy_template, legacy_project_path)
        legacy_open = call("open_legacy_project_without_audio_settings", {
            "cmd": "open_project",
            "file_path": str(legacy_project_path),
        })
        legacy_settings = call("legacy_get_audio_settings", {"cmd": "project.get_audio_settings"})
        if int(audio_setting(legacy_settings, "sample_rate_hz", 0)) != 48000:
            raise RuntimeError("legacy project fallback sample_rate_hz is not 48000")
        if int(audio_setting(legacy_settings, "record_bit_depth", 0)) != 24:
            raise RuntimeError("legacy project fallback record_bit_depth is not 24")
        if str(audio_setting(legacy_settings, "record_file_type", "")) != "WAV/BWF":
            raise RuntimeError("legacy project fallback record_file_type is not WAV/BWF")
        if str(audio_setting(legacy_settings, "migration_state", "")) != "defaulted_from_legacy":
            raise RuntimeError("legacy project did not report defaulted_from_legacy migration_state")

        restore_settings = {
            "sample_rate_hz": 48000,
            "record_bit_depth": 24,
            "record_file_type": "WAV/BWF",
            "pcm_format": "int24",
            "import_sample_rate_policy": "ask",
            "import_bit_depth_policy": "keep_source",
            "media_copy_policy": "reference_original",
            "channel_import_policy": "preserve_interleaved",
            "render_default_sample_rate_hz": 48000,
            "render_default_bit_depth": 24,
            "render_default_file_type": "WAV",
            "dither_policy": "unknown",
        }
        restore = call("restore_default_production_settings", {
            "cmd": "project.set_audio_settings",
            "audio_settings": restore_settings,
        })

        preflight = call("training_stems_import_preflight", {
            "cmd": "project.import_preflight",
            "folder_path": str(training_folder),
            "recursive": False,
            "media_kinds": ["audio"],
            "intended_mode": "stems_folder",
            "target_policy": "create_tracks",
            "start_time_seconds": 0,
        })
        summary = preflight.get("summary")
        if not isinstance(summary, dict):
            raise RuntimeError("project.import_preflight did not return summary")
        discovered = int(summary.get("discovered_audio_file_count") or 0)
        readable = int(summary.get("readable_file_count") or 0)
        tracks_to_create = int(summary.get("tracks_to_create") or 0)
        if discovered <= 0:
            raise RuntimeError("project.import_preflight discovered no audio files")
        if readable <= 0:
            raise RuntimeError("project.import_preflight found no readable audio files")
        if tracks_to_create != readable:
            raise RuntimeError("project.import_preflight tracks_to_create does not match readable files")

        inspect = call("training_stems_inspect_files", {
            "cmd": "media.inspect_files",
            "folder_path": str(training_folder),
            "recursive": False,
            "media_kinds": ["audio"],
        })
        inspect_summary = inspect.get("summary")
        if not isinstance(inspect_summary, dict) or int(inspect_summary.get("discovered_audio_file_count") or 0) <= 0:
            raise RuntimeError("media.inspect_files did not inspect audio files")

        status = "passed"
        failure = ""
    except Exception as exc:
        status = "failed"
        failure = str(exc)
    finally:
        req.close(0)
        context.term()

    report = {
        "schema_version": "project_audio_settings_preflight_smoke.v1",
        "created_at": now_iso(),
        "status": status,
        "error": failure,
        "req_url": args.req_url,
        "project_path": str(project_path),
        "training_folder": str(training_folder),
        "commands": commands,
        "replies": replies,
        "compact": {label: compact_reply(reply) for label, reply in replies.items()},
        "assertions": {
            "default_project_sample_rate_hz": 48000,
            "default_record_bit_depth": 24,
            "default_record_file_type": "WAV/BWF",
            "cd_export_preset_is_delivery_not_default": locals().get("cd_export", {}).get("role") == "delivery_export"
                and not bool(locals().get("cd_export", {}).get("default_project_working_spec")),
            "saved_vit_container_magic": locals().get("saved_container_magic"),
            "saved_reopened_sample_rate_hz": args.mismatch_sample_rate,
            "saved_reopened_record_bit_depth": 16,
            "legacy_fallback_sample_rate_hz": (
                replies.get("legacy_get_audio_settings", {}).get("audio_settings", {}).get("sample_rate_hz")
                if isinstance(replies.get("legacy_get_audio_settings", {}).get("audio_settings"), dict)
                else None
            ),
            "legacy_fallback_record_bit_depth": (
                replies.get("legacy_get_audio_settings", {}).get("audio_settings", {}).get("record_bit_depth")
                if isinstance(replies.get("legacy_get_audio_settings", {}).get("audio_settings"), dict)
                else None
            ),
            "legacy_fallback_migration_state": (
                replies.get("legacy_get_audio_settings", {}).get("audio_settings", {}).get("migration_state")
                if isinstance(replies.get("legacy_get_audio_settings", {}).get("audio_settings"), dict)
                else None
            ),
            "mismatch_warning_present": locals().get("mismatch_warning_present", False),
            "preflight_discovered_audio_file_count": (
                replies.get("training_stems_import_preflight", {}).get("summary", {}).get("discovered_audio_file_count")
                if isinstance(replies.get("training_stems_import_preflight", {}).get("summary"), dict)
                else None
            ),
            "preflight_tracks_to_create": (
                replies.get("training_stems_import_preflight", {}).get("summary", {}).get("tracks_to_create")
                if isinstance(replies.get("training_stems_import_preflight", {}).get("summary"), dict)
                else None
            ),
        },
    }
    output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    if status != "passed":
        print(json.dumps({"status": status, "error": failure, "output": str(output_path)}, ensure_ascii=False))
        return 1
    print(json.dumps({
        "status": "passed",
        "output": str(output_path),
        "project_path": str(project_path),
        "preflight": report["assertions"],
    }, ensure_ascii=False))
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--req-url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--training-folder", required=True)
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--legacy-template", required=True,
                        help="plain-XML project template (with VIT_AUDIO_SETTINGS) used to build the legacy fixture")
    parser.add_argument("--req-timeout-ms", type=int, default=30000)
    parser.add_argument("--mismatch-sample-rate", type=int, default=44100)
    return run_probe(parser.parse_args())


if __name__ == "__main__":
    sys.exit(main())
