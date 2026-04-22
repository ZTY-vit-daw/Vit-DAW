import json
import os
from pathlib import Path

from bridge_core import BridgeConfig, run_bridge


def _to_bool(value, default=False):
    if value is None:
        return default
    return str(value).strip().lower() in ("1", "true", "on", "yes")


def _load_overrides(default_config_path: Path):
    config_path = Path(os.getenv("VIT_BRIDGE_CONFIG", str(default_config_path)))
    if not config_path.exists():
        return {}
    try:
        with config_path.open("r", encoding="utf-8") as f:
            data = json.load(f)
            return data if isinstance(data, dict) else {}
    except Exception:
        return {}


def main():
    repo_root = Path(__file__).resolve().parent.parent
    logs_dir = repo_root / "VitApp" / "Workspace" / "Logs"
    default_config_path = Path(__file__).resolve().parent / "bridge_prod.config.json"
    overrides = _load_overrides(default_config_path)

    zmq_sub_url = os.getenv("VIT_BRIDGE_ZMQ_SUB_URL", overrides.get("zmq_sub_url", "tcp://127.0.0.1:5556"))
    zmq_req_url = os.getenv("VIT_BRIDGE_ZMQ_REQ_URL", overrides.get("zmq_req_url", "tcp://127.0.0.1:5555"))
    godot_ip = os.getenv("VIT_BRIDGE_GODOT_IP", overrides.get("godot_ip", "127.0.0.1"))
    udp_to_godot = int(os.getenv("VIT_BRIDGE_UDP_TO_GODOT", overrides.get("udp_to_godot", 4444)))
    udp_from_godot = int(os.getenv("VIT_BRIDGE_UDP_FROM_GODOT", overrides.get("udp_from_godot", 4445)))
    req_timeout_ms = int(os.getenv("VIT_BRIDGE_REQ_TIMEOUT_MS", overrides.get("req_timeout_ms", 2000)))
    req_max_retries = int(os.getenv("VIT_BRIDGE_REQ_MAX_RETRIES", overrides.get("req_max_retries", 1)))
    keep_last_log_lines = int(os.getenv("VIT_BRIDGE_KEEP_LAST_LOG_LINES", overrides.get("keep_last_log_lines", 400)))
    verbose = _to_bool(os.getenv("VIT_BRIDGE_VERBOSE", overrides.get("verbose", False)), False)
    last_log_path = os.getenv(
        "VIT_BRIDGE_LAST_LOG_PATH",
        overrides.get("last_log_path", str(logs_dir / "bridge_last.log")),
    )

    cfg = BridgeConfig(
        zmq_sub_url=zmq_sub_url,
        zmq_req_url=zmq_req_url,
        godot_ip=godot_ip,
        udp_to_godot=udp_to_godot,
        udp_from_godot=udp_from_godot,
        req_timeout_ms=req_timeout_ms,
        req_max_retries=req_max_retries,
        verbose=verbose,
        last_log_path=last_log_path,
        keep_last_log_lines=keep_last_log_lines,
    )
    run_bridge(cfg)


if __name__ == "__main__":
    main()
