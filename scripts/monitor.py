import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

import zmq

try:
    from rich.console import Console
    from rich.theme import Theme
except ImportError as error:
    raise SystemExit(
        "rich is required for monitor.py. Install it with: pip install rich"
    ) from error


CONTROL_CHAR_PATTERN = re.compile(r"[\x00-\x08\x0b-\x1f\x7f]")

THEME = Theme(
    {
        "monitor": "bold cyan",
        "info": "green",
        "warn": "yellow",
        "error": "bold red",
    }
)


def get_journal_path():
    repo_root = Path(__file__).resolve().parents[1]
    journal_dir = repo_root / "VitApp" / "Workspace" / "Logs"
    journal_dir.mkdir(parents=True, exist_ok=True)
    return journal_dir / "system_journal.jsonl"


def in_integrated_terminal():
    term_program = os.environ.get("TERM_PROGRAM", "").lower()
    return term_program in {"vscode", "cursor"}


def sanitize_for_terminal(message):
    escaped = (
        message.replace("\x1b", "\\x1b")
        .replace("\r", "\\r")
        .replace("\n", "\\n")
        .replace("\t", "\\t")
    )
    return CONTROL_CHAR_PATTERN.sub(lambda match: f"\\x{ord(match.group(0)):02x}", escaped)


def get_level_style(level, message):
    lowered = message.lower()

    if level == "ERROR" or "error" in lowered or "failed" in lowered:
        return "error"

    if level == "WARN" or "warn" in lowered:
        return "warn"

    return "info"


def append_journal_line(journal_path, payload):
    with journal_path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(payload, ensure_ascii=False) + "\n")


def main():
    use_rich = not in_integrated_terminal()
    console = Console(theme=THEME) if use_rich else None
    journal_path = get_journal_path()

    context = zmq.Context()
    socket = context.socket(zmq.SUB)
    socket.connect("tcp://127.0.0.1:5557")
    socket.setsockopt_string(zmq.SUBSCRIBE, "")

    if use_rich:
        console.print("[monitor] listening on tcp://127.0.0.1:5557", style="monitor")
        console.print(f"[monitor] journaling to {journal_path}", style="monitor")
    else:
        print("[monitor] listening on tcp://127.0.0.1:5557")
        print(f"[monitor] journaling to {journal_path}")
        print("[monitor] integrated terminal detected, using plain text safe mode")

    sys.stdout.flush()

    try:
        while True:
            payload = socket.recv_json()
            payload.setdefault("received_at", datetime.now(timezone.utc).isoformat())

            level = str(payload.get("level", "INFO")).upper()
            message = str(payload.get("message", ""))
            safe_message = sanitize_for_terminal(message)
            style = get_level_style(level, message)

            if use_rich:
                console.print(f"[{level}] {safe_message}", style=style)
            else:
                print(f"[{level}] {safe_message}")

            sys.stdout.flush()
            append_journal_line(journal_path, payload)
    except KeyboardInterrupt:
        if use_rich:
            console.print("[monitor] stopped", style="monitor")
        else:
            print("[monitor] stopped")
        sys.stdout.flush()
    finally:
        socket.close(0)
        context.term()


if __name__ == "__main__":
    main()
