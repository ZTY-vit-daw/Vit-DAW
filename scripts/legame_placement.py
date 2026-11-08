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

  relay / place-copy (LEGAME-FRESH-1, card 316)
     relay: fresh-project guarantee for the handtest target - delete the
     target .vit plus its agent workspace siblings (.vit_agent/,
     .vit_history/, <name>_Media), re-place the pristine fixture from the
     repo template (coord/fixtures/legame/, provenance+sha256 in
     FIXTURE.md), re-declare the binding (prior bound_at/bound_source
     preserved so consecutive relays converge byte-identical; opened
     markers reset to null = "new project" semantics), then read back:
     project sha256 == template sha256, binding fresh in place, workspace
     residue zero (.vit_agent holds ONLY the binding, .vit_history absent).
     Idempotent: consecutive runs converge to the identical end state.
     place-copy: rehearsal-copy rule (design 2.3-2) - place the SAME
     fixture to an ISOLATED destination path (run artifacts dir / %TEMP%)
     with its own binding, resetting any destination workspace residue from
     an earlier rehearsal session (converges to relay's fresh semantics);
     never touches the user target project. All rehearsal/smoke jam faces
     run on copies; the target project is a read-only reference until
     relay time.

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


DEFAULT_FIXTURE = "coord/fixtures/legame/手测·小星星乐局.vit"


def resolve_fixture_template(template_arg: str) -> Path:
    if template_arg:
        return Path(template_arg).resolve()
    return Path(__file__).resolve().parent.parent / DEFAULT_FIXTURE


def workspace_siblings(project: Path) -> Dict[str, Path]:
    """Agent-workspace sibling paths of a .vit project file (projectstore
    AgentDirName/HistoryDirName domains + the kernel media folder)."""
    base = project.stem + "_Media"
    return {
        "agent_dir": project.parent / LEGAME_AGENT_DIRNAME,
        "history_dir": project.parent / ".vit_history",
        "media_dir": project.parent / base,
    }


def delete_workspace_residue(siblings: Dict[str, Path]) -> Dict[str, bool]:
    removed: Dict[str, bool] = {}
    for name, path in siblings.items():
        if path.is_dir():
            shutil.rmtree(path)
            removed[name] = True
        else:
            removed[name] = False
    return removed


def write_fresh_binding(project: Path, level_id: str, bound_source: str,
                        prior: Optional[Dict[str, Any]]) -> Dict[str, Any]:
    """Fresh binding next to the project. Prior bound_at/bound_source are
    preserved when a binding existed before the relay (provenance survives
    state resets; consecutive relays converge byte-identical)."""
    binding: Dict[str, Any] = {
        "schema_version": LEGAME_BINDING_SCHEMA,
        "level_id": level_id,
        "bound_at": (prior or {}).get("bound_at") or datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds"),
        "bound_source": (prior or {}).get("bound_source") or bound_source,
        "opened_at": None,
        "opened_conversation_id": None,
    }
    path = binding_path_for(str(project))
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(binding, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return binding


def cmd_relay(args: argparse.Namespace, summary: Dict[str, Any]) -> int:
    """LEGAME-FRESH-1 relay: reset the handtest target to the pristine
    fixture state (project file + binding fresh + zero workspace residue)."""
    project = Path(args.project).resolve()
    template = resolve_fixture_template(args.template)
    if not template.is_file():
        print(f"[legame] FAIL environment: relay template missing: {template}", flush=True)
        summary["relay"] = {"error": f"template missing: {template}"}
        return EXIT_ENV
    if normpath_equal(str(project), str(template)):
        print("[legame] FAIL environment: relay target must not be the fixture template itself", flush=True)
        summary["relay"] = {"error": "target == template"}
        return EXIT_ENV

    siblings = workspace_siblings(project)
    prior_binding: Optional[Dict[str, Any]] = None
    prior_path = binding_path_for(str(project))
    if prior_path.is_file():
        try:
            prior_binding = json.loads(prior_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            prior_binding = None

    # 1. delete target project + workspace siblings (state reset).
    if project.exists():
        project.unlink()
    removed = delete_workspace_residue(siblings)

    # 2. re-place pristine fixture (byte copy + hash verify).
    project.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(template, project)
    placed_sha = sha256_file(project)
    template_sha = sha256_file(template)
    if placed_sha != template_sha:
        print(f"[legame] FAIL environment: relay copy hash mismatch {placed_sha} != {template_sha}", flush=True)
        summary["relay"] = {"error": "copy hash mismatch"}
        return EXIT_ENV

    # 3. fresh binding (prior provenance preserved, opened markers null).
    binding = write_fresh_binding(project, args.level_id, args.bound_source, prior_binding)

    # 4. readback: residue zero + binding fresh.
    agent_entries = sorted(p.name for p in siblings["agent_dir"].iterdir()) if siblings["agent_dir"].is_dir() else []
    residue_clean = (
        agent_entries == [LEGAME_BINDING_FILENAME]
        and not siblings["history_dir"].exists()
        and not siblings["media_dir"].exists()
    )
    back = json.loads(binding_path_for(str(project)).read_text(encoding="utf-8"))
    binding_fresh = (
        back.get("schema_version") == LEGAME_BINDING_SCHEMA
        and back.get("level_id") == args.level_id
        and back.get("opened_at") is None
        and back.get("opened_conversation_id") is None
    )
    summary["relay"] = {
        "project": str(project),
        "template": str(template),
        "template_sha256": template_sha,
        "placed_sha256": placed_sha,
        "removed": removed,
        "binding": binding,
        "agent_dir_entries": agent_entries,
        "history_dir_absent": not siblings["history_dir"].exists(),
        "media_dir_absent": not siblings["media_dir"].exists(),
        "residue_clean": residue_clean,
        "binding_fresh": binding_fresh,
        "verified": placed_sha == template_sha and residue_clean and binding_fresh,
    }
    if not summary["relay"]["verified"]:
        print(f"[legame] FAIL environment: relay verification failed: {json.dumps(summary['relay'], ensure_ascii=False)[:400]}", flush=True)
        return EXIT_ENV
    print(f"[legame] relay verified: {project} (sha256={placed_sha[:12]}..., binding fresh, residue zero)", flush=True)
    return EXIT_PASS


def cmd_place_copy(args: argparse.Namespace, summary: Dict[str, Any]) -> int:
    """LEGAME-FRESH-1 rehearsal-copy rule: place the fixture to an ISOLATED
    destination with its own binding. The user target project is never
    referenced by this command (read-only reference until relay time)."""
    dest = Path(args.dest).resolve()
    template = resolve_fixture_template(args.template)
    if not template.is_file():
        print(f"[legame] FAIL environment: place-copy template missing: {template}", flush=True)
        summary["place_copy"] = {"error": f"template missing: {template}"}
        return EXIT_ENV
    if normpath_equal(str(dest), str(template)):
        print("[legame] FAIL environment: place-copy destination must not be the fixture template itself", flush=True)
        summary["place_copy"] = {"error": "dest == template"}
        return EXIT_ENV
    dest.parent.mkdir(parents=True, exist_ok=True)
    # Reset the destination workspace siblings too: re-placing a copy over a
    # dest that a previous rehearsal session used must converge to the same
    # fresh state as relay (residue from the earlier session is rehearsal
    # state, not evidence - it must not leak into the next copy session).
    delete_workspace_residue(workspace_siblings(dest))
    if dest.exists():
        dest.unlink()
    shutil.copyfile(template, dest)
    placed_sha = sha256_file(dest)
    template_sha = sha256_file(template)
    if placed_sha != template_sha:
        print(f"[legame] FAIL environment: place-copy hash mismatch {placed_sha} != {template_sha}", flush=True)
        summary["place_copy"] = {"error": "copy hash mismatch"}
        return EXIT_ENV
    binding = write_fresh_binding(dest, args.level_id, args.bound_source, None)
    back = json.loads(binding_path_for(str(dest)).read_text(encoding="utf-8"))
    ok = back.get("opened_at") is None and back.get("level_id") == args.level_id
    summary["place_copy"] = {
        "dest": str(dest),
        "template": str(template),
        "template_sha256": template_sha,
        "placed_sha256": placed_sha,
        "binding": binding,
        "verified": ok,
    }
    if not ok:
        print("[legame] FAIL environment: place-copy binding verification failed", flush=True)
        return EXIT_ENV
    print(f"[legame] place-copy verified: {dest} (isolated rehearsal copy, target project untouched)", flush=True)
    return EXIT_PASS


def normpath_equal(a: str, b: str) -> bool:
    return os.path.normcase(os.path.normpath(a)) == os.path.normcase(os.path.normpath(b))


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

    p_relay = sub.add_parser("relay", help="LEGAME-FRESH-1: reset the handtest target to the pristine fixture (project+binding fresh, residue zero)")
    p_relay.add_argument("--project", required=True, help="handtest target .vit path (its workspace siblings get reset)")
    p_relay.add_argument("--template", default="", help="pristine fixture .vit (default: repo coord/fixtures/legame/)")
    p_relay.add_argument("--level-id", default="first-run-twinkle")
    p_relay.add_argument("--bound-source", default="placement_script")
    p_relay.add_argument("--output", default="", help="summary json path (stdout summary when empty)")

    p_copy = sub.add_parser("place-copy", help="LEGAME-FRESH-1: rehearsal copy to an ISOLATED path (target project untouched)")
    p_copy.add_argument("--dest", required=True, help="isolated destination .vit path (run artifacts dir / %TEMP%)")
    p_copy.add_argument("--template", default="", help="pristine fixture .vit (default: repo coord/fixtures/legame/)")
    p_copy.add_argument("--level-id", default="first-run-twinkle")
    p_copy.add_argument("--bound-source", default="placement_script")
    p_copy.add_argument("--output", default="", help="summary json path (stdout summary when empty)")

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
    elif args.command == "relay":
        code = cmd_relay(args, summary)
    elif args.command == "place-copy":
        code = cmd_place_copy(args, summary)
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
