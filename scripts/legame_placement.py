#!/usr/bin/env python3
"""Legame L1 placement tooling (LEGAME-DIR-1 / LEGAME-AUTOINTRO-1 / LEGAME-FRESH-1, 2026-11-08).

Product-style handtest placement helper per design report
coord/reports/2026-11-08-legame-ux-design-v1.md sections 2.1-2.3. The 311
placement driver (scripts/m2_handtest_prep.py) stays untouched (different
card's evidence domain); this tool follows the same script family pattern
(python + summary json + fixed exit classes) and owns the L1 legs:

  install-levels (LEGAME-DIR-1, card 314)
     Copy the repo level card directories (coord/levels/<level_id>/ whole
     card: level.yaml + persona + assets + tools) into the product install
     root ~/.vit/levels/<level_id>/ with per-file SHA256 verification
     (MANIFEST-sourced discipline, AGENTS.md section 10). Content
     distribution belongs to the placement/release face - the agent never
     self-installs (design 2.1-3). Idempotent: consecutive runs converge
     to identical installed bytes.

  write-binding  (LEGAME-AUTOINTRO-1, card 315)
     Declare the project <-> level binding sidecar next to the target .vit
     project file: <project_dir>/.vit_agent/legame_binding.json
     (schema vit.legame_binding.v1, opened_at=null until the agent's
     auto-intro fires and writes the markers back). The AGENT never authors
     the binding - this placement face (or a future project wizard,
     bound_source tag) does. Refuses to reset a binding whose opened_at is
     already set unless --force (project-level intro marker discipline).

  relay / place-copy (LEGAME-FRESH-1, card 316 - added by that card)

Exit codes (fixed before any run; single run, no rerun unless declared):
  0 pass            install verified (every file hash matches source)
  2 environment     source card missing/invalid, target unwritable
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

EXIT_PASS = 0
EXIT_ENV = 2
EXIT_CLASS = {EXIT_PASS: "pass", EXIT_ENV: "environment"}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def copy_tree_verified(src: Path, dst: Path) -> List[Dict[str, Any]]:
    """Recursively copy src -> dst, returning per-file rows with source and
    destination sha256 for the verification face. Destination files are
    overwritten (idempotent re-install)."""
    rows: List[Dict[str, Any]] = []
    for path in sorted(src.rglob("*")):
        if path.is_dir():
            continue
        rel = path.relative_to(src)
        target = dst / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target)
        rows.append({
            "file": rel.as_posix(),
            "src_sha256": sha256_file(path),
            "dst_sha256": sha256_file(target),
        })
    return rows


def resolve_levels_root(home_override: str) -> Path:
    if home_override:
        return Path(home_override) / ".vit" / "levels"
    return Path.home() / ".vit" / "levels"


LEGAME_BINDING_SCHEMA = "vit.legame_binding.v1"
LEGAME_BINDING_FILENAME = "legame_binding.json"
LEGAME_AGENT_DIRNAME = ".vit_agent"


def binding_path_for(project_path: str) -> Path:
    return Path(project_path).resolve().parent / LEGAME_AGENT_DIRNAME / LEGAME_BINDING_FILENAME


def cmd_write_binding(args: argparse.Namespace, summary: Dict[str, Any]) -> int:
    """LEGAME-AUTOINTRO-1: declare a project <-> level binding sidecar next to
    the project file (<project_dir>/.vit_agent/legame_binding.json). The agent
    reads the binding and only writes back the opened markers; the binding
    itself is authored by this placement face (or a future project wizard)."""
    project = Path(args.project).resolve()
    binding_path = binding_path_for(str(project))
    summary["write_binding"] = {
        "project": str(project),
        "binding_path": str(binding_path),
        "level_id": args.level_id,
        "bound_source": args.bound_source,
    }
    block = summary["write_binding"]

    if binding_path.exists():
        try:
            existing = json.loads(binding_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            print(f"[legame] FAIL environment: existing binding unreadable: {binding_path} ({exc})", flush=True)
            block["error"] = f"existing binding unreadable: {exc}"
            return EXIT_ENV
        if existing.get("opened_at") and not args.force:
            print(f"[legame] FAIL environment: binding already opened (opened_at={existing.get('opened_at')}); "
                  f"refusing to reset the project-level intro marker without --force (use relay to reset the whole project)", flush=True)
            block["error"] = "binding already opened (need --force or relay)"
            return EXIT_ENV

    binding = {
        "schema_version": LEGAME_BINDING_SCHEMA,
        "level_id": args.level_id,
        "bound_at": datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds"),
        "bound_source": args.bound_source,
        "opened_at": None,
        "opened_conversation_id": None,
    }
    try:
        binding_path.parent.mkdir(parents=True, exist_ok=True)
        binding_path.write_text(json.dumps(binding, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    except OSError as exc:
        print(f"[legame] FAIL environment: write binding: {exc}", flush=True)
        block["error"] = f"write binding: {exc}"
        return EXIT_ENV
    # Readback verification (placement face asserts what the agent will read).
    try:
        back = json.loads(binding_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        print(f"[legame] FAIL environment: binding readback: {exc}", flush=True)
        block["error"] = f"binding readback: {exc}"
        return EXIT_ENV
    if back.get("schema_version") != LEGAME_BINDING_SCHEMA or back.get("level_id") != args.level_id \
            or back.get("opened_at") is not None:
        print(f"[legame] FAIL environment: binding readback mismatch: {json.dumps(back, ensure_ascii=False)}", flush=True)
        block["error"] = "binding readback mismatch"
        return EXIT_ENV
    block["verified"] = True
    print(f"[legame] binding written+verified: {binding_path}", flush=True)
    return EXIT_PASS


def cmd_install_levels(args: argparse.Namespace, summary: Dict[str, Any]) -> int:
    levels_root = resolve_levels_root(args.home)
    source_root = Path(args.levels_source).resolve()
    summary["install_levels"] = {
        "levels_root": str(levels_root),
        "levels_source": str(source_root),
        "level_ids": list(args.level_id),
        "files": [],
    }
    block = summary["install_levels"]

    for level_id in args.level_id:
        src = source_root / level_id
        level_yaml = src / "level.yaml"
        if not src.is_dir() or not level_yaml.is_file():
            print(f"[legame] FAIL environment: level card source missing: {level_yaml}", flush=True)
            block["error"] = f"source missing: {level_yaml}"
            return EXIT_ENV
        dst = levels_root / level_id
        try:
            if dst.exists():
                shutil.rmtree(dst)
            dst.mkdir(parents=True)
            rows = copy_tree_verified(src, dst)
        except OSError as exc:
            print(f"[legame] FAIL environment: install {level_id}: {exc}", flush=True)
            block["error"] = f"install {level_id}: {exc}"
            return EXIT_ENV
        mismatched = [row for row in rows if row["src_sha256"] != row["dst_sha256"]]
        if mismatched:
            print(f"[legame] FAIL environment: hash mismatch after copy for {level_id}: "
                  f"{[row['file'] for row in mismatched]}", flush=True)
            block["error"] = f"hash mismatch: {level_id}"
            return EXIT_ENV
        if not (dst / "level.yaml").is_file():
            print(f"[legame] FAIL environment: level.yaml missing after install: {dst}", flush=True)
            block["error"] = f"level.yaml missing: {dst}"
            return EXIT_ENV
        block["files"].append({"level_id": level_id, "installed_dir": str(dst), "count": len(rows), "rows": rows})
        print(f"[legame] installed {level_id}: {len(rows)} files verified at {dst}", flush=True)

    return EXIT_PASS


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Legame L1 placement tooling (install-levels / write-binding / relay)")
    sub = parser.add_subparsers(dest="command", required=True)

    p_install = sub.add_parser("install-levels", help="LEGAME-DIR-1: install level cards into ~/.vit/levels with hash verification")
    p_install.add_argument("--levels-source", default="", help="repo levels source root (default: <script_dir>/../coord/levels)")
    p_install.add_argument("--level-id", action="append", default=[], help="level id to install (repeatable; default first-run-twinkle)")
    p_install.add_argument("--home", default="", help="home dir override for tests (default: real user home)")
    p_install.add_argument("--output", default="", help="summary json path (stdout summary when empty)")

    p_binding = sub.add_parser("write-binding", help="LEGAME-AUTOINTRO-1: declare project<->level binding sidecar (.vit_agent/legame_binding.json)")
    p_binding.add_argument("--project", required=True, help="target .vit project path (binding lands next to it)")
    p_binding.add_argument("--level-id", default="first-run-twinkle", help="bound level id")
    p_binding.add_argument("--bound-source", default="placement_script", help="binding author tag (placement_script | project_wizard)")
    p_binding.add_argument("--force", action="store_true", help="allow overwriting a binding whose opened_at is already set")
    p_binding.add_argument("--output", default="", help="summary json path (stdout summary when empty)")

    return parser


def main() -> int:
    args = build_parser().parse_args()
    if getattr(args, "levels_source", "") == "":
        args.levels_source = str(Path(__file__).resolve().parent.parent / "coord" / "levels")
    if args.command == "install-levels" and not args.level_id:
        args.level_id = ["first-run-twinkle"]

    summary: Dict[str, Any] = {
        "tool": "legame_placement",
        "command": args.command,
        "stamp": datetime.now(timezone.utc).astimezone().strftime("%Y%m%d_%H%M%S"),
    }

    if args.command == "install-levels":
        code = cmd_install_levels(args, summary)
    elif args.command == "write-binding":
        code = cmd_write_binding(args, summary)
    else:
        print(f"[legame] FAIL environment: unknown command {args.command}", flush=True)
        return EXIT_ENV

    summary["exit_code"] = code
    summary["exit_class"] = EXIT_CLASS.get(code, "unknown")
    out = getattr(args, "output", "")
    if out:
        Path(out).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"exit_code": code, "exit_class": summary["exit_class"]}, ensure_ascii=False), flush=True)
    return code


if __name__ == "__main__":
    sys.exit(main())
