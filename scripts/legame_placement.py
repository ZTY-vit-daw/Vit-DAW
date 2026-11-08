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

  write-binding  (LEGAME-AUTOINTRO-1, card 315 - added by that card)
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

    return parser


def main() -> int:
    args = build_parser().parse_args()
    if getattr(args, "levels_source", "") == "":
        args.levels_source = str(Path(__file__).resolve().parent.parent / "coord" / "levels")
    if hasattr(args, "level_id") and not args.level_id:
        args.level_id = ["first-run-twinkle"]

    summary: Dict[str, Any] = {
        "tool": "legame_placement",
        "command": args.command,
        "stamp": datetime.now(timezone.utc).astimezone().strftime("%Y%m%d_%H%M%S"),
    }

    if args.command == "install-levels":
        code = cmd_install_levels(args, summary)
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
