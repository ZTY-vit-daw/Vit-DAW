"""
Development bridge entrypoint.
Use scripts/bridge_prod.py for release mode.
"""

from pathlib import Path

from bridge_core import BridgeConfig, run_bridge


def main():
    logs_dir = Path(__file__).resolve().parent.parent / "VitApp" / "Workspace" / "Logs"
    cfg = BridgeConfig(
        verbose=True,
        last_log_path=str(logs_dir / "bridge_last_dev.log"),
        keep_last_log_lines=600,
        req_timeout_ms=180000,
    )
    run_bridge(cfg)

if __name__ == "__main__":
    main()