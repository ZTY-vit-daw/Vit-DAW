#!/usr/bin/env python3
"""Check Godot IPC command names against C++ CommandDispatcher handlers."""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path


DEFAULT_CPP = Path(r"D:\Vit_DAW\VitApp\Source\Service\CommandDispatcher.cpp")
DEFAULT_GODOT = Path(r"D:\Godot\project\vit-daw-frontend")

HANDLER_RE = re.compile(r'handlers\.emplace\s*\(\s*"([^"]+)"')
STATIC_CMD_RE = re.compile(r'"cmd"\s*:\s*"([^"]+)"')
DYNAMIC_CMD_RE = re.compile(r'"cmd"\s*:\s*([^",}\]\r\n]+)')
STRING_ARG_CMD_RE = re.compile(r"\b(?:send_command_async_await|send_command_async|send_command_no_wait|send_command_json_async)\s*\(\s*\"([^\"]+)\"")
LEGACY_FIELD_RE = re.compile(r'"(?:action|command)"\s*:')


@dataclass(frozen=True)
class Occurrence:
    value: str
    path: Path
    line: int
    text: str


def iter_files(root: Path, suffix: str) -> list[Path]:
    if root.is_file():
        return [root] if root.suffix == suffix else []
    return sorted(p for p in root.rglob(f"*{suffix}") if p.is_file())


def rel(path: Path, base: Path) -> str:
    try:
        return str(path.relative_to(base))
    except ValueError:
        return str(path)


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8", errors="ignore")


def collect_handlers(cpp_path: Path) -> set[str]:
    return set(HANDLER_RE.findall(read_text(cpp_path)))


def collect_godot_commands(godot_root: Path) -> tuple[list[Occurrence], list[Occurrence], list[Occurrence]]:
    static: list[Occurrence] = []
    dynamic: list[Occurrence] = []
    legacy_fields: list[Occurrence] = []

    for path in iter_files(godot_root, ".gd"):
        for line_no, line in enumerate(read_text(path).splitlines(), start=1):
            static_spans: list[tuple[int, int]] = []

            for match in STATIC_CMD_RE.finditer(line):
                static_spans.append(match.span())
                static.append(Occurrence(match.group(1), path, line_no, line.strip()))

            for match in STRING_ARG_CMD_RE.finditer(line):
                static.append(Occurrence(match.group(1), path, line_no, line.strip()))

            for match in DYNAMIC_CMD_RE.finditer(line):
                if any(start <= match.start() and match.end() <= end for start, end in static_spans):
                    continue
                dynamic.append(Occurrence(match.group(1).strip(), path, line_no, line.strip()))

            if LEGACY_FIELD_RE.search(line):
                legacy_fields.append(Occurrence("", path, line_no, line.strip()))

    return static, dynamic, legacy_fields


def format_occurrences(items: list[Occurrence], base: Path, limit: int = 40) -> str:
    if not items:
        return "  none"

    lines: list[str] = []
    for item in items[:limit]:
        value = f"{item.value} " if item.value else ""
        lines.append(f"  {value}{rel(item.path, base)}:{item.line}: {item.text}")
    if len(items) > limit:
        lines.append(f"  ... {len(items) - limit} more")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cpp", type=Path, default=DEFAULT_CPP, help="Path to CommandDispatcher.cpp")
    parser.add_argument("--godot", type=Path, default=DEFAULT_GODOT, help="Path to Godot frontend root")
    args = parser.parse_args()

    if not args.cpp.is_file():
        print(f"ERROR: missing C++ dispatcher: {args.cpp}", file=sys.stderr)
        return 2
    if not args.godot.exists():
        print(f"ERROR: missing Godot root: {args.godot}", file=sys.stderr)
        return 2

    handlers = collect_handlers(args.cpp)
    static_commands, dynamic_commands, legacy_fields = collect_godot_commands(args.godot)

    by_name: dict[str, list[Occurrence]] = {}
    for occurrence in static_commands:
        by_name.setdefault(occurrence.value, []).append(occurrence)

    missing_names = sorted(cmd for cmd in by_name if cmd not in handlers)
    unused_handlers = sorted(handlers.difference(by_name.keys()))

    print("IPC contract check")
    print(f"  C++ handlers: {len(handlers)}")
    print(f"  Godot static command names: {len(by_name)}")
    print(f"  Dynamic cmd sites: {len(dynamic_commands)}")
    print(f"  Legacy action/command field sites: {len(legacy_fields)}")
    print()

    if missing_names:
        print("Missing C++ handlers for Godot static commands:")
        for name in missing_names:
            print(f"  {name}")
            print(format_occurrences(by_name[name], args.godot, limit=5))
        print()
    else:
        print("Missing C++ handlers for Godot static commands: none")
        print()

    if dynamic_commands:
        print("Dynamic cmd sites to review manually:")
        print(format_occurrences(dynamic_commands, args.godot))
        print()

    if legacy_fields:
        print('Legacy "action"/"command" field sites to review manually:')
        print(format_occurrences(legacy_fields, args.godot))
        print()

    if unused_handlers:
        print("C++ handlers not referenced by static Godot cmd literals:")
        for name in unused_handlers:
            print(f"  {name}")
        print()

    return 1 if missing_names else 0


if __name__ == "__main__":
    raise SystemExit(main())
