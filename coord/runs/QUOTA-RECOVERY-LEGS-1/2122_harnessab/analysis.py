#!/usr/bin/env python3
"""G3-REOPEN-COSTCMP-1 three-metric extraction from harness_ab run artifacts.

Metrics (card: G3 reopen condition ① evidence):
  M1 round-level LLM call count  -- telemetry records attributed to each
     round conversation (phase-tagged via the raw chat capture)
  M2 per-turn injection bytes    -- telemetry section_stats prefix/dynamic
  M3 probe physical cost (wall)  -- raw chat capture executed_kernel_reply
     elapsed_ms sums, tiered by the PULL-PROBE-METER-1 table (probe tier:
     ccb.observation_request / mix.observe / mix.request_observation /
     clip.warm_waveform_bake; index tier: ref.query / ref.diff /
     ccb.observation_catalog / project.state; unlisted: recorded, not
     summed into probe cost -- matches settleRecordProbeCost semantics).

Quota truncation detection: response error/reply text carrying
"预扣费额度失败" marks the LLM account quota wall (environment interrupt,
M2-TIMEFX-1 precedent) -- censored rounds are flagged, not dropped.

Usage: python analysis.py <run_dir> [<run_dir> ...]   (writes <run_dir>/analysis.json)
"""
import json
import sys
from pathlib import Path

PROBE_TIER = {
    "ccb.observation_request", "mix.observe", "mix.request_observation",
    "clip.warm_waveform_bake", "clip.warm.waveform.bake",
}
INDEX_TIER = {"ref.query", "ref.diff", "ccb.observation_catalog", "project.state"}
QUOTA_NEEDLE = "预扣费额度失败"


def normalize_tool(name):
    return str(name or "").strip().replace("_", ".")


def load_jsonl(path):
    rows = []
    if not path.exists():
        return rows
    for line in path.read_text(encoding="utf-8-sig").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rows.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return rows


def analyze_run(run_dir: Path):
    rounds = {}          # conversation_id -> round record
    conv_phase = {}      # conversation_id -> phase tag
    # --- raw chat captures: rounds + probe cost (M3) ---
    for mode in ("push", "pull"):
        for row in load_jsonl(run_dir / f"harness_ab_chat_raw_{mode}.jsonl"):
            phase = row.get("phase") or "?"
            conv = row.get("conversation_id") or ""
            resp = row.get("response") or {}
            key = conv or f"{mode}:{phase}:{row.get('turn')}"
            conv_phase[key] = phase
            rec = rounds.setdefault(key, {
                "mode": mode, "phase": phase, "conversation_id": conv,
                "turns_captured": 0, "goal_status_last": "", "stop_last": "",
                "quota_blocked": False, "probe_calls": [], "index_calls": [],
                "unlisted_calls": [], "probe_ms_sum": 0.0,
                "probe_tool_breakdown": {},
            })
            rec["turns_captured"] += 1
            rec["goal_status_last"] = resp.get("goal_status") or rec["goal_status_last"]
            rec["stop_last"] = resp.get("stop_reason") or rec["stop_last"]
            err = " ".join(filter(None, [str(resp.get("error") or ""),
                                          str(resp.get("reply") or "")]))
            if QUOTA_NEEDLE in err:
                rec["quota_blocked"] = True
            for entry in resp.get("executed_kernel_reply") or []:
                if not isinstance(entry, dict):
                    continue
                tool = normalize_tool(entry.get("tool"))
                ms = entry.get("elapsed_ms")
                row_tier = ("probe" if tool in PROBE_TIER else
                            "index" if tool in INDEX_TIER else "unlisted")
                item = {"tool": tool, "elapsed_ms": ms, "tier": row_tier}
                if row_tier == "probe":
                    rec["probe_calls"].append(item)
                    if isinstance(ms, (int, float)):
                        rec["probe_ms_sum"] += ms
                        rec["probe_tool_breakdown"][tool] = (
                            rec["probe_tool_breakdown"].get(tool, 0) + ms)
                elif row_tier == "index":
                    rec["index_calls"].append(item)
                else:
                    rec["unlisted_calls"].append(item)

    # --- telemetry: LLM calls + injection bytes (M1/M2) ---
    for mode in ("push", "pull"):
        for row in load_jsonl(run_dir / f"harness_ab_telemetry_{mode}.jsonl"):
            conv = row.get("conversation_id") or ""
            rec = rounds.get(conv)
            if rec is None:
                # fixture/authority legs share no conversation: record as infra
                rec = rounds.setdefault(conv or f"{mode}:<noconv>", {
                    "mode": mode, "phase": "nonround", "conversation_id": conv,
                    "turns_captured": 0, "goal_status_last": "", "stop_last": "",
                    "quota_blocked": False, "probe_calls": [], "index_calls": [],
                    "unlisted_calls": [], "probe_ms_sum": 0.0,
                    "probe_tool_breakdown": {},
                })
            stats = row.get("section_stats") or {}
            rec.setdefault("llm_calls", 0)
            rec.setdefault("prefix_bytes", [])
            rec.setdefault("dynamic_bytes", [])
            rec.setdefault("llm_errors", 0)
            rec["llm_calls"] += 1
            if isinstance(stats.get("prefix_bytes"), (int, float)):
                rec["prefix_bytes"].append(int(stats["prefix_bytes"]))
            if isinstance(stats.get("dynamic_bytes"), (int, float)):
                rec["dynamic_bytes"].append(int(stats["dynamic_bytes"]))
            if str(row.get("error") or ""):
                rec["llm_errors"] += 1
            rec.setdefault("input_tokens", 0)
            rec["input_tokens"] += int((row.get("usage") or {}).get("input_tokens") or 0)

    # --- summarize per (mode, phase) ---
    summary = {"run_dir": str(run_dir), "rounds": [], "mode_totals": {}}
    for key, rec in rounds.items():
        if rec["phase"] == "nonround":
            continue
        rec = dict(rec)
        rec.pop("probe_calls", None)
        rec.pop("index_calls", None)
        rec.pop("unlisted_calls", None)
        rec["llm_calls"] = rec.get("llm_calls", 0)
        rec["prefix_bytes_total"] = sum(rec.pop("prefix_bytes", []))
        rec["dynamic_bytes_total"] = sum(rec.pop("dynamic_bytes", []))
        rec.pop("llm_errors", None)
        summary["rounds"].append(rec)
    summary["rounds"].sort(key=lambda r: (r["mode"], r["phase"]))
    for mode in ("push", "pull"):
        fams = [r for r in summary["rounds"]
                if r["mode"] == mode and str(r["phase"]).startswith("family_")]
        summary["mode_totals"][mode] = {
            "rounds_counted": len(summary["rounds"]) - len(
                [r for r in summary["rounds"] if r["mode"] != mode]),
            "family_rounds": len(fams),
            "family_llm_calls": sum(r["llm_calls"] for r in fams),
            "family_probe_ms_sum": sum(r["probe_ms_sum"] for r in fams),
            "quota_blocked_rounds": sum(1 for r in summary["rounds"]
                                        if r["mode"] == mode and r["quota_blocked"]),
            "all_llm_calls": sum(r["llm_calls"] for r in summary["rounds"]
                                 if r["mode"] == mode),
            "all_probe_ms_sum": sum(r["probe_ms_sum"] for r in summary["rounds"]
                                    if r["mode"] == mode),
        }
    out = run_dir / "analysis.json"
    out.write_text(json.dumps(summary, ensure_ascii=False, indent=2),
                   encoding="utf-8")
    print(f"wrote {out}")
    for mode in ("push", "pull"):
        print(f"  {mode}: family_llm_calls="
              f"{summary['mode_totals'][mode]['family_llm_calls']} "
              f"family_probe_ms={summary['mode_totals'][mode]['family_probe_ms_sum']} "
              f"quota_blocked={summary['mode_totals'][mode]['quota_blocked_rounds']}")


if __name__ == "__main__":
    for arg in sys.argv[1:]:
        analyze_run(Path(arg))
