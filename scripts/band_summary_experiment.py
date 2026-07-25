#!/usr/bin/env python3
"""Controlled experiment: does eq_band_summary fix LLM band selection?

Hypothesis under test:
    Today's wrong-band failure is caused by MISSING BAND STRUCTURE in the
    context, not by the LLM being unable to reason about EQ at all.

Design: 3 cases x 2 arms (with / without a hand-built eq_band_summary) x N reps.
Verdict is read from the param_id actually passed to set_plugin_param, never
from the model's prose.

  Case 1  TDR Nova (fixed_slot)  "3.4kHz -3dB"   -> must pick B3 (1606/1604)
  Case 2  TDR Nova (fixed_slot)  "100Hz  -2dB"   -> must pick B1 (52/50)
          (Case 2 exists to rule out "always picks the first band")
  Case 3  Pro-Q 3  (free_floating) "3.4kHz -3dB" -> must write Used+Freq+Gain

Usage:
    python scripts/band_summary_experiment.py --setup      # load plugins, print ids
    python scripts/band_summary_experiment.py --run        # run the experiment
"""
from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.request

AGENT = "http://127.0.0.1:7878"
TDR_PATH = r"C:\Program Files\Common Files\VST3\TDR Nova.vst3"
PROQ_PATH = r"C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3"


def post(path: str, payload: dict, timeout: float = 300.0) -> dict:
    data = json.dumps(payload, ensure_ascii=False).encode()
    req = urllib.request.Request(
        AGENT + path, data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8", errors="replace"))


def invoke(tool: str, args: dict, confirmed: bool = True, timeout: float = 60.0) -> dict:
    return post("/agent/invoke",
                {"tool": tool, "args": args, "confirmed": confirmed, "source": "band_exp"},
                timeout)


def chat(conv_id: str, message: str, context: dict, timeout: float = 300.0) -> dict:
    return post("/agent/chat",
                {"conversation_id": conv_id, "message": message, "context": context},
                timeout)


# ---------------------------------------------------------------------------
# Setup
# ---------------------------------------------------------------------------

def setup() -> dict:
    """Create a fresh project with one track carrying TDR Nova and Pro-Q 3."""
    print("== setup")
    invoke("project.new", {})
    time.sleep(0.6)

    ta = invoke("track.add_audio", {"name": "Band Summary Experiment"})
    res = ta.get("result", {})
    track_id = str(res.get("track_id") or res.get("id") or "")
    if not track_id:
        raise SystemExit(f"track.add_audio gave no track_id: {json.dumps(ta)[:400]}")
    print(f"  track_id={track_id}")

    ids = {"track_id": track_id}
    for label, path in (("tdr", TDR_PATH), ("proq", PROQ_PATH)):
        r = invoke("plugin.load_to_rack", {"track_id": track_id, "plugin_path": path})
        rr = r.get("result", {})
        pid = str(rr.get("plugin_id") or rr.get("node_id") or "")
        if not pid:
            print(f"  WARN: could not load {label} from {path}: status={r.get('status')}")
            continue
        ids[label] = pid
        print(f"  {label} plugin_id={pid}")
        time.sleep(0.5)
    return ids


def read_params(track_id: str, plugin_id: str) -> list[dict]:
    r = invoke("get_plugin_parameters",
               {"track_id": track_id, "plugin_id": plugin_id, "include_parameters": True},
               confirmed=False)
    return (r.get("result") or {}).get("parameters") or []


def band_slots(params: list[dict]) -> dict:
    """Group params into {band_label: {slot: (param_id, value_text)}}."""
    out: dict[str, dict] = {}
    for p in params:
        name = str(p.get("name") or "")
        pid = str(p.get("param_id") or "")
        val = str(p.get("value_text") or "")
        if not name.startswith("Band "):
            continue
        parts = name.split(None, 2)
        if len(parts) < 3:
            continue
        label = f"B{parts[1]}"
        slot = parts[2].strip().lower()
        out.setdefault(label, {})[slot] = (pid, val)
    return out


# ---------------------------------------------------------------------------
# Hand-built summaries  (these are what the Go-layer band_summary builder
# will produce automatically; here we build them from live data so the
# param_ids are always current)
# ---------------------------------------------------------------------------

def build_tdr_summary(params: list[dict]) -> dict:
    """fixed_slot summary for TDR Nova."""
    slots = band_slots(params)
    bands = []
    for label, s in sorted(slots.items()):
        freq_pid, freq_val = s.get("frequency", ("", ""))
        gain_pid, gain_val = s.get("gain", ("", ""))
        if not freq_pid:
            continue
        try:
            freq_hz = float(freq_val)
        except ValueError:
            freq_hz = 0.0
        bands.append({
            "band": label,
            "freq_param_id": freq_pid, "gain_param_id": gain_pid,
            "current_freq_hz": freq_hz,
            "current_gain_db": float(gain_val) if gain_val not in ("", "On", "Off") else 0.0,
        })
    return {
        "eq_model": "fixed_slot",
        "description": "Fixed band slots. To process a frequency: pick the band whose current_freq_hz is CLOSEST to the target. Only adjust gain (and optionally fine-tune frequency). Never pick a band just because it is first.",
        "bands": bands,
    }


def build_proq_summary(params: list[dict]) -> dict:
    """free_floating summary for Pro-Q 3."""
    slots = band_slots(params)
    active, available = [], []
    for label, s in sorted(slots.items()):
        used_pid, used_val = s.get("used", ("", ""))
        freq_pid, freq_val = s.get("frequency", ("", ""))
        gain_pid, gain_val = s.get("gain", ("", ""))
        shape_pid, _ = s.get("shape", ("", ""))
        if str(used_val).lower() in ("unused", "0", "off", "false", ""):
            available.append({"band": label, "used_param_id": used_pid,
                               "freq_param_id": freq_pid, "gain_param_id": gain_pid,
                               "shape_param_id": shape_pid})
        else:
            try:
                freq_hz = float(freq_val)
            except ValueError:
                freq_hz = 0.0
            active.append({"band": label, "current_freq_hz": freq_hz,
                            "current_gain_db": float(gain_val) if gain_val not in ("", "On", "Off") else 0.0,
                            "used_param_id": used_pid, "freq_param_id": freq_pid,
                            "gain_param_id": gain_pid, "shape_param_id": shape_pid})
    return {
        "eq_model": "free_floating",
        "description": (
            "Bands are created on demand. active_bands shows existing bands. "
            "available_slots are empty. "
            "To add a new band at a frequency: pick the first available_slot, then write ALL FOUR params: "
            "1) used_param_id = 'Used', 2) freq_param_id = target Hz, "
            "3) gain_param_id = target dB, 4) shape_param_id = 'Bell'. "
            "If an active band is already near the target, just adjust its gain_param_id instead."
        ),
        "active_bands": active,
        "available_slots": available[:5],  # show first 5 to keep context short
    }


# ---------------------------------------------------------------------------
# Core experiment runner
# ---------------------------------------------------------------------------

def extract_writes(resp: dict) -> list[dict]:
    """Return list of {param_id, value} from set_plugin_param calls."""
    writes = []
    for entry in resp.get("executed_kernel_reply") or []:
        if "set_plugin_param" not in str(entry.get("command_name") or ""):
            continue
        r = entry.get("result") or {}
        pid = str(r.get("param_id") or entry.get("args", {}).get("param_id") or "")
        val = str(r.get("new_value_text") or r.get("new_normalised_value") or "")
        writes.append({"param_id": pid, "value": val,
                       "new_norm": r.get("new_normalised_value"),
                       "new_text": r.get("new_value_text")})
    return writes


def run_one(conv_id: str, message: str, context: dict, timeout: float = 300.0) -> dict:
    resp = chat(conv_id, message, context, timeout)
    return {
        "goal_status": resp.get("goal_status"),
        "reply_snippet": str(resp.get("reply") or "")[:200],
        "writes": extract_writes(resp),
    }


def run_one_two_turn(conv_id: str, message: str, context: dict,
                     summary_json: dict, timeout: float = 300.0) -> dict:
    """
    Two-turn flow that simulates the Go-layer summary injection:
      Turn 1: LLM calls explain_controls (we let it run naturally)
      Turn 2: We feed the augmented context pack back as a tool-result simulation
              by appending the summary to the user message.

    Simpler alternative used here: inject summary as an XML block inside the
    user message — this is equivalent to the summary appearing in a tool result
    since the LLM reads both with the same attention.
    """
    summary_block = (
        "\n\n<eq_band_summary>\n"
        + json.dumps(summary_json, ensure_ascii=False, indent=2)
        + "\n</eq_band_summary>\n\n"
        + "Based on the eq_band_summary above, pick the correct band and immediately "
        + "call plugin.set_parameter (set_plugin_param) with the right param_ids. "
        + "Do NOT ask for clarification — all information needed is in the summary."
    )
    augmented = message + summary_block
    resp = chat(conv_id, augmented, context, timeout)
    return {
        "goal_status": resp.get("goal_status"),
        "reply_snippet": str(resp.get("reply") or "")[:200],
        "writes": extract_writes(resp),
    }


# ---------------------------------------------------------------------------
# Verdict helpers
# ---------------------------------------------------------------------------

def verdict_case1(writes: list[dict], tdr_slots: dict) -> tuple[str, str]:
    """3.4kHz -3dB on TDR Nova. Must touch B3's params, not B1's."""
    b1_freq = str(tdr_slots.get("B1", {}).get("frequency", ("", ""))[0])
    b3_freq = str(tdr_slots.get("B3", {}).get("frequency", ("", ""))[0])
    b3_gain = str(tdr_slots.get("B3", {}).get("gain", ("", ""))[0])
    touched = {w["param_id"] for w in writes}
    if b3_gain in touched or b3_freq in touched:
        return "PASS", f"correctly targeted B3 (pid {b3_gain})"
    if b1_freq in touched:
        return "FAIL", f"wrongly targeted B1 (pid {b1_freq}) — same error as today"
    return "INCONCLUSIVE", f"touched params {touched}, expected B3 freq={b3_freq} gain={b3_gain}"


def verdict_case2(writes: list[dict], tdr_slots: dict) -> tuple[str, str]:
    """100Hz -2dB on TDR Nova. Must touch B1 (closest to 100Hz)."""
    b1_freq = str(tdr_slots.get("B1", {}).get("frequency", ("", ""))[0])
    b1_gain = str(tdr_slots.get("B1", {}).get("gain", ("", ""))[0])
    touched = {w["param_id"] for w in writes}
    if b1_gain in touched or b1_freq in touched:
        return "PASS", f"correctly targeted B1 (pid {b1_gain})"
    return "FAIL", f"did not touch B1; touched {touched}"


def verdict_case3(writes: list[dict], proq_slots: dict) -> tuple[str, str]:
    """3.4kHz -3dB on Pro-Q 3. Must write Used + Freq + Gain (+ ideally Shape)."""
    if not proq_slots.get("B1"):
        return "SKIP", "Pro-Q 3 not loaded"
    b1 = proq_slots["B1"]
    used_pid = str(b1.get("used", ("", ""))[0])
    freq_pid = str(b1.get("frequency", ("", ""))[0])
    gain_pid = str(b1.get("gain", ("", ""))[0])
    touched = {w["param_id"] for w in writes}
    has_used = used_pid in touched
    has_freq = freq_pid in touched
    has_gain = gain_pid in touched
    if has_used and has_freq and has_gain:
        return "PASS", "wrote Used+Frequency+Gain (band activated correctly)"
    if has_freq and has_gain and not has_used:
        return "FAIL-SILENT", "wrote Freq+Gain but omitted Used — band will NOT appear in GUI (silent failure)"
    if has_gain and not has_freq and not has_used:
        return "FAIL", "only wrote gain, band not activated, frequency not set"
    if not writes:
        return "FAIL", "no writes at all"
    return "INCONCLUSIVE", f"touched {touched}, needed used={used_pid} freq={freq_pid} gain={gain_pid}"


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def run_experiment(ids: dict, reps: int = 3) -> None:
    track_id = ids["track_id"]
    tdr_id   = ids.get("tdr", "")
    proq_id  = ids.get("proq", "")

    # Collect live param data once
    tdr_params = read_params(track_id, tdr_id) if tdr_id else []
    proq_params = read_params(track_id, proq_id) if proq_id else []

    tdr_slots = band_slots(tdr_params)
    proq_slots_raw = band_slots(proq_params)

    tdr_summary = build_tdr_summary(tdr_params) if tdr_params else {}
    proq_summary = build_proq_summary(proq_params) if proq_params else {}

    print(f"\nTDR Nova bands (from live params):")
    for bl, s in sorted(tdr_slots.items()):
        fp, fv = s.get("frequency", ("?", "?"))
        gp, gv = s.get("gain", ("?", "?"))
        print(f"  {bl}: freq pid={fp} val={fv}  gain pid={gp} val={gv}")

    print(f"\nPro-Q 3 bands (sample):")
    n_active = sum(1 for s in proq_slots_raw.values() if str(s.get("used", ("",""))[1]).lower() not in ("unused",""))
    print(f"  {len(proq_slots_raw)} slots, {n_active} active")

    CASES = [
        {
            "name": "C1-TDR-3.4kHz",
            "plugin": "TDR Nova",
            "plugin_id": tdr_id,
            "message": "用TDR Nova把3.4kHz降3dB",
            "verdict_fn": lambda w: verdict_case1(w, tdr_slots),
            "summary": tdr_summary,
        },
        {
            "name": "C2-TDR-100Hz",
            "plugin": "TDR Nova",
            "plugin_id": tdr_id,
            "message": "用TDR Nova把100Hz降2dB",
            "verdict_fn": lambda w: verdict_case2(w, tdr_slots),
            "summary": tdr_summary,
        },
        {
            "name": "C3-ProQ-3.4kHz",
            "plugin": "Pro-Q 3",
            "plugin_id": proq_id,
            "message": "用FabFilter Pro-Q 3把3.4kHz降3dB",
            "verdict_fn": lambda w: verdict_case3(w, proq_slots_raw),
            "summary": proq_summary,
        },
    ]

    results: dict[str, list[str]] = {}

    for arm_name, use_summary in [("WITHOUT_SUMMARY", False), ("WITH_SUMMARY", True)]:
        print(f"\n{'='*60}")
        print(f"ARM: {arm_name}")
        print(f"{'='*60}")
        for case in CASES:
            pid = case["plugin_id"]
            if not pid:
                print(f"\n  {case['name']}: SKIP (plugin not loaded)")
                continue
            case_results = []
            for rep in range(1, reps + 1):
                conv_id = f"band_exp_{arm_name}_{case['name']}_r{rep}_{int(time.time())}"
                context = {
                    "agent_mode": "chat",
                    "selected_track_id": track_id,
                    "selected_plugin_id": pid,
                    "selected_plugin_name": case["plugin"],
                }
                print(f"\n  [{case['name']} rep {rep}] {case['message']}", end=" ... ", flush=True)
                try:
                    if use_summary and case["summary"]:
                        r = run_one_two_turn(conv_id, case["message"], context,
                                             case["summary"])
                    else:
                        r = run_one(conv_id, case["message"], context)
                    verdict, detail = case["verdict_fn"](r["writes"])
                    print(f"{verdict}")
                    print(f"    writes={[w['param_id'] for w in r['writes']]}  status={r['goal_status']}")
                    print(f"    {detail}")
                    case_results.append(verdict)
                except Exception as exc:
                    print(f"ERROR: {exc}")
                    case_results.append("ERROR")
                time.sleep(1.5)

            key = f"{arm_name}|{case['name']}"
            results[key] = case_results
            passes = sum(1 for v in case_results if v == "PASS")
            print(f"  -> {case['name']}: {passes}/{reps} PASS")

    # Final summary table
    print(f"\n{'='*60}")
    print("FINAL RESULTS")
    print(f"{'='*60}")
    print(f"{'Case':<22} {'Without summary':>17} {'With summary':>14}")
    print("-" * 55)
    for case in CASES:
        n = case["name"]
        w_key = f"WITHOUT_SUMMARY|{n}"
        s_key = f"WITH_SUMMARY|{n}"
        wres = results.get(w_key, [])
        sres = results.get(s_key, [])
        wp = sum(1 for v in wres if v == "PASS")
        sp = sum(1 for v in sres if v == "PASS")
        print(f"  {n:<20} {wp}/{len(wres)} {wres!s:>10}   {sp}/{len(sres)} {sres!s:>10}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--setup", action="store_true")
    parser.add_argument("--run", action="store_true")
    parser.add_argument("--track-id", default="")
    parser.add_argument("--tdr-id", default="")
    parser.add_argument("--proq-id", default="")
    parser.add_argument("--reps", type=int, default=3)
    args = parser.parse_args()

    if args.setup:
        ids = setup()
        print("\nTo run the experiment:")
        print(f"  python scripts/band_summary_experiment.py --run "
              f"--track-id {ids['track_id']} "
              f"--tdr-id {ids.get('tdr','')} "
              f"--proq-id {ids.get('proq','')}")
        return

    if args.run:
        if not args.track_id or not args.tdr_id:
            sys.exit("--run requires --track-id and --tdr-id")
        ids = {
            "track_id": args.track_id,
            "tdr": args.tdr_id,
            "proq": args.proq_id,
        }
        run_experiment(ids, reps=args.reps)
        return

    parser.print_help()


if __name__ == "__main__":
    main()

