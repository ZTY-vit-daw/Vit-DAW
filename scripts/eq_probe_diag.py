#!/usr/bin/env python3
"""Dump display-probe evidence for EQ plugins.

Read-only diagnostic: loads each plugin onto its own scratch track, then calls
plugin.get_parameters and plugin_grabber.explain_controls. Writes no plugin
parameters, so it cannot disturb plugin state.

The question it answers: for each band frequency parameter, where does the unit
string come from (rendered value text vs the plugin's declared getLabel()), and
what min/max do the probe samples actually observe? That decides whether band
role detection can key on unit strings at all.
"""
from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.request
from pathlib import Path

DEFAULT_PLUGINS = [
    ("TDR Nova", r"C:\Program Files\Common Files\VST3\TDR Nova.vst3"),
    ("FreeEQ8", r"C:\Program Files\Common Files\VST3\FreeEQ8.vst3"),
    ("Marvel GEQ", r"C:\Program Files\Common Files\VST3\Marvel GEQ.vst3"),
    ("Pro-Q 3", r"C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3"),
    ("ZamEQ2", r"C:\Program Files\Common Files\VST3\zam-plugins-4.5-win64\zam-plugins-4.5\ZamEQ2.vst3"),
    ("ZamGEQ31", r"C:\Program Files\Common Files\VST3\zam-plugins-4.5-win64\zam-plugins-4.5\ZamGEQ31.vst3"),
]


def request_json(method: str, url: str, payload: dict | None, timeout: float) -> dict:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode()
    req = urllib.request.Request(
        url, data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8", errors="replace"))


def invoke(base: str, tool: str, args: dict, timeout: float, confirmed: bool = False) -> dict:
    return request_json("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool, "args": args, "confirmed": confirmed, "source": "eq_probe_diag",
    }, timeout)


def result_of(resp: dict) -> dict:
    inner = resp.get("result", resp)
    return inner if isinstance(inner, dict) else {}


def ok(resp: dict, label: str) -> dict:
    if str(resp.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{label}: status={resp.get('status')} error={resp.get('error', '')}")
    return result_of(resp)


def load_plugin(base: str, label: str, path: str, timeout: float) -> tuple[str, str]:
    track = ok(invoke(base, "track.add_audio", {"name": f"diag {label}"}, timeout, True),
               f"{label} track.add_audio")
    track_id = str(track.get("track_id") or track.get("id") or "").strip()
    if not track_id:
        raise RuntimeError(f"{label}: no track_id")
    rack = ok(invoke(base, "plugin.load_to_rack",
                     {"track_id": track_id, "plugin_path": path}, timeout, True),
              f"{label} plugin.load_to_rack")
    plugin_id = str(rack.get("plugin_id") or rack.get("node_id") or "").strip()
    if not plugin_id:
        raise RuntimeError(f"{label}: no plugin_id")
    return track_id, plugin_id


def probe_line(row: dict) -> str:
    probe = row.get("display_probe")
    if not isinstance(probe, dict):
        return "    display_probe: ABSENT"
    samples = [s for s in probe.get("samples", []) if isinstance(s, dict)]
    texts = [str(s.get("text", "")) for s in samples]
    return (f"    label={probe.get('label', '')!r}"
            f" issues={probe.get('issues', [])}"
            f" samples={texts}")


def interesting(row: dict) -> bool:
    name = str(row.get("name", "")).lower()
    return ("freq" in name or "hz" in name or "gain" in name
            or name.startswith("band") or " q" in name or name.endswith(" q"))


def report(base: str, label: str, path: str, timeout: float, out_dir: Path) -> None:
    print(f"\n{'=' * 70}\n== {label}\n{'=' * 70}")
    track_id, plugin_id = load_plugin(base, label, path, timeout)
    time.sleep(0.7)

    params = ok(invoke(base, "plugin.get_parameters",
                       {"track_id": track_id, "plugin_id": plugin_id,
                        "include_parameters": True}, timeout),
                f"{label} get_parameters")
    rows = [r for r in params.get("parameters", []) if isinstance(r, dict)]

    explain = ok(invoke(base, "plugin_grabber.explain_controls",
                        {"track_id": track_id, "plugin_id": plugin_id}, timeout),
                 f"{label} explain_controls")
    summary = explain.get("eq_band_summary")

    (out_dir / f"{label.replace(' ', '_')}_params.json").write_text(
        json.dumps({"parameters": params, "explain": explain}, ensure_ascii=False, indent=2),
        encoding="utf-8")

    with_probe = sum(1 for r in rows if isinstance(r.get("display_probe"), dict))
    with_label = sum(1 for r in rows
                     if isinstance(r.get("display_probe"), dict)
                     and str(r["display_probe"].get("label", "")).strip())
    print(f"  params={len(rows)}  with_display_probe={with_probe}  with_nonempty_label={with_label}")
    if isinstance(summary, dict):
        print(f"  eq_model={summary.get('eq_model')!r}  "
              f"band_count={summary.get('band_count', summary.get('active_band_count'))}")
    else:
        print(f"  eq_band_summary=None  (template_role={explain.get('template_role')!r} "
              f"class={explain.get('plugin_class')!r})")

    shown = [r for r in rows if interesting(r)][:8]
    if not shown:
        shown = rows[:8]
    for row in shown:
        print(f"  [{row.get('param_id', row.get('id'))}] {row.get('name')!r} "
              f"value_text={row.get('value_text')!r} discrete={row.get('is_discrete')}")
        print(probe_line(row))


def main() -> None:
    ap = argparse.ArgumentParser(description="dump EQ display-probe evidence")
    ap.add_argument("--agent-http", default="http://127.0.0.1:7878")
    ap.add_argument("--timeout-sec", type=float, default=120.0)
    ap.add_argument("--out-dir", default="scripts/_eq_probe_diag_out")
    ap.add_argument("--only", default="", help="comma-separated subset of labels")
    args = ap.parse_args()

    base = args.agent_http.rstrip("/")
    health = request_json("GET", base + "/health", None, 10)
    if str(health.get("status", "")).lower() not in {"ok", "ready"}:
        print(f"FAIL: agent not ready: {health}")
        sys.exit(1)

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    wanted = {s.strip() for s in args.only.split(",") if s.strip()}
    plugins = [p for p in DEFAULT_PLUGINS if not wanted or p[0] in wanted]

    scan_dirs = sorted({str(Path(p).parent) for _, p in plugins})
    print(f"== scanning {len(scan_dirs)} dirs")
    invoke(base, "plugin.scan", {"paths": scan_dirs}, args.timeout_sec, True)

    failures = []
    for label, path in plugins:
        if not Path(path).exists():
            print(f"\n-- SKIP {label}: not found at {path}")
            continue
        try:
            report(base, label, path, args.timeout_sec, out_dir)
        except Exception as exc:
            print(f"  ERROR: {exc}")
            failures.append((label, str(exc)))

    print(f"\n{'=' * 70}")
    print(f"full dumps written to {out_dir}")
    if failures:
        print(f"{len(failures)} plugin(s) errored:")
        for label, err in failures:
            print(f"  - {label}: {err}")


if __name__ == "__main__":
    main()
